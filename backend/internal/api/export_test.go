package api

// AITaskKeys lists the keys of the AI task registry for the external tests.
func AITaskKeys() []string {
	keys := make([]string, 0, len(aiTasks))
	for _, t := range aiTasks {
		keys = append(keys, t.Key)
	}
	return keys
}

// PatientMergeTables lists the tables patient_merge.go moves from one record to the other.
func PatientMergeTables() []string { return patientTables }

// SealedColumns is every column the code seals with encFields, as "table.column".
func SealedColumns() []string {
	var out []string
	for _, c := range encounterSealed {
		out = append(out, "encounters."+c)
	}
	for _, c := range rxSealed {
		out = append(out, "prescriptions."+c)
	}
	for _, c := range certSealed {
		out = append(out, "certificates."+c)
	}
	return out
}
