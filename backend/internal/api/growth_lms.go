package api

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Growth references are never written in this code: an administrator loads the official WHO/CDC tables from a CSV.
// This file holds the maths (LMS Z-scores, interpolation) and the strict CSV validation.

var growthIndicators = []string{"weight_for_age", "length_height_for_age", "bmi_for_age", "head_circumference_for_age"}

// growthPctKeys are the percentile columns a CSV may carry (p1 ... p99), in ascending order.
var growthPctKeys = []int{1, 3, 5, 10, 15, 25, 50, 75, 85, 90, 95, 97, 99}

const (
	growthMaxRows      = 20000
	growthMaxAgeMonths = 240.0
)

// growthZ is the Z-score of x under the LMS method: ((x/M)^L - 1) / (L*S), or ln(x/M)/S when L is 0.
func growthZ(x, l, m, s float64) (float64, bool) {
	if x <= 0 || m <= 0 || s <= 0 || math.IsNaN(l) || math.IsInf(l, 0) {
		return 0, false
	}
	var z float64
	if math.Abs(l) < 1e-9 {
		z = math.Log(x/m) / s
	} else {
		z = (math.Pow(x/m, l) - 1) / (l * s)
	}
	if math.IsNaN(z) || math.IsInf(z, 0) {
		return 0, false
	}
	return z, true
}

// growthValueAtZ inverts growthZ: the measurement that has the given Z-score.
func growthValueAtZ(z, l, m, s float64) (float64, bool) {
	var x float64
	if math.Abs(l) < 1e-9 {
		x = m * math.Exp(s*z)
	} else {
		base := 1 + l*s*z
		if base <= 0 {
			return 0, false
		}
		x = m * math.Pow(base, 1/l)
	}
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0, false
	}
	return x, true
}

// growthPercentile converts a Z-score to a percentile (0 to 100) of the standard normal distribution.
func growthPercentile(z float64) float64 { return 50 * math.Erfc(-z/math.Sqrt2) }

// growthZFromPercentile is the Z-score of a percentile (0 < p < 100).
func growthZFromPercentile(p float64) float64 { return math.Sqrt2 * math.Erfinv(2*p/100-1) }

// growthRef is one row of a reference table.
type growthRef struct {
	Age     float64
	L, M, S *float64
	Pcts    map[int]float64
}

func (g growthRef) hasLMS() bool { return g.L != nil && g.M != nil && g.S != nil }

// growthInterp interpolates linearly between the two rows around age. Ages outside the table give false:
// a reference is never extrapolated.
func growthInterp(rows []growthRef, age float64) (growthRef, bool) {
	if len(rows) == 0 || age < rows[0].Age || age > rows[len(rows)-1].Age {
		return growthRef{}, false
	}
	i := sort.Search(len(rows), func(i int) bool { return rows[i].Age >= age })
	if rows[i].Age == age || i == 0 {
		return rows[i], true
	}
	a, b := rows[i-1], rows[i]
	t := (age - a.Age) / (b.Age - a.Age)
	lerp := func(x, y float64) float64 { return x + (y-x)*t }
	out := growthRef{Age: age}
	if a.hasLMS() && b.hasLMS() {
		l, m, s := lerp(*a.L, *b.L), lerp(*a.M, *b.M), lerp(*a.S, *b.S)
		out.L, out.M, out.S = &l, &m, &s
	}
	if len(a.Pcts) > 0 && len(b.Pcts) > 0 {
		out.Pcts = map[int]float64{}
		for k, va := range a.Pcts {
			if vb, ok := b.Pcts[k]; ok {
				out.Pcts[k] = lerp(va, vb)
			}
		}
	}
	return out, true
}

