package ai

import (
	"fmt"

	"github.com/hmdyt/pprotein/internal/collect"
	"github.com/labstack/echo/v4"
)

type entryPair struct {
	a, b *collect.Entry
}

func (h *handler) resolvePair(c echo.Context) (*group, *group, error) {
	a, err := h.resolve(c.Param("id"))
	if err != nil {
		return nil, nil, fmt.Errorf("before: %w", err)
	}
	b, err := h.resolve(c.Param("other"))
	if err != nil {
		return nil, nil, fmt.Errorf("after: %w", err)
	}
	return a, b, nil
}

func describeGroup(g *group) string {
	s := "`" + g.ID + "`"
	if g.Commit != nil {
		s += fmt.Sprintf(" commit `%s` %s", shortHash(g.Commit.Hash), g.Commit.Message)
	}
	if g.Score != nil {
		s += fmt.Sprintf(" score %d", *g.Score)
	}
	return s
}

func (h *handler) newDiffDoc(c echo.Context, title string, a, b *group) *Doc {
	base := baseURL(c)
	d := &Doc{
		Title: title,
		Notes: []string{
			"before: " + describeGroup(a),
			"after: " + describeGroup(b),
		},
	}
	if a.Score != nil && b.Score != nil {
		d.Notes = append(d.Notes, fmt.Sprintf("score: %d -> %d (%+d)", *a.Score, *b.Score, *b.Score-*a.Score))
	}
	for _, t := range viewTypes {
		d.Links = append(d.Links, Link{"diff_" + t, fmt.Sprintf("%s/ai/groups/%s/diff/%s/%s", base, a.ID, b.ID, t)})
	}
	d.Links = append(d.Links,
		Link{"before_views", fmt.Sprintf("%s/ai/groups/%s/{pprof,httplog,slowlog}", base, a.ID)},
		Link{"after_views", fmt.Sprintf("%s/ai/groups/%s/{pprof,httplog,slowlog}", base, b.ID)},
	)
	return d
}

// pairEntries matches snapshots of the two groups by label, or by the
// explicit label_a/label_b query parameters. Labels without a partner are
// returned as sections explaining why they were not compared.
func pairEntries(c echo.Context, typ string, a, b *group) ([]entryPair, []Section) {
	ea, eb := a.Entries[typ], b.Entries[typ]
	if len(ea) == 0 || len(eb) == 0 {
		return nil, []Section{{Title: typ, Status: "missing", Message: fmt.Sprintf("%s snapshots: before=%d, after=%d; nothing to compare", typ, len(ea), len(eb))}}
	}

	find := func(es []*collect.Entry, label string) *collect.Entry {
		for _, e := range es {
			if e.Snapshot.Label == label {
				return e
			}
		}
		return nil
	}

	la, lb := c.QueryParam("label_a"), c.QueryParam("label_b")
	if la != "" || lb != "" {
		x, y := find(ea, la), find(eb, lb)
		if x == nil || y == nil {
			return nil, []Section{{Title: typ, Status: "missing", Message: fmt.Sprintf("label_a=%q found=%v, label_b=%q found=%v", la, x != nil, lb, y != nil)}}
		}
		return []entryPair{{x, y}}, nil
	}

	var pairs []entryPair
	var notes []Section
	for _, x := range ea {
		if y := find(eb, x.Snapshot.Label); y != nil {
			pairs = append(pairs, entryPair{x, y})
		} else {
			notes = append(notes, Section{Title: typ, Label: x.Snapshot.Label, Status: "not_compared", Message: "label exists only in before; use ?label_a=&label_b= to pair it"})
		}
	}
	for _, y := range eb {
		if find(ea, y.Snapshot.Label) == nil {
			notes = append(notes, Section{Title: typ, Label: y.Snapshot.Label, Status: "not_compared", Message: "label exists only in after; use ?label_a=&label_b= to pair it"})
		}
	}
	return pairs, notes
}

func pairLabel(p entryPair) string {
	if p.a.Snapshot.Label == p.b.Snapshot.Label {
		return p.a.Snapshot.Label
	}
	return p.a.Snapshot.Label + " -> " + p.b.Snapshot.Label
}

