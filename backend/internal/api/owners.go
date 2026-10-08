package api

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Owners (propietarios) of animal patients. An owner is a record of the clinic; its pets point to it, so searching
// the owner finds all the pets and searching a pet shows whose it is. patients.guardian_* keeps a copy of the owner's
// data for everything that already reads it (printing, reminders, the portal).

type ownerRef struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

// ownerQuerier is what resolveOwner needs from the pool or a transaction.
type ownerQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type ownerPet struct {
	ID         string  `json:"id"`
	FileNumber int     `json:"file_number"`
	Names      string  `json:"names"`
	LastNames  string  `json:"last_names"`
	Species    string  `json:"species"`
	Archived   bool    `json:"archived"`
	Matched    bool    `json:"matched"` // the search found this pet itself (not only through its owner)
	LastVisit  *string `json:"last_visit,omitempty"`
}

type ownerGroup struct {
	Owner ownerRef   `json:"owner"`
	Pets  []ownerPet `json:"pets"`
}

// resolveOwner links an animal to its owner and makes patients.guardian_* a copy of that owner's data.
//   - owner_id given: that owner (of this clinic) is used as is;
//   - only the typed data: an owner with the same name and contact is reused, otherwise a new one is created.
//
// Anything but an animal, or an animal without an owner name, has no owner.
func resolveOwner(ctx context.Context, q ownerQuerier, clinicID string, in *patientIn) (*string, error) {
	if in.Subject != "animal" {
		return nil, nil
	}
	if in.OwnerID != nil && strings.TrimSpace(*in.OwnerID) != "" {
		id := strings.TrimSpace(*in.OwnerID)
		if !validUUID(id) {
			return nil, fail(http.StatusBadRequest, "El propietario no es válido.")
		}
		var o ownerRef
		err := q.QueryRow(ctx, `SELECT id::text, name, phone, email FROM owners WHERE clinic_id = $1 AND id = $2`, clinicID, id).Scan(&o.ID, &o.Name, &o.Phone, &o.Email)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fail(http.StatusBadRequest, "El propietario no existe en este consultorio.")
		}
		if err != nil {
			return nil, err
		}
		in.GuardianName, in.GuardianPhone, in.GuardianEmail = o.Name, o.Phone, o.Email
		return &o.ID, nil
	}
	in.GuardianName = strings.Join(strings.Fields(in.GuardianName), " ") // single spaces
	if in.GuardianName == "" {
		return nil, nil
	}
	var o ownerRef
	err := q.QueryRow(ctx, `
		SELECT id::text, name, phone, email FROM owners
		WHERE clinic_id = $1 AND caresia_name_key(name) = caresia_name_key($2) AND caresia_contact_key(phone, email) = caresia_contact_key($3, $4)
		ORDER BY created_at LIMIT 1`, clinicID, in.GuardianName, in.GuardianPhone, in.GuardianEmail).Scan(&o.ID, &o.Name, &o.Phone, &o.Email)
	switch {
	case err == nil:
		// The same person: complete what the owner did not have yet (and the copies in their pets).
		phone, email := o.Phone, o.Email
		if phone == "" {
			phone = in.GuardianPhone
		}
		if email == "" {
			email = in.GuardianEmail
		}
		if phone != o.Phone || email != o.Email {
			o.Phone, o.Email = phone, email
			if _, err := q.Exec(ctx, `UPDATE owners SET phone=$3, email=$4, updated_at=now() WHERE clinic_id=$1 AND id=$2`, clinicID, o.ID, phone, email); err != nil {
				return nil, err
			}
			if _, err := q.Exec(ctx, `UPDATE patients SET guardian_phone=$3, guardian_email=$4, updated_at=now() WHERE clinic_id=$1 AND owner_id=$2`, clinicID, o.ID, phone, email); err != nil {
				return nil, err
			}
		}
	case errors.Is(err, pgx.ErrNoRows):
		err = q.QueryRow(ctx, `INSERT INTO owners (clinic_id, name, phone, email) VALUES ($1,$2,$3,$4) RETURNING id::text`,
			clinicID, in.GuardianName, in.GuardianPhone, in.GuardianEmail).Scan(&o.ID)
		if err != nil {
			return nil, err
		}
		o.Name, o.Phone, o.Email = in.GuardianName, in.GuardianPhone, in.GuardianEmail
	default:
		return nil, err
	}
	in.GuardianName, in.GuardianPhone, in.GuardianEmail = o.Name, o.Phone, o.Email
	return &o.ID, nil
}

