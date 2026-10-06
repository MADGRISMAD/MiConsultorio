package api

import "net/http"

type clinic struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	PhoneNumber string `json:"phone_number"`
	Address     string `json:"address"`
	ImageURL    string `json:"image_url"`
}

func (s *Server) clinicInfo(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	var c clinic
	err := s.db.QueryRow(r.Context(),
		`SELECT id, name, phone_number, address, image_url FROM clinics WHERE id = $1`, p.ClinicID,
	).Scan(&c.ID, &c.Name, &c.PhoneNumber, &c.Address, &c.ImageURL)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"clinic": c})
}
