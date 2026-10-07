package fieldcrypt

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Target is one protected column.
type Target struct {
	Table, Column string
	JSON          bool // jsonb profile instead of text
}

// Targets lists every column that is stored sealed. A source-scan test in package api keeps the code in sync with it.
var Targets = []Target{
	{Table: "encounters", Column: "subjective"},
	{Table: "encounters", Column: "exam"},
	{Table: "encounters", Column: "assessment"},
	{Table: "encounters", Column: "plan"},
	{Table: "encounters", Column: "notes"},
	{Table: "prescriptions", Column: "diagnosis"},
	{Table: "prescriptions", Column: "instructions"},
	{Table: "patients", Column: "profile", JSON: true},
	{Table: "appointments", Column: "details"},
}

// Stats counts what one run saw for a column.
type Stats struct {
	Target
	Scanned   int // non-empty values
	Sealed    int // plaintext values sealed (or that would be, in a dry run)
	Rekeyed   int // sealed with an older key and re-sealed with the current one
	Already   int // already sealed with the current key
	Undecrypt int // sealed values that cannot be opened with the available keys (left untouched)
	Raced     int // rows the application changed while the run was working (picked up by the next run)
}

// Options of EncryptAll.
type Options struct {
	DryRun bool
	Batch  int
}

// EncryptAll seals every plaintext value of the targets, in batches. It can be run any number of times:
// sealed values are skipped, and an update only applies while the row still holds the value that was read.
func EncryptAll(ctx context.Context, pool *pgxpool.Pool, ring *Ring, opt Options) ([]Stats, error) {
	if opt.Batch <= 0 {
		opt.Batch = 500
	}
	byTable := map[string][]Target{}
	var order []string
	for _, t := range Targets {
		if _, ok := byTable[t.Table]; !ok {
			order = append(order, t.Table)
		}
		byTable[t.Table] = append(byTable[t.Table], t)
	}
	var all []Stats
	for _, table := range order {
		st, err := encryptTable(ctx, pool, ring, opt, table, byTable[table])
		all = append(all, st...)
		if err != nil {
			return all, fmt.Errorf("%s: %w", table, err)
		}
	}
	return all, nil
}

type pending struct {
	id       string
	col      int
	old, new string
}

func encryptTable(ctx context.Context, pool *pgxpool.Pool, ring *Ring, opt Options, table string, cols []Target) ([]Stats, error) {
	stats := make([]Stats, len(cols))
	sel := make([]string, len(cols))
	for i, c := range cols {
		stats[i].Target = c
		sel[i] = c.Column + "::text"
	}
	last := "00000000-0000-0000-0000-000000000000"
	for {
		rows, err := pool.Query(ctx, `SELECT id::text, `+strings.Join(sel, ", ")+` FROM `+table+` WHERE id > $1::uuid ORDER BY id LIMIT $2`, last, opt.Batch)
		if err != nil {
			return stats, err
		}
		var changes []pending
		n := 0
		for rows.Next() {
			vals := make([]string, len(cols))
			ptrs := []any{new(string)}
			for i := range vals {
				ptrs = append(ptrs, &vals[i])
			}
			if err := rows.Scan(ptrs...); err != nil {
				rows.Close()
				return stats, err
			}
			id := *(ptrs[0].(*string))
			n++
			last = id
			for i, c := range cols {
				nv, kind, err := resealValue(ring, c, id, vals[i])
				if err != nil {
					rows.Close()
					return stats, err
				}
				if kind == kindEmpty {
					continue
				}
				stats[i].Scanned++
				switch kind {
				case kindUndecrypt:
					stats[i].Undecrypt++
				case kindCurrent:
					stats[i].Already++
				case kindSeal:
					stats[i].Sealed++
					changes = append(changes, pending{id, i, vals[i], nv})
				case kindRekey:
					stats[i].Rekeyed++
					changes = append(changes, pending{id, i, vals[i], nv})
				}
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return stats, err
		}
		if !opt.DryRun && len(changes) > 0 {
			if err := applyBatch(ctx, pool, table, cols, stats, changes); err != nil {
				return stats, err
			}
		}
		if n < opt.Batch {
			return stats, nil
		}
	}
}

func applyBatch(ctx context.Context, pool *pgxpool.Pool, table string, cols []Target, stats []Stats, changes []pending) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, ch := range changes {
		c := cols[ch.col]
		cast := ""
		if c.JSON {
			cast = "::jsonb"
		}
		// Compare against the value that was read, so a concurrent edit is never overwritten.
		tag, err := tx.Exec(ctx, `UPDATE `+table+` SET `+c.Column+` = $2`+cast+` WHERE id = $1::uuid AND `+c.Column+`::text = $3`, ch.id, ch.new, ch.old)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			stats[ch.col].Raced++
		}
	}
	return tx.Commit(ctx)
}

type valueKind int

const (
	kindEmpty valueKind = iota
	kindSeal
	kindRekey
	kindCurrent
	kindUndecrypt
)

// resealValue decides what to do with one stored value and returns the new stored form when it changes.
func resealValue(ring *Ring, c Target, id, stored string) (string, valueKind, error) {
	if c.JSON {
		if stored == "" || stored == "{}" || stored == "null" {
			return "", kindEmpty, nil
		}
		if sealed, kid := IsProfileSealed([]byte(stored)); sealed {
			if _, err := ring.OpenProfile(c.Table, c.Column, id, []byte(stored)); err != nil {
				return "", kindUndecrypt, nil
			}
			if kid == ring.Current() {
				return "", kindCurrent, nil
			}
			return rewrite(ring, c, id, stored, kindRekey)
		}
		return rewrite(ring, c, id, stored, kindSeal)
	}
	if stored == "" {
		return "", kindEmpty, nil
	}
	if IsSealed(stored) {
		if _, err := ring.Open(c.Table, c.Column, id, stored); err != nil {
			return "", kindUndecrypt, nil
		}
		if KeyID(stored) == ring.Current() {
			return "", kindCurrent, nil
		}
		return rewrite(ring, c, id, stored, kindRekey)
	}
	return rewrite(ring, c, id, stored, kindSeal)
}

func rewrite(ring *Ring, c Target, id, stored string, kind valueKind) (string, valueKind, error) {
	if c.JSON {
		m, err := ring.OpenProfile(c.Table, c.Column, id, []byte(stored))
		if err != nil {
			return "", kindUndecrypt, nil
		}
		b, err := ring.SealProfile(c.Table, c.Column, id, m)
		return string(b), kind, err
	}
	plain, err := ring.Open(c.Table, c.Column, id, stored)
	if err != nil {
		return "", kindUndecrypt, nil
	}
	s, err := ring.Seal(c.Table, c.Column, id, plain)
	return s, kind, err
}