// ---------------------------------------------------------------------------
// GET /patients/owners?q= : owners with their pets (the picker of the forms)
// ---------------------------------------------------------------------------

var nonDigits = regexp.MustCompile(`\D`)

type ownerListItem struct {
	ownerRef
	PetCount int        `json:"pet_count"`
	Pets     []ownerPet `json:"pets"`
}

func (s *Server) searchOwners(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	digits := nonDigits.ReplaceAllString(q, "")
	if len(digits) < 3 {
		digits = ""
	}
	rows, err := s.db.Query(r.Context(), `
		SELECT o.id::text, o.name, o.phone, o.email,
		       (SELECT count(*) FROM patients p WHERE p.owner_id = o.id AND p.archived_at IS NULL)::int
		FROM owners o
		WHERE o.clinic_id = $1 AND ($2 = '' OR o.name ILIKE $3 OR o.email ILIKE $3
		      OR ($4 <> '' AND regexp_replace(o.phone, '\D', '', 'g') LIKE '%' || $4 || '%')
		      OR EXISTS (SELECT 1 FROM patients p WHERE p.owner_id = o.id AND p.names ILIKE $3))
		ORDER BY lower(o.name) LIMIT 12`, p.ClinicID, q, "%"+escapeLike(q)+"%", digits)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	out := []ownerListItem{}
	ids := []string{}
	for rows.Next() {
		var x ownerListItem
		if err := rows.Scan(&x.ID, &x.Name, &x.Phone, &x.Email, &x.PetCount); err != nil {
			serverError(w, r, err)
			return
		}
		x.Pets = []ownerPet{}
		out = append(out, x)
		ids = append(ids, x.ID)
	}
	rows.Close()
	pets, err := s.petsOfOwners(r.Context(), p.ClinicID, ids, false, "", "")
	if err != nil {
		serverError(w, r, err)
		return
	}
	for i := range out {
		out[i].Pets = pets[out[i].ID]
		if out[i].Pets == nil {
			out[i].Pets = []ownerPet{}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"owners": out})
}

