package ai

import (
	"strings"
)

// httplog: alp TSV aggregated by method + URI.

type httpRow struct {
	Method, URI             string
	Count, S2xx, S4xx, S5xx float64
	Sum, Avg, P99, Max      float64
	hasP99, hasStatus       bool
}

func parseHTTPLog(raw string) []httpRow {
	t := parseTSV(raw)
	var rows []httpRow
	for _, r := range t.rows {
		rows = append(rows, httpRow{
			Method:    t.str(r, "method"),
			URI:       t.str(r, "uri"),
			Count:     t.num(r, "count"),
			S2xx:      t.num(r, "2xx"),
			S4xx:      t.num(r, "4xx"),
			S5xx:      t.num(r, "5xx"),
			Sum:       t.num(r, "sum"),
			Avg:       t.num(r, "avg"),
			P99:       t.num(r, "p99"),
			Max:       t.num(r, "max"),
			hasP99:    t.has("p99"),
			hasStatus: t.has("2xx"),
		})
	}
	return rows
}

func buildHTTPLogSection(label string, rows []httpRow, limit int) Section {
	var count, sum, s4xx, s5xx float64
	for _, r := range rows {
		count += r.Count
		sum += r.Sum
		s4xx += r.S4xx
		s5xx += r.S5xx
	}

	s := Section{Title: "httplog", Label: label, Status: "ok"}
	s.Totals = Fields{
		{"requests", count},
		{"total_time_s", round(sum, 3)},
		{"endpoints", len(rows)},
		{"error_rate_4xx_5xx", round(ratio(s4xx+s5xx, count), 4)},
		{"rate_5xx", round(ratio(s5xx, count), 4)},
	}

	var tableRows []map[string]any
	for _, r := range rows {
		row := map[string]any{
			"count":  r.Count,
			"method": r.Method,
			"uri":    r.URI,
			"sum":    r.Sum,
			"avg":    r.Avg,
			"max":    r.Max,
			"share":  round(ratio(r.Sum, sum), 4),
		}
		if r.hasStatus {
			row["2xx"], row["4xx"], row["5xx"] = r.S2xx, r.S4xx, r.S5xx
		}
		if r.hasP99 {
			row["p99"] = r.P99
		}
		tableRows = append(tableRows, row)
	}
	tableRows, om := limitRows(tableRows, "sum", limit, sum)

	s.Tables = append(s.Tables, &Table{
		Title: "Endpoints by total response time",
		Columns: []Column{
			{Key: "count", Header: "count", Right: true, Format: fmtInt},
			{Key: "2xx", Header: "2xx", Right: true, Format: fmtInt},
			{Key: "4xx", Header: "4xx", Right: true, Format: fmtInt},
			{Key: "5xx", Header: "5xx", Right: true, Format: fmtInt},
			{Key: "method", Header: "method"},
			{Key: "uri", Header: "uri"},
			{Key: "sum", Header: "sum", Right: true, Format: fmtSec},
			{Key: "avg", Header: "avg", Right: true, Format: fmtSec},
			{Key: "p99", Header: "p99", Right: true, Format: fmtSec},
			{Key: "max", Header: "max", Right: true, Format: fmtSec},
			{Key: "share", Header: "share", Right: true, Format: fmtPct},
		},
		Rows:    tableRows,
		Omitted: om,
	})
	return s
}

func httpLogMetrics(rows []httpRow) metricSet {
	m := metricSet{Items: map[string]metricItem{}}
	for _, r := range rows {
		m.Total += r.Sum
		m.Items[r.Method+" "+r.URI] = metricItem{
			Desc:     map[string]any{"method": r.Method, "uri": r.URI},
			Value:    r.Sum,
			Count:    r.Count,
			HasCount: true,
		}
	}
	return m
}

// slowlog: slp TSV aggregated by normalized query.

type slowRow struct {
	Query                string
	Count, Sum, Avg, Max float64
	ExaminedAvg, SentAvg float64
}

// Queries that examine far more rows than they return are the classic
// sign of a missing index.
func scanSuspect(r slowRow) bool {
	sent := r.SentAvg
	if sent < 1 {
		sent = 1
	}
	return r.ExaminedAvg >= 100 && r.ExaminedAvg >= 10*sent
}

func parseSlowLog(raw string) []slowRow {
	t := parseTSV(raw)
	var rows []slowRow
	for _, r := range t.rows {
		rows = append(rows, slowRow{
			Query:       strings.TrimSpace(t.str(r, "query")),
			Count:       t.num(r, "count"),
			Sum:         t.num(r, "sum(querytime)"),
			Avg:         t.num(r, "avg(querytime)"),
			Max:         t.num(r, "max(querytime)"),
			ExaminedAvg: t.num(r, "avg(rowsexamined)"),
			SentAvg:     t.num(r, "avg(rowssent)"),
		})
	}
	return rows
}

func buildSlowLogSection(label string, rows []slowRow, limit, queryLen int) Section {
	var count, sum float64
	suspects := 0
	for _, r := range rows {
		count += r.Count
		sum += r.Sum
		if scanSuspect(r) {
			suspects++
		}
	}

	s := Section{Title: "slowlog", Label: label, Status: "ok"}
	s.Totals = Fields{
		{"queries", count},
		{"total_time_s", round(sum, 3)},
		{"distinct_queries", len(rows)},
		{"scan_suspects", suspects},
	}

	var tableRows []map[string]any
	for _, r := range rows {
		flag := ""
		if scanSuspect(r) {
			flag = "scan?"
		}
		tableRows = append(tableRows, map[string]any{
			"count":         r.Count,
			"sum":           r.Sum,
			"avg":           r.Avg,
			"max":           r.Max,
			"rows_examined": round(r.ExaminedAvg, 1),
			"rows_sent":     round(r.SentAvg, 1),
			"share":         round(ratio(r.Sum, sum), 4),
			"flag":          flag,
			"query":         truncate(r.Query, queryLen),
		})
	}
	tableRows, om := limitRows(tableRows, "sum", limit, sum)

	s.Tables = append(s.Tables, &Table{
		Title: "Queries by total time",
		Note:  `rows_examined / rows_sent are per-query averages. "scan?" marks queries that examine >=100 rows and >=10x the rows they return (likely missing index).`,
		Columns: []Column{
			{Key: "count", Header: "count", Right: true, Format: fmtInt},
			{Key: "sum", Header: "sum", Right: true, Format: fmtSec},
			{Key: "avg", Header: "avg", Right: true, Format: fmtSec},
			{Key: "max", Header: "max", Right: true, Format: fmtSec},
			{Key: "rows_examined", Header: "rows_examined", Right: true, Format: fmtNum},
			{Key: "rows_sent", Header: "rows_sent", Right: true, Format: fmtNum},
			{Key: "share", Header: "share", Right: true, Format: fmtPct},
			{Key: "flag", Header: "flag"},
			{Key: "query", Header: "query"},
		},
		Rows:    tableRows,
		Omitted: om,
	})
	return s
}

func slowLogMetrics(rows []slowRow, queryLen int) metricSet {
	m := metricSet{Items: map[string]metricItem{}}
	for _, r := range rows {
		m.Total += r.Sum
		m.Items[r.Query] = metricItem{
			Desc:     map[string]any{"query": truncate(r.Query, queryLen)},
			Value:    r.Sum,
			Count:    r.Count,
			HasCount: true,
		}
	}
	return m
}
