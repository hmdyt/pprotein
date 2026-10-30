package ai

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

func shortHash(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	return h
}

func oneLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

func formatAny(v any) string {
	switch x := v.(type) {
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return fmt.Sprint(x)
	}
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}

// round keeps JSON values short without losing meaningful precision.
func round(f float64, digits int) float64 {
	p := math.Pow(10, float64(digits))
	return math.Round(f*p) / p
}

func fmtSec(v any) string { return fmt.Sprintf("%.3fs", toFloat(v)) }
func fmtPct(v any) string { return fmt.Sprintf("%.1f%%", toFloat(v)*100) }
func fmtInt(v any) string { return strconv.FormatInt(int64(math.Round(toFloat(v))), 10) }
func fmtNum(v any) string { return strconv.FormatFloat(round(toFloat(v), 1), 'f', -1, 64) }

func fmtDeltaSec(v any) string { return fmt.Sprintf("%+.3fs", toFloat(v)) }
func fmtDeltaInt(v any) string { return fmt.Sprintf("%+d", int64(math.Round(toFloat(v)))) }
func fmtDeltaPct(v any) string {
	f := toFloat(v)
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return "n/a"
	}
	return fmt.Sprintf("%+.1f%%", f*100)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if n <= 0 || len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// shortPath keeps the last two path elements of a source file.
func shortPath(p string) string {
	parts := strings.Split(p, "/")
	if len(parts) <= 2 {
		return p
	}
	return strings.Join(parts[len(parts)-2:], "/")
}

func ratio(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

// change is the relative change from a to b; +Inf when a is zero.
func change(a, b float64) float64 {
	if a == 0 {
		if b == 0 {
			return 0
		}
		return math.Inf(1)
	}
	return (b - a) / a
}

type tsv struct {
	header map[string]int
	rows   [][]string
}

// parseTSV reads alp/slp TSV output; column lookup is case-insensitive.
func parseTSV(raw string) *tsv {
	t := &tsv{header: map[string]int{}}
	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return t
	}
	for i, h := range strings.Split(lines[0], "\t") {
		t.header[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, l := range lines[1:] {
		if l == "" {
			continue
		}
		t.rows = append(t.rows, strings.Split(l, "\t"))
	}
	return t
}

func (t *tsv) has(col string) bool {
	_, ok := t.header[strings.ToLower(col)]
	return ok
}

func (t *tsv) str(row []string, col string) string {
	i, ok := t.header[strings.ToLower(col)]
	if !ok || i >= len(row) {
		return ""
	}
	return row[i]
}

func (t *tsv) num(row []string, col string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(t.str(row, col)), 64)
	return f
}

// limitRows sorts rows by key (descending) and cuts them to n, reporting
// how many rows were dropped and, when total > 0, their share of it.
func limitRows(rows []map[string]any, key string, n int, total float64) ([]map[string]any, *Omitted) {
	return cutRows(rows, func(r map[string]any) float64 { return toFloat(r[key]) }, n, total)
}

// cutRows is limitRows with an arbitrary sort key (e.g. absolute delta).
func cutRows(rows []map[string]any, key func(map[string]any) float64, n int, total float64) ([]map[string]any, *Omitted) {
	sort.SliceStable(rows, func(i, j int) bool {
		ki, kj := key(rows[i]), key(rows[j])
		if ki != kj {
			return ki > kj
		}
		return fmt.Sprint(rows[i]) < fmt.Sprint(rows[j])
	})
	if n <= 0 || len(rows) <= n {
		return rows, nil
	}
	om := &Omitted{Rows: len(rows) - n}
	if total > 0 {
		rest := 0.0
		for _, r := range rows[n:] {
			rest += key(r)
		}
		share := round(rest/total, 4)
		om.Share = &share
	}
	return rows[:n], om
}