// getOwner is GET /patients/owners/{id}: one owner with their pets.
func (s *Server) getOwner(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Propietario no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	var x ownerListItem
	err := s.db.QueryRow(r.Context(), `SELECT id::text, name, phone, email FROM owners WHERE clinic_id = $1 AND id = $2`, p.ClinicID, id).Scan(&x.ID, &x.Name, &x.Phone, &x.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Propietario no encontrado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	pets, err := s.petsOfOwners(r.Context(), p.ClinicID, []string{id}, false, "", "")
	if err != nil {
		serverError(w, r, err)
		return
	}
	x.Pets = pets[id]
	if x.Pets == nil {
		x.Pets = []ownerPet{}
	}
	x.PetCount = len(x.Pets)
	writeJSON(w, http.StatusOK, map[string]any{"owner": x})
}

// petsOfOwners loads the pets of those owners; matched marks the ones the search (q) found by themselves.
func (s *Server) petsOfOwners(ctx context.Context, clinicID string, ids []string, archived bool, q, like string) (map[string][]ownerPet, error) {
	out := map[string][]ownerPet{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT p.owner_id::text, p.id::text, p.file_number, p.names, p.last_names, coalesce(p.profile->>'species', ''), p.archived_at IS NOT NULL,
		       ($3 <> '' AND (p.names ILIKE $4 OR p.last_names ILIKE $4 OR (p.names || ' ' || p.last_names) ILIKE $4 OR p.curp = upper($3) OR p.file_number::text = $3)),
		       to_char(p.last_encounter_at, 'YYYY-MM-DD')
		FROM patients p
		WHERE p.clinic_id = $1 AND p.subject = 'animal' AND p.owner_id::text = ANY($2) AND ((p.archived_at IS NOT NULL) = $5)
		ORDER BY lower(p.names), p.file_number`, clinicID, ids, q, like, archived)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var owner string
		var x ownerPet
		if err := rows.Scan(&owner, &x.ID, &x.FileNumber, &x.Names, &x.LastNames, &x.Species, &x.Archived, &x.Matched, &x.LastVisit); err != nil {
			return nil, err
		}
		out[owner] = append(out[owner], x)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// GET /patients/grouped?q=&archived=1 : the list by owner
// ---------------------------------------------------------------------------

func (s *Server) groupedPatients(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	archived := r.URL.Query().Get("archived") == "1"
	like := "%" + escapeLike(q) + "%"
	digits := nonDigits.ReplaceAllString(q, "")
	if len(digits) < 3 {
		digits = ""
	}
	// The owners that have a pet in the requested state and match the search by themselves or through a pet.
	rows, err := s.db.Query(r.Context(), `
		SELECT o.id::text, o.name, o.phone, o.email
		FROM owners o
		WHERE o.clinic_id = $1 AND EXISTS (
			SELECT 1 FROM patients p WHERE p.owner_id = o.id AND p.subject = 'animal' AND ((p.archived_at IS NOT NULL) = $5)
			  AND ($2 = '' OR o.name ILIKE $3 OR o.email ILIKE $3 OR ($4 <> '' AND regexp_replace(o.phone, '\D', '', 'g') LIKE '%' || $4 || '%')
			       OR p.names ILIKE $3 OR p.last_names ILIKE $3 OR (p.names || ' ' || p.last_names) ILIKE $3 OR p.curp = upper($2) OR p.file_number::text = $2))
		ORDER BY lower(o.name) LIMIT 60`, p.ClinicID, q, like, digits, archived)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	groups := []ownerGroup{}
	ids := []string{}
	for rows.Next() {
		var g ownerGroup
		if err := rows.Scan(&g.Owner.ID, &g.Owner.Name, &g.Owner.Phone, &g.Owner.Email); err != nil {
			serverError(w, r, err)
			return
		}
		groups = append(groups, g)
		ids = append(ids, g.Owner.ID)
	}
	rows.Close()
	pets, err := s.petsOfOwners(r.Context(), p.ClinicID, ids, archived, q, like)
	if err != nil {
		serverError(w, r, err)
		return
	}
	for i := range groups {
		groups[i].Pets = pets[groups[i].Owner.ID]
		if groups[i].Pets == nil {
			groups[i].Pets = []ownerPet{}
		}
		// Found through the owner (name, phone or e-mail): all of their pets are results.
		o := groups[i].Owner
		byOwner := q != "" && (strings.Contains(strings.ToLower(o.Name), strings.ToLower(q)) || strings.Contains(strings.ToLower(o.Email), strings.ToLower(q)) ||
			(digits != "" && strings.Contains(nonDigits.ReplaceAllString(o.Phone, ""), digits)))
		for j := range groups[i].Pets {
			if byOwner {
				groups[i].Pets[j].Matched = true
			}
		}
	}
	// Animals that have no owner registered.
	where, args := searchWhere(p.ClinicID, q)
	where += " AND subject = 'animal' AND owner_id IS NULL"
	if archived {
		where += " AND archived_at IS NOT NULL"
	} else {
		where += " AND archived_at IS NULL"
	}
	orphans := s.patientQuery(w, r, where, args, 30)
	if orphans == nil {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": groups, "orphans": orphans})
}

// ---------------------------------------------------------------------------
// GET /patients/{id}/owner : the owner of a pet and the pet's siblings
// ---------------------------------------------------------------------------

func (s *Server) patientOwner(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	var o ownerRef
	err := s.db.QueryRow(r.Context(), `
		SELECT o.id::text, o.name, o.phone, o.email FROM patients p JOIN owners o ON o.id = p.owner_id
		WHERE p.clinic_id = $1 AND p.id = $2`, p.ClinicID, id).Scan(&o.ID, &o.Name, &o.Phone, &o.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusOK, map[string]any{"owner": nil, "siblings": []ownerPet{}})
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	pets, err := s.petsOfOwners(r.Context(), p.ClinicID, []string{o.ID}, false, "", "")
	if err != nil {
		serverError(w, r, err)
		return
	}
	siblings := []ownerPet{}
	for _, x := range pets[o.ID] {
		if x.ID != id {
			siblings = append(siblings, x)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"owner": o, "siblings": siblings})
}

// ---------------------------------------------------------------------------
// PUT /patients/owners/{id} and POST /patients/owners/{id}/merge
// ---------------------------------------------------------------------------

type ownerIn struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

func (s *Server) updateOwner(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Propietario no encontrado.")
		return
	}
	var in ownerIn
	if !decode(w, r, &in) {
		return
	}
	in.Name, in.Phone, in.Email = strings.TrimSpace(in.Name), strings.TrimSpace(in.Phone), strings.TrimSpace(in.Email)
	switch {
	case in.Name == "":
		writeError(w, http.StatusBadRequest, "Escribe el nombre del propietario.")
		return
	case utf8.RuneCountInString(in.Name) > 160 || utf8.RuneCountInString(in.Phone) > 30 || utf8.RuneCountInString(in.Email) > 160:
		writeError(w, http.StatusBadRequest, "Uno de los datos es demasiado largo.")
		return
	case in.Email != "" && !regexp.MustCompile(`^\S+@\S+\.\S+$`).MatchString(in.Email):
		writeError(w, http.StatusBadRequest, "Revisa el correo electrónico.")
		return
	}
	p := principalFrom(r.Context())
	var out ownerRef
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		err := tx.QueryRow(r.Context(), `UPDATE owners SET name=$3, phone=$4, email=$5, updated_at=now() WHERE clinic_id=$1 AND id=$2 RETURNING id::text, name, phone, email`,
			p.ClinicID, id, in.Name, in.Phone, in.Email).Scan(&out.ID, &out.Name, &out.Phone, &out.Email)
		if errors.Is(err, pgx.ErrNoRows) {
			return fail(http.StatusNotFound, "Propietario no encontrado.")
		}
		if err != nil {
			return err
		}
		// Every pet carries a copy of the owner's data.
		if _, err := tx.Exec(r.Context(), `UPDATE patients SET guardian_name=$3, guardian_phone=$4, guardian_email=$5, updated_at=now() WHERE clinic_id=$1 AND owner_id=$2`,
			p.ClinicID, id, out.Name, out.Phone, out.Email); err != nil {
			return err
		}
		audit(r.Context(), tx, p.ClinicID, p, "owner_updated", "Actualizó los datos de un propietario", map[string]any{"owner": id})
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"owner": out})
}

// mergeOwner moves every pet of one owner to another (the same person registered twice) and removes the empty one.
func (s *Server) mergeOwner(w http.ResponseWriter, r *http.Request) {
	from := chi.URLParam(r, "id")
	var in struct {
		Into string `json:"into"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !validUUID(from) || !validUUID(in.Into) || from == in.Into {
		writeError(w, http.StatusBadRequest, "Elige otro propietario con quien unir.")
		return
	}
	p := principalFrom(r.Context())
	var moved int
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		var target ownerRef
		if err := tx.QueryRow(r.Context(), `SELECT id::text, name, phone, email FROM owners WHERE clinic_id=$1 AND id=$2 FOR UPDATE`, p.ClinicID, in.Into).Scan(&target.ID, &target.Name, &target.Phone, &target.Email); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fail(http.StatusNotFound, "Propietario no encontrado.")
			}
			return err
		}
		var exists bool
		if err := tx.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM owners WHERE clinic_id=$1 AND id=$2)`, p.ClinicID, from).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fail(http.StatusNotFound, "Propietario no encontrado.")
		}
		tag, err := tx.Exec(r.Context(), `UPDATE patients SET owner_id=$3, guardian_name=$4, guardian_phone=$5, guardian_email=$6, updated_at=now() WHERE clinic_id=$1 AND owner_id=$2`,
			p.ClinicID, from, target.ID, target.Name, target.Phone, target.Email)
		if err != nil {
			return err
		}
		moved = int(tag.RowsAffected())
		if _, err := tx.Exec(r.Context(), `DELETE FROM owners WHERE clinic_id=$1 AND id=$2`, p.ClinicID, from); err != nil {
			return err
		}
		audit(r.Context(), tx, p.ClinicID, p, "owner_merged", "Unió dos propietarios ("+itoa(moved)+" mascotas)", map[string]any{"from": from, "into": in.Into})
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"moved": moved})
}
