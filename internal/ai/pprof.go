package ai

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/google/pprof/profile"
)

type (
	pprofFunc struct {
		Name     string
		Location string
		Flat     float64
		Cum      float64
	}

	pprofData struct {
		Unit     string // "s" for CPU time, "samples" otherwise
		Total    float64
		Duration float64 // seconds
		Funcs    map[string]*pprofFunc
		stacks   []pprofStack
	}

	// pprofStack is one sample's call stack, leaf first, with inlined
	// frames expanded.
	pprofStack struct {
		frames []string
		value  float64
	}

	pprofOptions struct {
		Limit int
		Cores int
		Focus string
	}
)

func appFunc(name string) bool {
	return strings.HasPrefix(name, "main.")
}

func parsePProf(r io.Reader) (*pprofData, error) {
	p, err := profile.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse profile: %w", err)
	}
	if len(p.SampleType) == 0 {
		return nil, fmt.Errorf("profile has no sample types")
	}

	idx := len(p.SampleType) - 1
	for i, st := range p.SampleType {
		if st.Type == "cpu" {
			idx = i
		}
	}
	scale, unit := 1.0, "samples"
	if p.SampleType[idx].Unit == "nanoseconds" {
		scale, unit = 1e-9, "s"
	}

	d := &pprofData{
		Unit:     unit,
		Duration: float64(p.DurationNanos) / 1e9,
		Funcs:    map[string]*pprofFunc{},
	}
	for _, s := range p.Sample {
		v := float64(s.Value[idx]) * scale
		d.Total += v

		var frames []string
		for _, loc := range s.Location {
			for _, line := range loc.Line {
				if line.Function == nil {
					continue
				}
				name := line.Function.Name
				f, ok := d.Funcs[name]
				if !ok {
					f = &pprofFunc{Name: name, Location: fmt.Sprintf("%s:%d", shortPath(line.Function.Filename), line.Function.StartLine)}
					d.Funcs[name] = f
				}
				frames = append(frames, name)
			}
		}
		if len(frames) == 0 {
			continue
		}
		d.Funcs[frames[0]].Flat += v
		seen := map[string]bool{}
		for _, name := range frames {
			if !seen[name] {
				seen[name] = true
				d.Funcs[name].Cum += v
			}
		}
		d.stacks = append(d.stacks, pprofStack{frames: frames, value: v})
	}
	return d, nil
}

func (d *pprofData) timeFmt() func(any) string {
	if d.Unit == "s" {
		return fmtSec
	}
	return fmtNum
}

func (d *pprofData) funcColumns(withShare bool) []Column {
	cols := []Column{
		{Key: "function", Header: "function"},
		{Key: "location", Header: "location"},
		{Key: "flat", Header: "flat", Right: true, Format: d.timeFmt()},
	}
	if withShare {
		cols = append(cols, Column{Key: "flat_share", Header: "flat%", Right: true, Format: fmtPct})
	}
	cols = append(cols, Column{Key: "cum", Header: "cum", Right: true, Format: d.timeFmt()})
	if withShare {
		cols = append(cols, Column{Key: "cum_share", Header: "cum%", Right: true, Format: fmtPct})
	}
	return cols
}

func (d *pprofData) funcRows(filter func(string) bool) []map[string]any {
	var rows []map[string]any
	for _, f := range d.Funcs {
		if filter != nil && !filter(f.Name) {
			continue
		}
		rows = append(rows, map[string]any{
			"function":   f.Name,
			"location":   f.Location,
			"flat":       round(f.Flat, 4),
			"flat_share": round(ratio(f.Flat, d.Total), 4),
			"cum":        round(f.Cum, 4),
			"cum_share":  round(ratio(f.Cum, d.Total), 4),
		})
	}
	return rows
}

