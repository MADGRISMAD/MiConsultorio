package api

// Catalog of common panels and analytes offered by the laboratory capture screen.
//
// These are GENERAL reference ranges (adults of general use, conventional units), kept to values that are widely
// published. They only pre-fill the form: the range printed by the laboratory that issued the result always takes
// priority and, when typed on a result, is the one used for the flag. Pediatric, pregnancy and altitude ranges differ
// and are deliberately not included.

const labCatalogVersion = "2026.1"

const labCatalogNotice = "Rangos de referencia generales: el rango del laboratorio que emite el resultado tiene prioridad."

type labBound struct {
	Low  *float64 `json:"low"`
	High *float64 `json:"high"`
}

type labAnalyteDef struct {
	Name      string    `json:"name"`
	Unit      string    `json:"unit"`
	Kind      string    `json:"kind"` // num | text
	Ref       *labBound `json:"ref,omitempty"`
	RefMale   *labBound `json:"ref_male,omitempty"`
	RefFemale *labBound `json:"ref_female,omitempty"`
	Expected  string    `json:"expected,omitempty"` // normal value of a qualitative analyte
	Note      string    `json:"note,omitempty"`
}

type labPanelDef struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Audience string          `json:"audience"`          // person | animal
	Species  string          `json:"species,omitempty"` // animal panels only
	Note     string          `json:"note,omitempty"`
	Analytes []labAnalyteDef `json:"analytes"`
}

func lbp(v float64) *float64 { return &v }

func labRng(low, high float64) *labBound { return &labBound{Low: lbp(low), High: lbp(high)} }

// labNum is an analyte with one range for everybody.
func labNum(name, unit string, low, high float64) labAnalyteDef {
	return labAnalyteDef{Name: name, Unit: unit, Kind: "num", Ref: labRng(low, high)}
}

// labMax has only an upper limit (desirable below).
func labMax(name, unit string, high float64, note string) labAnalyteDef {
	return labAnalyteDef{Name: name, Unit: unit, Kind: "num", Ref: &labBound{High: lbp(high)}, Note: note}
}

// labMin has only a lower limit (desirable above).
func labMin(name, unit string, low float64, note string) labAnalyteDef {
	return labAnalyteDef{Name: name, Unit: unit, Kind: "num", Ref: &labBound{Low: lbp(low)}, Note: note}
}

// labSex has different ranges for men and women.
func labSex(name, unit string, ml, mh, fl, fh float64) labAnalyteDef {
	return labAnalyteDef{Name: name, Unit: unit, Kind: "num", RefMale: labRng(ml, mh), RefFemale: labRng(fl, fh)}
}

func labText(name, expected string) labAnalyteDef {
	return labAnalyteDef{Name: name, Kind: "text", Expected: expected}
}

func labFree(name string) labAnalyteDef { return labAnalyteDef{Name: name, Kind: "text"} }

