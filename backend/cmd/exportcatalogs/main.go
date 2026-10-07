// Command exportcatalogs writes the clinical catalogs embedded in the app as CSV files for a professional to review
// (medicines, CIE-10, vaccination and deworming schemes, consent texts). It only reads: it never changes a dose.
//
//	cd backend && go run ./cmd/exportcatalogs            # writes ../docs/revision-clinica/*.csv
//	cd backend && go run ./cmd/exportcatalogs -out /tmp  # another folder
//
// It needs no configuration. The consent texts are read from the frontend source when it is next to the backend.
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/madgrismad/miconsultorio/backend/internal/api"
)

// Every file ends with the same three columns, to be filled in by the reviewer.
var reviewCols = []string{"Revisado por", "Fecha de revisión", "Observación"}

func main() {
	out := flag.String("out", "", "output folder (default: docs/revision-clinica at the repository root)")
	flag.Parse()
	root := repoRoot()
	dir := *out
	if dir == "" {
		dir = filepath.Join(root, "docs", "revision-clinica")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatal(err)
	}

	meds := api.CatalogMedications()
	var human, animal [][]string
	for _, m := range meds {
		if m.Subject == "animal" {
			animal = append(animal, medRow(m, true))
		} else {
			human = append(human, medRow(m, false))
		}
	}
	write(dir, "medicamentos-humanos.csv", medHeader(false), human)
	write(dir, "medicamentos-veterinarios.csv", medHeader(true), animal)

	var icd [][]string
	for _, e := range api.CatalogICD10() {
		icd = append(icd, []string{e.Code, e.Name})
	}
	write(dir, "cie10-parcial.csv", []string{"Código", "Descripción"}, icd)

	names, schemes := api.CatalogVaccineSchemes()
	var vac [][]string
	for _, n := range names {
		who := n
		if n == "person" {
			who = "Personas (esquema básico de referencia)"
		}
		for _, v := range schemes[n] {
			vac = append(vac, []string{who, kindLabel(v.Kind), v.Name, interval(v.IntervalDays), v.Note})
		}
	}
	write(dir, "vacunacion-desparasitacion.csv", []string{"Especie o sujeto", "Tipo", "Nombre", "Refuerzo sugerido", "Nota"}, vac)

	consents := consentRows(filepath.Join(root, "frontend", "src", "lib", "components", "specialty", "consentTexts.ts"))
	if consents != nil {
		write(dir, "consentimientos.csv", []string{"Tipo", "Texto sugerido (los marcadores ${...} se llenan con datos del paciente y del consultorio)"}, consents)
	} else {
		log.Printf("consentimientos.csv omitido: no se encontró frontend/src/lib/components/specialty/consentTexts.ts")
	}
	fmt.Printf("Listo: %d medicamentos humanos, %d veterinarios, %d códigos CIE-10, %d esquemas, %d consentimientos en %s\n",
		len(human), len(animal), len(icd), len(vac), len(consents), dir)
}

// repoRoot finds the repository folder: the one that holds backend/go.mod, walking up from the working directory.
func repoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	for d := wd; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "backend", "go.mod")); err == nil {
			return d
		}
		if filepath.Dir(d) == d {
			return filepath.Dir(wd) // run from backend/: its parent
		}
	}
}

func medHeader(animal bool) []string {
	h := []string{"ID", "Denominación genérica", "Nombre comercial"}
	if animal {
		h = append(h, "Especies")
	}
	h = append(h, "Categoría", "Control (fracción / receta)", "Vía", "Presentaciones", "Dosis típica", "mg/kg por dosis", "Máx. mg/kg por día", "Concentraciones (mg/mL)", "Notas")
	return append(h, reviewCols...)
}

func medRow(m api.CatalogMed, animal bool) []string {
	row := []string{m.ID, m.Name, m.Brand}
	if animal {
		row = append(row, strings.Join(m.Species, ", "))
	}
	var conc []string
	for _, c := range m.Concentrations {
		conc = append(conc, c.Label+": "+num(c.MgPerMl)+" mg/mL")
	}
	row = append(row, m.Category, m.Control, m.Route, strings.Join(m.Presentations, " | "), m.TypicalDose, num(m.MgPerKg), num(m.MaxMgPerKgDay), strings.Join(conc, " | "), m.Notes)
	return append(row, "", "", "")
}

func num(f float64) string {
	if f == 0 {
		return ""
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func kindLabel(k string) string {
	switch k {
	case "vaccine":
		return "Vacuna"
	case "deworming_internal":
		return "Desparasitación interna"
	case "deworming_external":
		return "Desparasitación externa"
	}
	return "Otro"
}

func interval(days int) string {
	if days == 0 {
		return "Sin refuerzo automático"
	}
	return fmt.Sprintf("%d días", days)
}

var consentCase = regexp.MustCompile("(?s)case '([a-z_]+)':\\s*return `(.*?)`;")

// consentRows pulls the suggested texts out of the frontend source. It returns nil when the file is not there.
func consentRows(path string) [][]string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var rows [][]string
	for _, m := range consentCase.FindAllStringSubmatch(string(raw), -1) {
		rows = append(rows, []string{m[1], m[2]})
	}
	return rows
}

// write saves a CSV with a UTF-8 byte-order mark so Excel opens the accents correctly. Review columns are added to
// the header of files that do not carry them yet (everything but the medicine files, which add them per row).
func write(dir, name string, header []string, rows [][]string) {
	if !strings.HasPrefix(name, "medicamentos") {
		header = append(append([]string{}, header...), reviewCols...)
		for i := range rows {
			rows[i] = append(rows[i], "", "", "")
		}
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		log.Fatal(err)
	}
	w := csv.NewWriter(f)
	if err := w.Write(header); err != nil {
		log.Fatal(err)
	}
	if err := w.WriteAll(rows); err != nil {
		log.Fatal(err)
	}
}
