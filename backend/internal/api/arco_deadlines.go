package api

import "time"

// ARCO deadlines.
//
// The LFPDPPP gives the responsable 20 días to communicate the determination about an ARCO request and
// 15 días more to carry it out when it is procedente. This file counts those days as business days
// (Monday to Friday) with NO holiday calendar: Mexican holidays and each clinic's non-working days are
// ignored, so the result can be a day or two earlier than the legal one. It is deliberately isolated here so
// a holiday calendar can replace arcoIsBusinessDay later. The numbers shown to users are a guide: the
// clinic must confirm the deadlines with its legal advisor.
const (
	arcoAckDays     = 3  // internal target to acknowledge receipt (not a legal deadline)
	arcoAnswerDays  = 20 // days to communicate the determination
	arcoExecuteDays = 15 // days to make it effective after a procedente determination
	arcoSoonDays    = 3  // business days left that count as "soon"
)

// arcoIsBusinessDay reports whether d counts as a day for deadlines. Replace this to add holidays.
func arcoIsBusinessDay(d time.Time) bool {
	wd := d.Weekday()
	return wd != time.Saturday && wd != time.Sunday
}

// arcoDay returns the calendar date of t in loc as midnight UTC, so dates compare and format the same everywhere.
func arcoDay(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// arcoAddBusinessDays returns the date n business days after from (day 1 is the next business day;
// the day of reception does not count).
func arcoAddBusinessDays(from time.Time, n int) time.Time {
	d := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	for n > 0 {
		d = d.AddDate(0, 0, 1)
		if arcoIsBusinessDay(d) {
			n--
		}
	}
	return d
}

// arcoBusinessDaysLeft counts the business days from today until due: positive when due is ahead,
// 0 when due is today and negative (days overdue) when it has passed.
func arcoBusinessDaysLeft(today, due time.Time) int {
	if due.Equal(today) {
		return 0
	}
	sign := 1
	a, b := today, due
	if due.Before(today) {
		sign, a, b = -1, due, today
	}
	n := 0
	for d := a.AddDate(0, 0, 1); !d.After(b); d = d.AddDate(0, 0, 1) {
		if arcoIsBusinessDay(d) {
			n++
		}
	}
	if sign < 0 && n == 0 {
		n = 1 // vencida el viernes y hoy es fin de semana: sigue vencida, no «por vencer»
	}
	return sign * n
}

// arcoDue holds the deadlines computed at reception.
type arcoDue struct {
	Ack, Answer time.Time
}

func arcoDeadlines(received time.Time) arcoDue {
	return arcoDue{Ack: arcoAddBusinessDays(received, arcoAckDays), Answer: arcoAddBusinessDays(received, arcoAnswerDays)}
}

// arcoState classifies a deadline: "overdue", "soon" or "ok".
func arcoState(left int) string {
	switch {
	case left < 0:
		return "overdue"
	case left <= arcoSoonDays:
		return "soon"
	}
	return "ok"
}
