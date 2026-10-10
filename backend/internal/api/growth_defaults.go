package api

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The WHO and CDC tables that come already loaded, so a specialist does not have to import them. Where the files come from and how
// they were prepared is in growthdata/build.py. They are stored like any import, without a clinic, and a clinic's own import wins.

//go:embed growthdata/oms.csv
var growthOMS string

//go:embed growthdata/cdc.csv
var growthCDC string

var growthDefaults = []struct{ standard, source, csv string }{
	{"OMS", "OMS · Patrones de crecimiento infantil, 0 a 5 años (tablas LMS mensuales de peso, talla, IMC y perímetro cefálico)", growthOMS},
	{"CDC", "CDC · Gráficas de crecimiento, 0 a 20 años (archivos de datos LMS de peso, talla, IMC y perímetro cefálico)", growthCDC},
}

// SeedGrowthDefaults loads the shipped tables the first time, and again as a new version when they change. It is safe to call at every start.
func SeedGrowthDefaults(ctx context.Context, pool *pgxpool.Pool) error {
	for _, d := range growthDefaults {
		sum := sha256.Sum256([]byte(d.csv))
		digest := hex.EncodeToString(sum[:])
		var have bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM growth_imports WHERE clinic_id IS NULL AND standard = $1 AND sha256 = $2)`, d.standard, digest).Scan(&have); err != nil {
			return err
		}
		if have {
			continue
		}
		res := parseGrowthCSV(d.csv)
		if res.Total > 0 {
			return fmt.Errorf("growth defaults %s: the shipped table is invalid (%d errors)", d.standard, res.Total)
		}
		err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('growth|defaults|' || $1, 0))`, d.standard); err != nil {
				return err
			}
			var version int
			var id string
			if err := tx.QueryRow(ctx, `SELECT coalesce(max(version), 0) + 1 FROM growth_imports WHERE clinic_id IS NULL AND standard = $1`, d.standard).Scan(&version); err != nil {
				return err
			}
			if err := tx.QueryRow(ctx, `INSERT INTO growth_imports (clinic_id, standard, version, source_name, file_name, sha256, row_count, created_by_name)
				VALUES (NULL, $1, $2, $3, $4, $5, $6, 'Caresia') RETURNING id::text`, d.standard, version, d.source, d.standard+".csv", digest, len(res.Rows)).Scan(&id); err != nil {
				return err
			}
			batch := &pgx.Batch{}
			for _, row := range res.Rows {
				pcts := map[string]float64{}
				for k, v := range row.Pcts {
					pcts[strconv.Itoa(k)] = v
				}
				raw, _ := json.Marshal(pcts)
				batch.Queue(`INSERT INTO growth_references (clinic_id, import_id, standard, indicator, sex, age_months, l, m, s, pcts) VALUES (NULL,$1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb)`,
					id, d.standard, row.Indicator, row.Sex, row.Age, row.L, row.M, row.S, string(raw))
			}
			br := tx.SendBatch(ctx, batch)
			for range res.Rows {
				if _, err := br.Exec(); err != nil {
					_ = br.Close()
					return err
				}
			}
			return br.Close()
		})
		if err != nil {
			return fmt.Errorf("growth defaults %s: %w", d.standard, err)
		}
	}
	return nil
}
