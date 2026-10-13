package api

import (
	"errors"
	"net/http"
	"slices"
	"strings"
)

// Before/after comparison of two clinical photos (dermatology). The model describes visible differences in neutral words and
// says when the photos cannot be compared (light, angle, distance). It never classifies a lesion or gives a diagnosis.
// The files are read from the encrypted store, sent in this one request and not kept by us. One magic use.

var photoMimes = []string{"image/jpeg", "image/png", "image/webp"}

func (s *Server) photoForAI(c *aiCtx, id string) (*geminiBlob, string, error) {
	a, err := s.loadAttachment(c.r.Context(), c.p, id)
	if err != nil {
		return nil, "", err
	}
	if a.PatientID != c.id || a.ArchivedAt != nil || !slices.Contains(photoMimes, a.Mime) || a.SizeBytes > 4<<20 {
		return nil, "", fail(http.StatusBadRequest, "Elige dos fotos (JPG, PNG o WebP de hasta 4 MB) de este paciente que no estén archivadas.")
	}
	data, err := s.getBlob(a.storageKey, a.SHA256)
	if err != nil {
		if errors.Is(err, errBlobMissing) {
			return nil, "", fail(http.StatusGone, "Una de las fotos no está disponible en el almacenamiento.")
		}
		return nil, "", err
	}
	s.logAccess(c.r.Context(), c.p.ClinicID, c.id, c.p, "file_view")
	return &geminiBlob{Data: data, Mime: a.Mime}, a.CreatedAt.Format("2006-01-02") + " · " + strings.TrimSpace(a.Title), nil
}

func (s *Server) photoCompareAI(w http.ResponseWriter, r *http.Request) {
	c, ok := s.newAICtx(w, r)
	if !ok {
		return
	}
	if !c.hasKind("DERMATOLOGY") || !c.p.worksIn("DERMATOLOGY") || c.isAnimal() {
		writeError(w, http.StatusNotFound, "Esta ayuda no está disponible para este paciente.")
		return
	}
	if !s.aiReady(w) {
		return
	}
	var in struct {
		Before string `json:"before"`
		After  string `json:"after"`
		Zone   string `json:"zone"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Zone = truncateRunes(strings.TrimSpace(in.Zone), 120)
	if in.Before == in.After {
		writeError(w, http.StatusBadRequest, "Elige dos fotos distintas.")
		return
	}
	before, bLabel, err := s.photoForAI(c, in.Before)
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	after, aLabel, err := s.photoForAI(c, in.After)
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	instr := "Se adjuntan DOS fotos clínicas de la piel: la primera es ANTES (" + bLabel + ") y la segunda es DESPUÉS (" + aLabel + "). Compáralas con palabras neutrales. " +
		"`summary`: 2 oraciones sobre qué cambió y si las fotos son comparables. `sections`: «Cambios que se observan» (tamaño aparente, color, bordes, textura, número de lesiones), «Sin cambios aparentes» y «Limitaciones de la comparación» (luz, ángulo, distancia, enfoque, maquillaje). " +
		"Describe solo lo visible. NO diagnostiques, NO clasifiques lesiones (benigna o maligna) ni indiques tratamiento: eso lo valora el especialista en persona. Si no puedes comparar, dilo. `text` y `lines` vacíos."
	data := "Zona: " + in.Zone + "\n"
	if in.Zone == "" {
		data = "Zona: no indicada\n"
	}
	prompt := aiPrompt(instr, c.header(), data)
	s.finishAIBlobs(w, r, c.p, c.id, "photo_compare", "Comparar fotos", false, prompt, []geminiBlob{*before, *after}, "", nil)
}
