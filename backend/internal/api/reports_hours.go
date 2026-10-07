package api

import "encoding/json"

// rptClinicHours reads the clinic's weekly hours out of clinics.settings.
func rptClinicHours(raw []byte) map[string]DayHours {
	var st Settings
	_ = json.Unmarshal(raw, &st)
	return st.normalized().Hours
}

// rptProHours reads professional_settings.hours: {"mon":[["09:00","14:00"]], ...}. Empty = use the clinic's.
func rptProHours(raw []byte) map[string][][2]string {
	m := map[string][][2]string{}
	_ = json.Unmarshal(raw, &m)
	return m
}
