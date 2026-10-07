package api

// Veterinary reference list, by species. Doses are typical references per kg of body weight.
func vetMeds() []catMed {
	const (
		aine   = "AINE"
		abx    = "Antibiótico"
		cort   = "Corticoide"
		gi     = "Gastrointestinal"
		cardio = "Cardiovascular"
		antip  = "Antiparasitario"
		antim  = "Antimicótico"
		antih  = "Antialérgico / dermatología"
		neuro  = "Neurología"
		analg  = "Analgésico"
		no     = "No"
		dc     = "Perro,Gato"
	)
	return []catMed{
		// Antiinflamatorios y analgésicos
		vet("Perro", "Meloxicam", aine, no, "Oral", "0.2 mg/kg el primer día y 0.1 mg/kg cada 24 h", "Suspensión 1.5 mg/mL", "Solución inyectable 5 mg/mL", "Tabletas 1 mg", "Tabletas 2.5 mg").kg(0.1, 0.2).conc("Suspensión 1.5 mg/mL", 1.5),
		vet("Gato", "Meloxicam", aine, no, "Oral", "0.1 mg/kg el primer día y 0.05 mg/kg cada 24 h", "Suspensión 0.5 mg/mL", "Solución inyectable 5 mg/mL").kg(0.05, 0.1).conc("Suspensión 0.5 mg/mL", 0.5).note("Uso prolongado en gatos solo con vigilancia renal."),
		vet("Perro", "Carprofeno", aine, no, "Oral", "2.2 mg/kg cada 12 h o 4.4 mg/kg cada 24 h", "Tabletas 25 mg", "Tabletas 75 mg", "Tabletas 100 mg").kg(2.2, 4.4),
		vet("Perro", "Firocoxib", aine, no, "Oral", "5 mg/kg cada 24 h", "Tabletas 57 mg", "Tabletas 227 mg").kg(5, 5),
		vet("Perro,Gato", "Robenacoxib", aine, no, "Oral", "1 a 2 mg/kg cada 24 h", "Tabletas").kg(1, 2.4),
		vet("Perro,Gato", "Metamizol sódico", analg, no, "Oral", "25 mg/kg cada 8-12 h", "Solución inyectable 500 mg/mL", "Gotas").kg(25, 0),
		vet("Perro", "Tramadol", analg, "Fracción III", "Oral", "2 a 5 mg/kg cada 8-12 h", "Tabletas 50 mg", "Solución en gotas 100 mg/mL").kg(3, 15),
		vet("Perro", "Gabapentina", neuro, no, "Oral", "10 mg/kg cada 8-12 h", "Cápsulas 100 mg", "Cápsulas 300 mg", "Solución 50 mg/mL").kg(10, 30).conc("Solución 50 mg/mL", 50),
		vet("Gato", "Gabapentina", neuro, no, "Oral", "5 a 10 mg/kg cada 12 h", "Cápsulas 100 mg", "Solución 50 mg/mL").kg(7.5, 20).conc("Solución 50 mg/mL", 50),
		// Antibióticos
		vet(dc, "Amoxicilina", abx, "Antibiótico", "Oral", "10 a 20 mg/kg cada 12 h", "Tabletas 50 mg", "Tabletas 250 mg", "Suspensión 50 mg/mL").kg(15, 40).conc("Suspensión 50 mg/mL", 50),
		vet(dc, "Amoxicilina / ácido clavulánico", abx, "Antibiótico", "Oral", "12.5 a 25 mg/kg cada 12 h (suma de ambos)", "Tabletas 62.5 mg", "Tabletas 250 mg", "Tabletas 500 mg", "Suspensión").kg(12.5, 50).note("mg/kg de la suma de amoxicilina y clavulanato."),
		vet(dc, "Cefalexina", abx, "Antibiótico", "Oral", "22 a 30 mg/kg cada 12 h", "Tabletas 300 mg", "Tabletas 600 mg", "Suspensión").kg(25, 60),
		vet(dc, "Cefovecina", abx, "Antibiótico", "Subcutánea", "8 mg/kg dosis única; repetible a los 14 días", "Frasco ámpula 800 mg").kg(8, 8),
		vet("Perro", "Enrofloxacina", abx, "Antibiótico", "Oral", "5 a 20 mg/kg cada 24 h", "Tabletas 50 mg", "Tabletas 150 mg", "Solución inyectable 5%", "Solución inyectable 10%").kg(5, 20),
		vet("Gato", "Enrofloxacina", abx, "Antibiótico", "Oral", "5 mg/kg cada 24 h; no exceder", "Tabletas 15 mg", "Tabletas 50 mg", "Solución inyectable 5%").kg(5, 5).note("Dosis mayores a 5 mg/kg/día pueden causar ceguera en gatos."),
		vet(dc, "Doxiciclina", abx, "Antibiótico", "Oral", "5 a 10 mg/kg cada 12 h", "Tabletas 100 mg", "Suspensión").kg(5, 20),
		vet(dc, "Metronidazol", abx, "Antibiótico", "Oral", "10 a 15 mg/kg cada 12 h", "Tabletas 250 mg", "Tabletas 500 mg", "Suspensión 125 mg/5 mL").kg(12.5, 30),
		vet(dc, "Clindamicina", abx, "Antibiótico", "Oral", "5 a 11 mg/kg cada 12 h", "Cápsulas 75 mg", "Cápsulas 150 mg", "Cápsulas 300 mg").kg(8, 22),
		vet(dc, "Trimetoprima / sulfametoxazol", abx, "Antibiótico", "Oral", "15 a 30 mg/kg cada 12 h (suma de ambos)", "Tabletas 480 mg", "Suspensión").kg(20, 60),
		// Corticoides
		vet("Perro", "Prednisona", cort, no, "Oral", "0.5 a 1 mg/kg cada 24 h (antiinflamatorio)", "Tabletas 5 mg", "Tabletas 20 mg").kg(0.5, 2),
		vet("Gato", "Prednisolona", cort, no, "Oral", "0.5 a 1 mg/kg cada 12-24 h (antiinflamatorio)", "Tabletas 5 mg", "Tabletas 20 mg").kg(0.5, 2),
		vet(dc, "Dexametasona", cort, no, "Oral", "0.1 a 0.2 mg/kg cada 24 h", "Tabletas 0.5 mg", "Solución inyectable 2 mg/mL").kg(0.1, 0.3),
		// Gastrointestinal
		vet(dc, "Maropitant", gi, no, "Oral", "2 mg/kg cada 24 h (perro); 1 mg/kg cada 24 h (gato)", "Tabletas 16 mg", "Tabletas 24 mg", "Tabletas 60 mg", "Solución inyectable 10 mg/mL").kg(1, 0),
		vet(dc, "Ondansetrón", gi, no, "Oral", "0.5 mg/kg cada 12 h", "Tabletas 4 mg", "Ampolleta 8 mg/4 mL").kg(0.5, 1),
		vet(dc, "Metoclopramida", gi, no, "Oral", "0.2 a 0.5 mg/kg cada 8 h", "Tabletas 10 mg", "Solución inyectable 5 mg/mL").kg(0.3, 1.5),
		vet(dc, "Omeprazol", gi, no, "Oral", "0.7 a 1 mg/kg cada 24 h", "Cápsulas 20 mg").kg(0.8, 2),
		vet(dc, "Famotidina", gi, no, "Oral", "0.5 a 1 mg/kg cada 12-24 h", "Tabletas 10 mg", "Tabletas 20 mg").kg(0.5, 2),
		// Cardiovascular
		vet("Perro", "Pimobendán", cardio, no, "Oral", "0.25 mg/kg cada 12 h", "Tabletas 1.25 mg", "Tabletas 5 mg", "Cápsulas 2.5 mg").kg(0.25, 0.6),
		vet("Perro", "Enalapril", cardio, no, "Oral", "0.5 mg/kg cada 12-24 h", "Tabletas 5 mg", "Tabletas 10 mg").kg(0.5, 1),
		vet("Perro", "Benazepril", cardio, no, "Oral", "0.25 a 0.5 mg/kg cada 24 h", "Tabletas 5 mg", "Tabletas 20 mg").kg(0.5, 0.5),
		vet(dc, "Furosemida", cardio, no, "Oral", "1 a 4 mg/kg cada 8-12 h según cuadro", "Tabletas 40 mg", "Solución inyectable 50 mg/mL").kg(2, 0),
		vet("Perro", "Espironolactona", cardio, no, "Oral", "1 a 2 mg/kg cada 12-24 h", "Tabletas 25 mg").kg(1, 4),
		// Antiparasitarios
		vet(dc, "Praziquantel", antip, no, "Oral", "5 mg/kg dosis única", "Tabletas 50 mg", "Tabletas 150 mg").kg(5, 0),
		vet(dc, "Pamoato de pirantel", antip, no, "Oral", "14.4 mg/kg (equivale a 5 mg/kg de pirantel base) dosis única", "Suspensión 50 mg/mL", "Tabletas").kg(14.4, 0),
		vet(dc, "Fenbendazol", antip, no, "Oral", "50 mg/kg cada 24 h por 3 a 5 días", "Suspensión 10%", "Tabletas 500 mg").kg(50, 0),
		vet("Perro", "Ivermectina", antip, no, "Oral", "6 mcg/kg mensual (dirofilariasis); 0.3 mg/kg en sarna", "Solución 1%", "Tabletas").kg(0.006, 0.4).note("No usar en razas con mutación MDR1 sin pruebas (Collie, Pastor, etc.) a dosis de sarna."),
		vet("Perro", "Afoxolaner", antip, no, "Oral", "2.5 mg/kg cada mes", "Tabletas masticables").kg(2.5, 0),
		vet("Perro", "Fluralaner", antip, no, "Oral", "25 mg/kg cada 12 semanas", "Tabletas masticables").kg(25, 0),
		vet(dc, "Selamectina", antip, no, "Tópica", "6 mg/kg cada mes", "Solución tópica spot-on").kg(6, 0),
		vet("Perro", "Milbemicina oxima", antip, no, "Oral", "0.5 mg/kg cada mes", "Tabletas").kg(0.5, 0),
		// Antimicóticos, alergia y neurología
		vet(dc, "Itraconazol", antim, no, "Oral", "5 a 10 mg/kg cada 24 h", "Cápsulas 100 mg", "Solución 10 mg/mL").kg(5, 10).conc("Solución 10 mg/mL", 10),
		vet(dc, "Ketoconazol", antim, no, "Oral", "5 a 10 mg/kg cada 12-24 h", "Tabletas 200 mg").kg(7.5, 20),
		vet(dc, "Fluconazol", antim, no, "Oral", "5 a 10 mg/kg cada 12-24 h", "Cápsulas 100 mg", "Suspensión").kg(5, 20),
		vet("Perro", "Oclacitinib", antih, no, "Oral", "0.4 a 0.6 mg/kg cada 12 h por 14 días y luego cada 24 h", "Tabletas 3.6 mg", "Tabletas 5.4 mg", "Tabletas 16 mg").kg(0.5, 1.2),
		vet(dc, "Cetirizina", antih, no, "Oral", "1 mg/kg cada 24 h", "Tabletas 10 mg", "Solución").kg(1, 2),
		vet(dc, "Difenhidramina", antih, no, "Oral", "2 mg/kg cada 8-12 h", "Tabletas 25 mg", "Jarabe").kg(2, 6),
		vet("Perro", "Fenobarbital", neuro, "Fracción III", "Oral", "2.5 mg/kg cada 12 h", "Tabletas 15 mg", "Tabletas 100 mg").kg(2.5, 0),
		vet("Perro", "Levetiracetam", neuro, no, "Oral", "20 mg/kg cada 8 h", "Tabletas 500 mg", "Solución 100 mg/mL").kg(20, 0).conc("Solución 100 mg/mL", 100),
		// Conejo
		vet("Conejo", "Meloxicam", aine, no, "Oral", "0.3 a 0.5 mg/kg cada 24 h", "Suspensión 1.5 mg/mL", "Solución inyectable 5 mg/mL").kg(0.4, 0.6).conc("Suspensión 1.5 mg/mL", 1.5),
		vet("Conejo", "Enrofloxacina", abx, "Antibiótico", "Oral", "10 mg/kg cada 12 h", "Solución inyectable 5%", "Tabletas 50 mg").kg(10, 20),
		vet("Conejo", "Fenbendazol", antip, no, "Oral", "20 mg/kg cada 24 h por 5 días", "Suspensión 10%").kg(20, 0),
		vet("Conejo", "Metoclopramida", gi, no, "Oral", "0.5 mg/kg cada 8-12 h", "Tabletas 10 mg", "Solución inyectable 5 mg/mL").kg(0.5, 0),
	}
}