func buildPProfSection(label string, d *pprofData, opts pprofOptions) Section {
	s := Section{Title: "pprof", Label: label, Status: "ok"}

	s.Totals = Fields{
		{"total_" + unitKey(d.Unit), round(d.Total, 3)},
		{"duration_s", round(d.Duration, 1)},
	}
	if d.Unit == "s" && d.Duration > 0 && opts.Cores > 0 {
		s.Totals = append(s.Totals,
			Field{"cores_assumed", opts.Cores},
			Field{"cpu_utilization", round(d.Total/(d.Duration*float64(opts.Cores)), 3)},
		)
	}

	flatRows, flatOm := limitRows(d.funcRows(nil), "flat", opts.Limit, d.Total)
	s.Tables = append(s.Tables, &Table{
		Title:   "Top functions by flat (time in the function itself)",
		Columns: d.funcColumns(true),
		Rows:    flatRows,
		Omitted: flatOm,
	})

	cumRows, cumOm := limitRows(d.funcRows(nil), "cum", opts.Limit, 0)
	s.Tables = append(s.Tables, &Table{
		Title:   "Top functions by cum (including callees)",
		Note:    "cum shares overlap and do not add up to 100%.",
		Columns: d.funcColumns(true),
		Rows:    cumRows,
		Omitted: cumOm,
	})

	appRows, appOm := limitRows(d.funcRows(appFunc), "cum", opts.Limit, 0)
	s.Tables = append(s.Tables, &Table{
		Title:   "App code (package main) by cum",
		Note:    "Where the application's own handlers and helpers spend time, including the DB and libraries they call.",
		Columns: d.funcColumns(true),
		Rows:    appRows,
		Omitted: appOm,
	})

	if opts.Focus != "" {
		s.Tables = append(s.Tables, d.focusTables(opts)...)
	}
	return s
}

// focusTables shows the callers and callees of functions whose name
// contains opts.Focus, weighted by the samples passing through them.
func (d *pprofData) focusTables(opts pprofOptions) []*Table {
	callers := map[string]float64{}
	callees := map[string]float64{}
	matched := map[string]bool{}
	total := 0.0

	for _, st := range d.stacks {
		for i, name := range st.frames {
			if !strings.Contains(name, opts.Focus) {
				continue
			}
			matched[name] = true
			total += st.value
			if i == 0 {
				callees["(self)"] += st.value
			} else {
				callees[st.frames[i-1]] += st.value
			}
			if i+1 < len(st.frames) {
				callers[st.frames[i+1]] += st.value
			} else {
				callers["(root)"] += st.value
			}
			break
		}
	}

	var names []string
	for n := range matched {
		names = append(names, n)
	}
	sort.Strings(names)
	note := fmt.Sprintf("Functions matching %q: %s. Values are time of samples passing through them.", opts.Focus, strings.Join(names, ", "))
	if len(names) == 0 {
		note = fmt.Sprintf("No function matches %q.", opts.Focus)
	}

	toTable := func(title string, m map[string]float64) *Table {
		var rows []map[string]any
		for name, v := range m {
			rows = append(rows, map[string]any{"function": name, "value": round(v, 4), "share": round(ratio(v, total), 4)})
		}
		rows, om := limitRows(rows, "value", opts.Limit, total)
		return &Table{
			Title: title,
			Note:  note,
			Columns: []Column{
				{Key: "function", Header: "function"},
				{Key: "value", Header: "time", Right: true, Format: d.timeFmt()},
				{Key: "share", Header: "share", Right: true, Format: fmtPct},
			},
			Rows:    rows,
			Omitted: om,
		}
	}
	return []*Table{
		toTable("Focus: callers", callers),
		toTable("Focus: callees", callees),
	}
}

func unitKey(unit string) string {
	if unit == "s" {
		return "cpu_s"
	}
	return "samples"
}

// pprofMetrics flattens a profile into comparable metric sets for diffs.
func pprofMetrics(d *pprofData) (flat, appCum metricSet) {
	flat = metricSet{Total: d.Total, Items: map[string]metricItem{}}
	appCum = metricSet{Total: d.Total, Items: map[string]metricItem{}}
	for _, f := range d.Funcs {
		desc := map[string]any{"function": f.Name}
		if f.Flat > 0 {
			flat.Items[f.Name] = metricItem{Desc: desc, Value: f.Flat}
		}
		if appFunc(f.Name) {
			appCum.Items[f.Name] = metricItem{Desc: desc, Value: f.Cum}
		}
	}
	return flat, appCum
}
