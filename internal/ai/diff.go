package ai

import (
	"fmt"
	"math"
)

type (
	// metricSet is one side of a diff: items keyed by a stable identity
	// (function name, method+URI, normalized query).
	metricSet struct {
		Total float64
		Items map[string]metricItem
	}
	metricItem struct {
		Desc     map[string]any // identifying columns shown in tables
		Value    float64        // primary metric (time)
		Count    float64
		HasCount bool
	}
)

// diffTables compares a (before) with b (after): items present on both
// sides sorted by absolute change, then items only in b (new) and only in a
// (gone).
func diffTables(title string, desc []Column, a, b metricSet, timeFmt func(any) string, limit int) []*Table {
	var changed, added, gone []map[string]any
	hasCount := false

	for key, ia := range a.Items {
		ib, ok := b.Items[key]
		if !ok {
			row := copyDesc(ia.Desc)
			row["before"] = round(ia.Value, 4)
			row["share_before"] = round(ratio(ia.Value, a.Total), 4)
			if ia.HasCount {
				hasCount = true
				row["count_before"] = ia.Count
			}
			gone = append(gone, row)
			continue
		}
		row := copyDesc(ib.Desc)
		row["before"] = round(ia.Value, 4)
		row["after"] = round(ib.Value, 4)
		row["delta"] = round(ib.Value-ia.Value, 4)
		if c := change(ia.Value, ib.Value); !math.IsInf(c, 0) {
			row["change"] = round(c, 4)
		}
		row["share_before"] = round(ratio(ia.Value, a.Total), 4)
		row["share_after"] = round(ratio(ib.Value, b.Total), 4)
		if ia.HasCount {
			hasCount = true
			row["count_before"] = ia.Count
			row["count_after"] = ib.Count
		}
		changed = append(changed, row)
	}
	for key, ib := range b.Items {
		if _, ok := a.Items[key]; ok {
			continue
		}
		row := copyDesc(ib.Desc)
		row["after"] = round(ib.Value, 4)
		row["share_after"] = round(ratio(ib.Value, b.Total), 4)
		if ib.HasCount {
			hasCount = true
			row["count_after"] = ib.Count
		}
		added = append(added, row)
	}

	absDelta := func(r map[string]any) float64 { return math.Abs(toFloat(r["delta"])) }
	changed, changedOm := cutRows(changed, absDelta, limit, 0)
	added, addedOm := limitRows(added, "after", limit, b.Total)
	gone, goneOm := limitRows(gone, "before", limit, a.Total)

	countCols := func(cols ...Column) []Column {
		if !hasCount {
			return nil
		}
		return cols
	}
	cb := Column{Key: "count_before", Header: "count_before", Right: true, Format: fmtInt}
	ca := Column{Key: "count_after", Header: "count_after", Right: true, Format: fmtInt}

	changedCols := append(append(append([]Column{}, desc...), countCols(cb, ca)...),
		Column{Key: "before", Header: "before", Right: true, Format: timeFmt},
		Column{Key: "after", Header: "after", Right: true, Format: timeFmt},
		Column{Key: "delta", Header: "delta", Right: true, Format: deltaFmt(timeFmt)},
		Column{Key: "change", Header: "change", Right: true, Format: fmtDeltaPct},
		Column{Key: "share_before", Header: "share_before", Right: true, Format: fmtPct},
		Column{Key: "share_after", Header: "share_after", Right: true, Format: fmtPct},
	)
	addedCols := append(append(append([]Column{}, desc...), countCols(ca)...),
		Column{Key: "after", Header: "after", Right: true, Format: timeFmt},
		Column{Key: "share_after", Header: "share_after", Right: true, Format: fmtPct},
	)
	goneCols := append(append(append([]Column{}, desc...), countCols(cb)...),
		Column{Key: "before", Header: "before", Right: true, Format: timeFmt},
		Column{Key: "share_before", Header: "share_before", Right: true, Format: fmtPct},
	)

	return []*Table{
		{Title: title + ": changed (largest absolute change first)", Columns: changedCols, Rows: changed, Omitted: changedOm},
		{Title: title + ": new (only after)", Columns: addedCols, Rows: added, Omitted: addedOm},
		{Title: title + ": gone (only before)", Columns: goneCols, Rows: gone, Omitted: goneOm},
	}
}

func copyDesc(d map[string]any) map[string]any {
	m := make(map[string]any, len(d)+8)
	for k, v := range d {
		m[k] = v
	}
	return m
}

func deltaFmt(timeFmt func(any) string) func(any) string {
	return func(v any) string {
		s := timeFmt(v)
		if toFloat(v) >= 0 {
			return "+" + s
		}
		return s
	}
}

// beforeAfter is a totals value in a diff; it prints compactly in Markdown
// and stays structured in JSON.
type beforeAfter struct {
	Before any     `json:"before"`
	After  any     `json:"after"`
	Delta  float64 `json:"delta"`
}

func (v beforeAfter) String() string {
	return fmt.Sprintf("%s -> %s (%+g)", formatAny(v.Before), formatAny(v.After), v.Delta)
}

// totalsDiff renders before/after/delta for each total that both sides have.
func totalsDiff(a, b Fields) Fields {
	bm := map[string]any{}
	for _, f := range b {
		bm[f.Key] = f.Value
	}
	var out Fields
	for _, f := range a {
		vb, ok := bm[f.Key]
		if !ok {
			continue
		}
		va := toFloat(f.Value)
		out = append(out, Field{f.Key, beforeAfter{Before: f.Value, After: vb, Delta: round(toFloat(vb)-va, 4)}})
	}
	return out
}
