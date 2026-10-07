package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"sync/atomic"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/madgrismad/miconsultorio/backend/internal/fieldcrypt"
)

// Encryption at rest of free-text clinical columns (see package fieldcrypt):
//
//	encounters.subjective/exam/assessment/plan/notes   prescriptions.diagnosis/instructions
//	patients.profile (antecedentes, alergias)          appointments.details
//	lab_orders.notes                                   lab_results.notes
//
// Everything that reads or writes those columns goes through encField/decField/encProfile/decProfile (or the scan
// helpers that call them); encfields_test.go fails when a new file touches them without being reviewed.
// Columns that SQL aggregates or filters on stay in plain text on purpose: reason, measures, diagnosis_codes,
// prescriptions.items, names, dates, statuses and profile.species.

// codeDecryptFailed is the error code the API answers when sealed content cannot be opened.
const codeDecryptFailed = "DECRYPT_FAILED"

// clinicalVault is the key ring of this process plus the pool used to audit failures to open a value. Scan helpers
// are free functions, so the ring is process-wide; newServer installs it.
type clinicalVault struct {
	ring *fieldcrypt.Ring
	db   *pgxpool.Pool
}

var activeVault atomic.Pointer[clinicalVault]

func installVault(db *pgxpool.Pool, key []byte) {
	ring, err := fieldcrypt.NewRing(key)
	if err != nil {
		log.Printf("clinical field encryption disabled: %v", err)
		activeVault.Store(&clinicalVault{db: db})
		return
	}
	activeVault.Store(&clinicalVault{ring: ring, db: db})
}

func vaultRing() *fieldcrypt.Ring {
	if v := activeVault.Load(); v != nil {
		return v.ring
	}
	return nil
}

// encField seals a text value for storage in table.column of the row with this id.
func encField(table, column, rowID, plain string) (string, error) {
	return vaultRing().Seal(table, column, rowID, plain)
}

// decField opens a stored text value (legacy plaintext is returned as it is).
func decField(table, column, rowID, stored string) (string, error) {
	return vaultRing().Open(table, column, rowID, stored)
}

// encFields seals several values of one row in place; names are the columns of ptrs in order.
func encFields(table, rowID string, cols []string, ptrs ...*string) error {
	for i, p := range ptrs {
		v, err := encField(table, cols[i], rowID, *p)
		if err != nil {
			return err
		}
		*p = v
	}
	return nil
}

// decFields opens several values of one row in place.
func decFields(table, rowID string, cols []string, ptrs ...*string) error {
	for i, p := range ptrs {
		v, err := decField(table, cols[i], rowID, *p)
		if err != nil {
			return err
		}
		*p = v
	}
	return nil
}

// encProfile builds the jsonb stored in patients.profile.
func encProfile(rowID string, profile map[string]any) ([]byte, error) {
	return vaultRing().SealProfile("patients", "profile", rowID, profile)
}

// decProfile reads the jsonb of patients.profile (never nil on success).
func decProfile(rowID string, raw []byte) (map[string]any, error) {
	return vaultRing().OpenProfile("patients", "profile", rowID, raw)
}

// newRowID is a random UUID v4: rows with sealed columns need their id before the INSERT because it is part of
// the authenticated data.
func newRowID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

// decryptFailure answers (and audits) a value that could not be opened. It reports whether err was one.
func decryptFailure(w http.ResponseWriter, r *http.Request, err error) bool {
	var de *fieldcrypt.DecryptError
	if !errors.As(err, &de) {
		return false
	}
	logf(r, "decrypt failed: %s.%s row %s", de.Table, de.Column, de.RowID)
	if v := activeVault.Load(); v != nil && v.db != nil {
		p := principalFrom(r.Context())
		clinic := ""
		if p != nil {
			clinic = p.ClinicID
		}
		audit(r.Context(), v.db, clinic, p, "decrypt_failed", "No se pudo descifrar contenido clínico guardado ("+de.Table+"."+de.Column+")",
			map[string]any{"table": de.Table, "column": de.Column, "row": de.RowID})
	}
	writeJSON(w, http.StatusInternalServerError, errorBody{
		Message: "No se pudo descifrar este contenido clínico: la llave de cifrado del servidor no coincide con la que se usó al guardarlo. Avisa al administrador del sistema y no vuelvas a guardar sobre este registro.",
		Code:    codeDecryptFailed,
	})
	return true
}