// diffSection compares one pair. limit bounds every table; summary keeps
// only the "changed" tables.
func (h *handler) diffSection(c echo.Context, typ string, p entryPair, limit int, summary bool) Section {
	label := pairLabel(p)
	for _, e := range []*collect.Entry{p.a, p.b} {
		if s, ok := statusSection(typ, e); !ok {
			s.Label = label
			return s
		}
	}
	fail := func(err error) Section {
		return Section{Title: typ, Label: label, Status: "fail", Message: err.Error()}
	}

	var tables []*Table
	var ta, tb Fields
	switch typ {
	case "pprof":
		da, err := h.loadPProf(p.a)
		if err != nil {
			return fail(err)
		}
		db, err := h.loadPProf(p.b)
		if err != nil {
			return fail(err)
		}
		opts := pprofOptions{Limit: 1, Cores: intParam(c, "cores", defaultCores)}
		ta, tb = buildPProfSection("", da, opts).Totals, buildPProfSection("", db, opts).Totals
		flatA, appA := pprofMetrics(da)
		flatB, appB := pprofMetrics(db)
		fn := []Column{{Key: "function", Header: "function"}}
		tables = append(tables, diffTables("flat (all functions)", fn, flatA, flatB, da.timeFmt(), limit)...)
		tables = append(tables, diffTables("cum (app code)", fn, appA, appB, da.timeFmt(), limit)...)
	case "httplog":
		ra, err := h.loadProcessed(h.src.HTTPLog, p.a)
		if err != nil {
			return fail(err)
		}
		rb, err := h.loadProcessed(h.src.HTTPLog, p.b)
		if err != nil {
			return fail(err)
		}
		ha, hb := parseHTTPLog(ra), parseHTTPLog(rb)
		ta, tb = buildHTTPLogSection("", ha, 1).Totals, buildHTTPLogSection("", hb, 1).Totals
		tables = diffTables("endpoints by total time", []Column{{Key: "method", Header: "method"}, {Key: "uri", Header: "uri"}},
			httpLogMetrics(ha), httpLogMetrics(hb), fmtSec, limit)
	case "slowlog":
		ra, err := h.loadProcessed(h.src.SlowLog, p.a)
		if err != nil {
			return fail(err)
		}
		rb, err := h.loadProcessed(h.src.SlowLog, p.b)
		if err != nil {
			return fail(err)
		}
		sa, sb := parseSlowLog(ra), parseSlowLog(rb)
		ql := queryLen(c)
		ta, tb = buildSlowLogSection("", sa, 1, ql).Totals, buildSlowLogSection("", sb, 1, ql).Totals
		tables = diffTables("queries by total time", []Column{{Key: "query", Header: "query"}},
			slowLogMetrics(sa, ql), slowLogMetrics(sb, ql), fmtSec, limit)
	default:
		return fail(fmt.Errorf("unknown type %q", typ))
	}

	if summary {
		var changed []*Table
		for _, t := range tables {
			if len(t.Rows) > 0 && len(changed) < 2 && isChangedTable(t) {
				changed = append(changed, t)
			}
		}
		tables = changed
	}
	return Section{Title: typ, Label: label, Status: "ok", Totals: totalsDiff(ta, tb), Tables: tables}
}

func isChangedTable(t *Table) bool {
	for _, c := range t.Columns {
		if c.Key == "delta" {
			return true
		}
	}
	return false
}

func (h *handler) getDiff(c echo.Context) error {
	typ := c.Param("type")
	if h.collector(typ) == nil {
		return errorDoc(c, "unknown view", fmt.Errorf("unknown view %q; use pprof, httplog or slowlog", typ))
	}
	a, b, err := h.resolvePair(c)
	if err != nil {
		return errorDoc(c, "diff", err)
	}

	d := h.newDiffDoc(c, fmt.Sprintf("%s diff %s -> %s", typ, a.ID, b.ID), a, b)
	d.Notes = append(d.Notes, "Rows are matched by "+matchKey(typ)+". delta = after - before; change = delta / before. Shares are of each side's own total.")

	pairs, notes := pairEntries(c, typ, a, b)
	limit := intParam(c, "limit", defaultLimit)
	for _, p := range pairs {
		d.Sections = append(d.Sections, h.diffSection(c, typ, p, limit, false))
	}
	d.Sections = append(d.Sections, notes...)
	return respond(c, d)
}

func (h *handler) getDiffSummary(c echo.Context) error {
	a, b, err := h.resolvePair(c)
	if err != nil {
		return errorDoc(c, "diff", err)
	}

	d := h.newDiffDoc(c, fmt.Sprintf("diff summary %s -> %s", a.ID, b.ID), a, b)
	d.Notes = append(d.Notes, fmt.Sprintf("Totals per view plus the %d largest changes. Follow the diff_* links for full tables.", summaryLimit))

	for _, typ := range viewTypes {
		pairs, notes := pairEntries(c, typ, a, b)
		for _, p := range pairs {
			d.Sections = append(d.Sections, h.diffSection(c, typ, p, summaryLimit, true))
		}
		d.Sections = append(d.Sections, notes...)
	}
	return respond(c, d)
}

func matchKey(typ string) string {
	switch typ {
	case "pprof":
		return "function name"
	case "httplog":
		return "method + URI"
	case "slowlog":
		return "normalized query"
	}
	return "key"
}