// growthScore gives the Z-score and percentile of a measurement. Rows with LMS use it; rows with percentiles only
// are interpolated between the two surrounding percentiles (outside the table's own range there is no score).
func growthScore(ref growthRef, x float64) (z, pct float64, ok bool) {
	if ref.hasLMS() {
		z, ok = growthZ(x, *ref.L, *ref.M, *ref.S)
		if !ok {
			return 0, 0, false
		}
		return z, growthPercentile(z), true
	}
	keys := make([]int, 0, len(ref.Pcts))
	for k := range ref.Pcts {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for i := 0; i+1 < len(keys); i++ {
		lo, hi := ref.Pcts[keys[i]], ref.Pcts[keys[i+1]]
		if x >= lo && x <= hi && hi > lo {
			p := float64(keys[i]) + (x-lo)/(hi-lo)*float64(keys[i+1]-keys[i])
			return growthZFromPercentile(p), p, true
		}
	}
	return 0, 0, false
}

// ---- CSV ---------------------------------------------------------------------------------------------------------

type growthCSVRow struct {
	Line      int
	Indicator string
	Sex       string
	Age       float64
	L, M, S   *float64
	Pcts      map[int]float64
}

type growthCSVError struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

type growthCSVResult struct {
	Rows    []growthCSVRow
	Errors  []growthCSVError
	Total   int // errors found, even when only the first ones are listed
	HasLMS  bool
	Columns []string
}

const growthMaxListedErrors = 25

func (res *growthCSVResult) fail(line int, format string, args ...any) {
	res.Total++
	if len(res.Errors) < growthMaxListedErrors {
		res.Errors = append(res.Errors, growthCSVError{Line: line, Message: fmt.Sprintf(format, args...)})
	}
}

// growthParseNumber accepts plain decimal numbers only (no thousands separators, no decimal comma).
func growthParseNumber(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

// parseGrowthCSV validates a reference table strictly: known columns only, complete rows, no duplicates, sane values.
func parseGrowthCSV(text string) growthCSVResult {
	res := growthCSVResult{}
	text = strings.TrimPrefix(text, "\xef\xbb\xbf")
	rd := csv.NewReader(strings.NewReader(text))
	rd.FieldsPerRecord = -1
	rd.TrimLeadingSpace = true
	header, err := rd.Read()
	if err != nil {
		res.fail(1, "El archivo está vacío o la primera fila no es válida.")
		return res
	}
	col := map[string]int{}
	for i, h := range header {
		h = strings.ToLower(strings.TrimSpace(h))
		header[i] = h
		allowed := h == "indicator" || h == "sex" || h == "age_months" || h == "l" || h == "m" || h == "s"
		if !allowed && strings.HasPrefix(h, "p") {
			if n, err := strconv.Atoi(h[1:]); err == nil && slices.Contains(growthPctKeys, n) && strconv.Itoa(n) == h[1:] {
				allowed = true
			}
		}
		switch {
		case !allowed:
			res.fail(1, "Columna no reconocida: «%s». Usa indicator, sex, age_months, l, m, s y p1, p3, p5, p10, p15, p25, p50, p75, p85, p90, p95, p97, p99.", h)
		case col[h] != 0:
			res.fail(1, "La columna «%s» está repetida.", h)
		default:
			col[h] = i + 1
		}
	}
	res.Columns = header
	for _, req := range []string{"indicator", "sex", "age_months"} {
		if col[req] == 0 {
			res.fail(1, "Falta la columna obligatoria «%s».", req)
		}
	}
	nLMS := 0
	for _, k := range []string{"l", "m", "s"} {
		if col[k] != 0 {
			nLMS++
		}
	}
	var pctCols []int
	for _, k := range growthPctKeys {
		if col["p"+strconv.Itoa(k)] != 0 {
			pctCols = append(pctCols, k)
		}
	}
	if nLMS != 0 && nLMS != 3 {
		res.fail(1, "Las columnas l, m y s deben venir juntas.")
	}
	if nLMS == 0 && len(pctCols) == 0 {
		res.fail(1, "Incluye las columnas l, m, s o al menos una columna de percentil (p3, p50, p97...).")
	}
	if res.Total > 0 {
		return res // the header is wrong: reading the rows would only repeat the problem
	}
	res.HasLMS = nLMS == 3

	seen := map[string]bool{}
	cell := func(rec []string, name string) string {
		if i := col[name]; i > 0 && i <= len(rec) {
			return strings.TrimSpace(rec[i-1])
		}
		return ""
	}
	for {
		rec, err := rd.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			var pe *csv.ParseError
			if errors.As(err, &pe) {
				res.fail(pe.StartLine, "La fila no se pudo leer: %v", pe.Err)
			} else {
				res.fail(0, "La fila no se pudo leer.")
			}
			continue
		}
		line, _ := rd.FieldPos(0)
		if len(rec) == 1 && strings.TrimSpace(rec[0]) == "" {
			continue
		}
		if len(res.Rows)+res.Total >= growthMaxRows {
			res.fail(line, "El archivo supera el máximo de %d filas.", growthMaxRows)
			break
		}
		if len(rec) != len(header) {
			res.fail(line, "La fila tiene %d columnas y el encabezado %d.", len(rec), len(header))
			continue
		}
		row := growthCSVRow{Line: line, Indicator: strings.ToLower(cell(rec, "indicator")), Sex: strings.ToUpper(cell(rec, "sex"))}
		bad := false
		if !slices.Contains(growthIndicators, row.Indicator) {
			res.fail(line, "Indicador no válido: «%s». Usa %s.", row.Indicator, strings.Join(growthIndicators, ", "))
			bad = true
		}
		if row.Sex != "M" && row.Sex != "F" {
			res.fail(line, "El sexo debe ser M o F.")
			bad = true
		}
		age, ok := growthParseNumber(cell(rec, "age_months"))
		if !ok || age < 0 || age > growthMaxAgeMonths {
			res.fail(line, "age_months debe ser un número entre 0 y %d.", int(growthMaxAgeMonths))
			bad = true
		}
		row.Age = age
		if res.HasLMS {
			ls, ms, ss := cell(rec, "l"), cell(rec, "m"), cell(rec, "s")
			if ls != "" || ms != "" || ss != "" {
				l, okL := growthParseNumber(ls)
				m, okM := growthParseNumber(ms)
				s, okS := growthParseNumber(ss)
				switch {
				case !okL || !okM || !okS:
					res.fail(line, "L, M y S deben ser números y venir completos.")
					bad = true
				case m <= 0 || s <= 0:
					res.fail(line, "M y S deben ser mayores que cero.")
					bad = true
				default:
					row.L, row.M, row.S = &l, &m, &s
				}
			}
		}
		prev, prevKey := 0.0, 0
		for _, k := range pctCols {
			v := cell(rec, "p"+strconv.Itoa(k))
			if v == "" {
				continue
			}
			x, ok := growthParseNumber(v)
			if !ok || x <= 0 {
				res.fail(line, "p%d debe ser un número mayor que cero.", k)
				bad = true
				continue
			}
			if prevKey != 0 && x < prev {
				res.fail(line, "Los percentiles deben ir en orden creciente (p%d es menor que p%d).", k, prevKey)
				bad = true
				continue
			}
			if row.Pcts == nil {
				row.Pcts = map[int]float64{}
			}
			row.Pcts[k] = x
			prev, prevKey = x, k
		}
		if !bad && row.L == nil && len(row.Pcts) == 0 {
			res.fail(line, "La fila no trae L, M, S ni percentiles.")
			bad = true
		}
		if bad {
			continue
		}
		key := fmt.Sprintf("%s|%s|%v", row.Indicator, row.Sex, row.Age)
		if seen[key] {
			res.fail(line, "Fila repetida para %s, sexo %s, edad %v meses.", row.Indicator, row.Sex, row.Age)
			continue
		}
		seen[key] = true
		res.Rows = append(res.Rows, row)
	}
	if res.Total == 0 && len(res.Rows) == 0 {
		res.fail(1, "El archivo no tiene filas de datos.")
	}
	return res
}