var labCatalogPanels = []labPanelDef{
	{ID: "bh", Name: "Biometría hemática", Audience: "person", Analytes: []labAnalyteDef{
		labSex("Hemoglobina", "g/dL", 13.5, 17.5, 12.0, 15.5),
		labSex("Hematocrito", "%", 41, 53, 36, 46),
		labSex("Eritrocitos", "x10^6/µL", 4.5, 5.9, 4.1, 5.1),
		labNum("Leucocitos", "x10^3/µL", 4.5, 11.0),
		labNum("Plaquetas", "x10^3/µL", 150, 400),
		labNum("VCM", "fL", 80, 100),
		labNum("HCM", "pg", 27, 33),
		labNum("CMHC", "g/dL", 32, 36),
		labNum("Neutrófilos", "%", 40, 70),
		labNum("Linfocitos", "%", 20, 40),
		labNum("Monocitos", "%", 2, 8),
		labNum("Eosinófilos", "%", 1, 4),
		labNum("Basófilos", "%", 0, 1),
	}},
	{ID: "qs6", Name: "Química sanguínea de 6 elementos", Audience: "person", Note: "La composición de la química de 6 varía entre laboratorios.", Analytes: []labAnalyteDef{
		labNum("Glucosa en ayuno", "mg/dL", 70, 99),
		labNum("Urea", "mg/dL", 15, 43),
		labSex("Creatinina", "mg/dL", 0.7, 1.3, 0.6, 1.1),
		labSex("Ácido úrico", "mg/dL", 3.4, 7.0, 2.4, 6.0),
		labMax("Colesterol total", "mg/dL", 200, "Deseable menor de 200"),
		labMax("Triglicéridos", "mg/dL", 150, "Deseable menor de 150"),
	}},
	{ID: "qs12", Name: "Química sanguínea de 12 elementos", Audience: "person", Note: "La composición de la química de 12 varía entre laboratorios.", Analytes: []labAnalyteDef{
		labNum("Glucosa en ayuno", "mg/dL", 70, 99),
		labNum("Urea", "mg/dL", 15, 43),
		labNum("BUN", "mg/dL", 7, 20),
		labSex("Creatinina", "mg/dL", 0.7, 1.3, 0.6, 1.1),
		labSex("Ácido úrico", "mg/dL", 3.4, 7.0, 2.4, 6.0),
		labMax("Colesterol total", "mg/dL", 200, "Deseable menor de 200"),
		labMax("Triglicéridos", "mg/dL", 150, "Deseable menor de 150"),
		labNum("Calcio", "mg/dL", 8.6, 10.2),
		labNum("Fósforo", "mg/dL", 2.5, 4.5),
		labNum("Sodio", "mEq/L", 136, 145),
		labNum("Potasio", "mEq/L", 3.5, 5.1),
		labNum("Cloro", "mEq/L", 98, 107),
	}},
	{ID: "lipidos", Name: "Perfil de lípidos", Audience: "person", Note: "Los límites son los deseables en adultos sin factores de riesgo adicionales.", Analytes: []labAnalyteDef{
		labMax("Colesterol total", "mg/dL", 200, "Deseable menor de 200"),
		labMax("LDL", "mg/dL", 100, "Óptimo menor de 100"),
		{Name: "HDL", Unit: "mg/dL", Kind: "num", RefMale: &labBound{Low: lbp(40)}, RefFemale: &labBound{Low: lbp(50)}, Note: "Deseable mayor de 40 (hombres) o 50 (mujeres)"},
		labMax("Triglicéridos", "mg/dL", 150, "Deseable menor de 150"),
		labMax("Colesterol no-HDL", "mg/dL", 130, "Deseable menor de 130"),
	}},
	{ID: "ego", Name: "Examen general de orina", Audience: "person", Analytes: []labAnalyteDef{
		labFree("Color"),
		labFree("Aspecto"),
		labNum("Densidad", "", 1.005, 1.030),
		labNum("pH", "", 5.0, 8.0),
		labText("Glucosa", "Negativo"),
		labText("Proteínas", "Negativo"),
		labText("Cetonas", "Negativo"),
		labText("Sangre", "Negativo"),
		labText("Nitritos", "Negativo"),
		labText("Leucocito esterasa", "Negativo"),
		labText("Bilirrubina", "Negativo"),
		labText("Urobilinógeno", "Normal"),
		labNum("Leucocitos (sedimento)", "por campo", 0, 5),
		labNum("Eritrocitos (sedimento)", "por campo", 0, 2),
		labFree("Células epiteliales"),
		labFree("Bacterias"),
		labFree("Cilindros"),
		labFree("Cristales"),
	}},
	{ID: "hepatico", Name: "Función hepática", Audience: "person", Analytes: []labAnalyteDef{
		labNum("ALT (TGP)", "U/L", 7, 56),
		labNum("AST (TGO)", "U/L", 10, 40),
		labNum("Fosfatasa alcalina", "U/L", 44, 147),
		labNum("GGT", "U/L", 9, 48),
		labNum("Bilirrubina total", "mg/dL", 0.1, 1.2),
		labNum("Bilirrubina directa", "mg/dL", 0.0, 0.3),
		labNum("Bilirrubina indirecta", "mg/dL", 0.2, 0.8),
		labNum("Proteínas totales", "g/dL", 6.0, 8.3),
		labNum("Albúmina", "g/dL", 3.5, 5.0),
	}},
	{ID: "tiroides", Name: "Tiroides básico", Audience: "person", Analytes: []labAnalyteDef{
		labNum("TSH", "mUI/L", 0.4, 4.0),
		labNum("T4 libre", "ng/dL", 0.8, 1.8),
		labNum("T4 total", "µg/dL", 5.0, 12.0),
		labNum("T3 total", "ng/dL", 80, 200),
	}},
	{ID: "hba1c", Name: "Hemoglobina glucosilada", Audience: "person", Analytes: []labAnalyteDef{
		labMax("HbA1c", "%", 5.7, "Menor de 5.7 normal; 5.7 a 6.4 prediabetes; 6.5 o más diabetes (ADA)"),
	}},

	// Veterinary. Intervals differ by species, analyzer and laboratory: they only orient, and the laboratory's own wins.
	{ID: "vet_hemo_perro", Name: "Hemograma de perro", Audience: "animal", Species: "Perro", Note: "Referencia general para perros adultos.", Analytes: []labAnalyteDef{
		labNum("Hematocrito", "%", 37, 55),
		labNum("Hemoglobina", "g/dL", 12, 18),
		labNum("Eritrocitos", "x10^6/µL", 5.5, 8.5),
		labNum("Leucocitos", "x10^3/µL", 6, 17),
		labNum("Plaquetas", "x10^3/µL", 200, 500),
	}},
	{ID: "vet_hemo_gato", Name: "Hemograma de gato", Audience: "animal", Species: "Gato", Note: "Referencia general para gatos adultos.", Analytes: []labAnalyteDef{
		labNum("Hematocrito", "%", 30, 45),
		labNum("Hemoglobina", "g/dL", 8, 15),
		labNum("Eritrocitos", "x10^6/µL", 5, 10),
		labNum("Leucocitos", "x10^3/µL", 5.5, 19.5),
		labNum("Plaquetas", "x10^3/µL", 175, 500),
	}},
	{ID: "vet_quim_perro", Name: "Química sanguínea de perro", Audience: "animal", Species: "Perro", Note: "Referencia general para perros adultos.", Analytes: []labAnalyteDef{
		labNum("Glucosa", "mg/dL", 70, 143),
		labNum("BUN", "mg/dL", 7, 27),
		labNum("Creatinina", "mg/dL", 0.5, 1.8),
		labNum("ALT", "U/L", 10, 125),
		labNum("Fosfatasa alcalina", "U/L", 23, 212),
		labNum("Proteínas totales", "g/dL", 5.2, 8.2),
		labNum("Albúmina", "g/dL", 2.5, 4.4),
		labNum("Calcio", "mg/dL", 7.9, 12.0),
		labNum("Fósforo", "mg/dL", 2.5, 6.8),
	}},
	{ID: "vet_quim_gato", Name: "Química sanguínea de gato", Audience: "animal", Species: "Gato", Note: "Referencia general para gatos adultos.", Analytes: []labAnalyteDef{
		labNum("Glucosa", "mg/dL", 74, 159),
		labNum("BUN", "mg/dL", 16, 36),
		labNum("Creatinina", "mg/dL", 0.8, 2.4),
		labNum("ALT", "U/L", 12, 130),
		labNum("Fosfatasa alcalina", "U/L", 14, 111),
		labNum("Proteínas totales", "g/dL", 5.7, 8.9),
		labNum("Albúmina", "g/dL", 2.2, 4.0),
		labNum("Calcio", "mg/dL", 7.8, 11.3),
		labNum("Fósforo", "mg/dL", 3.1, 7.5),
	}},
}

// labCatalogRef finds the catalog range of an analyte of a panel for a patient sex ("Hombre", "Mujer" or empty).
func labCatalogRef(panelName, analyte, sex string) (low, high *float64, unit string, ok bool) {
	for _, p := range labCatalogPanels {
		if p.Name != panelName && p.ID != panelName {
			continue
		}
		for _, a := range p.Analytes {
			if a.Name != analyte {
				continue
			}
			b := a.Ref
			switch {
			case sex == "Hombre" && a.RefMale != nil:
				b = a.RefMale
			case sex == "Mujer" && a.RefFemale != nil:
				b = a.RefFemale
			}
			if b == nil {
				return nil, nil, a.Unit, true
			}
			return b.Low, b.High, a.Unit, true
		}
	}
	return nil, nil, "", false
}
