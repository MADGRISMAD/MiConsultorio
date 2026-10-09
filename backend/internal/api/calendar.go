package api

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

// Calendar. The e-mails of an appointment carry an .ics file (opens in Google Calendar, Outlook and Apple Calendar,
// and Gmail offers to add it) and a link that adds the visit to Google Calendar. The event keeps the same UID, and its
// SEQUENCE grows with every change, so a rescheduled visit updates the one already in the calendar.

const calendarDefaultMinutes = 30

func (a apptInfo) hasCalendar() bool { return a.ID != "" && !a.Start.IsZero() }

func (a apptInfo) endTime() time.Time {
	if a.End.After(a.Start) {
		return a.End
	}
	return a.Start.Add(calendarDefaultMinutes * time.Minute)
}

func (a apptInfo) calendarTitle() string {
	who := "Cita"
	if a.PetName != "" {
		who = "Cita de " + a.PetName
	}
	return who + " en " + a.ClinicName
}

func (a apptInfo) calendarWhere() string {
	switch {
	case a.ClinicAddress != "":
		return a.ClinicAddress
	case a.VideoURL != "":
		return "Videollamada"
	}
	return a.ClinicName
}

func (a apptInfo) calendarNotes() string {
	var lines []string
	add := func(label, v string) {
		if v != "" {
			lines = append(lines, label+": "+v)
		}
	}
	add("Atiende", a.Professional)
	add("Servicio", a.Service)
	add("Videollamada", a.VideoURL)
	add("Consultorio", a.ClinicName)
	add("Teléfono", a.ClinicPhone)
	return strings.Join(lines, "\n")
}

// icsText escapes a value for an iCalendar text field.
func icsText(s string) string {
	return strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\r\n", `\n`, "\n", `\n`, "\r", "").Replace(s)
}

// icsFold wraps a content line at 75 octets, as the format asks.
func icsFold(line string) string {
	var b strings.Builder
	n := 0
	for _, r := range line {
		size := len(string(r))
		if n+size > 74 {
			b.WriteString("\r\n ")
			n = 1
		}
		b.WriteRune(r)
		n += size
	}
	return b.String()
}

const icsStamp = "20060102T150405Z"

// calendarFile builds the .ics of the visit. tentative marks a request the clinic still has to accept.
func (a apptInfo) calendarFile(tentative bool, manage string) mail.Attachment {
	status := "CONFIRMED"
	if tentative {
		status = "TENTATIVE"
	}
	seq := a.Modified.Unix()
	if seq < 0 {
		seq = 0
	}
	lines := []string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Caresia//Citas//ES", "CALSCALE:GREGORIAN", "METHOD:PUBLISH",
		"BEGIN:VEVENT",
		"UID:cita-" + a.ID + "@caresia",
		"DTSTAMP:" + time.Now().UTC().Format(icsStamp),
		"SEQUENCE:" + fmt.Sprint(seq),
		"DTSTART:" + a.Start.UTC().Format(icsStamp),
		"DTEND:" + a.endTime().UTC().Format(icsStamp),
		"SUMMARY:" + icsText(a.calendarTitle()),
		"LOCATION:" + icsText(a.calendarWhere()),
		"DESCRIPTION:" + icsText(a.calendarNotes()),
		"STATUS:" + status,
	}
	if manage != "" {
		lines = append(lines, "URL:"+manage)
	}
	lines = append(lines,
		"BEGIN:VALARM", "ACTION:DISPLAY", "DESCRIPTION:"+icsText("Tienes una cita en "+a.ClinicName), "TRIGGER:-PT2H", "END:VALARM",
		"END:VEVENT", "END:VCALENDAR")
	for i, l := range lines {
		lines[i] = icsFold(l)
	}
	return mail.Attachment{Name: "cita.ics", ContentType: "text/calendar; charset=UTF-8; method=PUBLISH", Data: []byte(strings.Join(lines, "\r\n") + "\r\n")}
}

// googleCalendarURL opens Google Calendar with the visit ready to save.
func (a apptInfo) googleCalendarURL() string {
	q := url.Values{}
	q.Set("action", "TEMPLATE")
	q.Set("text", a.calendarTitle())
	q.Set("dates", a.Start.UTC().Format("20060102T150405Z")+"/"+a.endTime().UTC().Format("20060102T150405Z"))
	q.Set("details", a.calendarNotes())
	q.Set("location", a.calendarWhere())
	return "https://calendar.google.com/calendar/render?" + q.Encode()
}

// calendarAttachments are the files to add to the e-mail of this visit (none when it lacks the data).
func (a apptInfo) calendarAttachments(tentative bool, manage string) []mail.Attachment {
	if !a.hasCalendar() {
		return nil
	}
	return []mail.Attachment{a.calendarFile(tentative, manage)}
}

// calendarHTML / calendarText are the line that offers the calendar in the body of the e-mail.
func (a apptInfo) calendarHTML() string {
	if !a.hasCalendar() {
		return ""
	}
	return `<p style="margin:14px 0 0;font-size:14px">📅 ` + smallLink(a.googleCalendarURL(), "Agregar a Google Calendar") +
		` <span style="color:#7a8b9b">· o abre el archivo adjunto «cita.ics» (Outlook, Apple Calendar).</span></p>`
}

func (a apptInfo) calendarText() string {
	if !a.hasCalendar() {
		return ""
	}
	return "\nAgregar a Google Calendar: " + a.googleCalendarURL() + "\n(También va adjunto el archivo cita.ics para Outlook o Apple Calendar.)\n"
}
