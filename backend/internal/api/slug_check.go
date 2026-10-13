package api

import (
	"net/http"
	"strings"
)

// slugify turns a name into the shape of a public address: lowercase letters and digits joined by single hyphens.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer("á", "a", "à", "a", "ä", "a", "é", "e", "è", "e", "ë", "e", "í", "i", "ï", "i", "ó", "o", "ò", "o", "ö", "o", "ú", "u", "ü", "u", "ñ", "n").Replace(s)
	var b strings.Builder
	dash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		default:
			if b.Len() > 0 && !dash {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 63 {
		out = strings.Trim(out[:63], "-")
	}
	return out
}

// slugCheck is GET /agenda/slug-check?slug=: whether an online address is free and, when it is not, a few free variants
// built from the same name plus the clinic's city, giro or a number. It is only advice: saving still enforces uniqueness.
func (s *Server) slugCheck(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	raw := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("slug")))
	res := map[string]any{"slug": raw, "available": false, "state": "invalid", "suggestions": []string{}}

	taken := func(list []string) (map[string]bool, error) {
		out := map[string]bool{}
		rows, err := s.db.Query(r.Context(), `SELECT lower(booking_slug) FROM agenda_settings WHERE lower(booking_slug) = ANY($1) AND clinic_id <> $2`, list, p.ClinicID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err != nil {
				return nil, err
			}
			out[v] = true
		}
		return out, rows.Err()
	}

	state := "ok"
	switch {
	case raw == "":
		writeJSON(w, http.StatusOK, res)
		return
	case reservedSlugs[raw]:
		state = "reserved"
	case !slugShape.MatchString(raw):
		state = "invalid"
	default:
		t, err := taken([]string{raw})
		if err != nil {
			serverError(w, r, err)
			return
		}
		if t[raw] {
			state = "taken"
		}
	}
	res["state"] = state
	res["available"] = state == "ok"
	if state == "ok" {
		writeJSON(w, http.StatusOK, res)
		return
	}

	// variants: the same name (cleaned) with the clinic's city, giro or a number
	base := slugify(raw)
	if base == "" {
		writeJSON(w, http.StatusOK, res)
		return
	}
	var city string
	_ = s.db.QueryRow(r.Context(), `SELECT coalesce(city, '') FROM clinic_profile WHERE clinic_id = $1`, p.ClinicID).Scan(&city)
	extras := []string{slugify(city)}
	if kinds, err := s.clinicKindsFor(r.Context(), p.ClinicID); err == nil && len(kinds) > 0 {
		extras = append(extras, slugify(areaLabels[kinds[0]]))
	}
	extras = append(extras, "consultorio", "clinica", "mx", "2")
	var cands []string
	seen := map[string]bool{raw: true}
	add := func(c string) {
		if len(c) > 63 {
			return
		}
		if !seen[c] && slugShape.MatchString(c) && !reservedSlugs[c] {
			seen[c] = true
			cands = append(cands, c)
		}
	}
	if state == "invalid" || state == "reserved" {
		add(base)
	}
	for _, x := range extras {
		if x != "" && !strings.HasSuffix(base, "-"+x) {
			add(base + "-" + x)
		}
	}
	for i := 3; i <= 5; i++ {
		add(base + "-" + string(rune('0'+i)))
	}
	if len(cands) > 0 {
		t, err := taken(cands)
		if err != nil {
			serverError(w, r, err)
			return
		}
		free := []string{}
		for _, c := range cands {
			if !t[c] && len(free) < 4 {
				free = append(free, c)
			}
		}
		res["suggestions"] = free
	}
	writeJSON(w, http.StatusOK, res)
}
