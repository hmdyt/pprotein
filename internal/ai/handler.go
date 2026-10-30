package ai

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-json"
	"github.com/hmdyt/pprotein/internal/collect"
	"github.com/labstack/echo/v4"
)

const (
	defaultLimit      = 30
	defaultGroupLimit = 20
	defaultQueryLen   = 300
	defaultCores      = 2
	summaryLimit      = 5
)

var (
	viewTypes = []string{"pprof", "httplog", "slowlog"}
	scoreRe   = regexp.MustCompile(`(?m)^\s*score\s*=\s*(-?\d+)`)
)

type (
	// Sources are the collectors whose snapshots the AI views read.
	Sources struct {
		PProf   *collect.Collector
		HTTPLog *collect.Collector
		SlowLog *collect.Collector
		Memo    *collect.Collector
	}

	handler struct {
		src Sources
	}

	group struct {
		ID      string
		Time    time.Time
		Entries map[string][]*collect.Entry
		Commit  *Commit
		Score   *int64
		Memos   []string
	}
)

func Register(g *echo.Group, src Sources) {
	h := &handler{src: src}
	g.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Cache-Control", "no-store")
			return next(c)
		}
	})
	g.GET("/groups", h.getGroups)
	g.GET("/groups/:id/:type", h.getView)
	// The first parameter is named "id" on every route so that echo's
	// shared param node resolves the same name.
	g.GET("/groups/:id/diff/:other", h.getDiffSummary)
	g.GET("/groups/:id/diff/:other/:type", h.getDiff)
}

func (h *handler) collector(typ string) *collect.Collector {
	switch typ {
	case "pprof":
		return h.src.PProf
	case "httplog":
		return h.src.HTTPLog
	case "slowlog":
		return h.src.SlowLog
	}
	return nil
}

// groups returns all groups that have at least one profile/log snapshot,
// newest first.
func (h *handler) groups() []*group {
	byID := map[string]*group{}
	for _, typ := range viewTypes {
		for _, e := range h.collector(typ).List() {
			id := e.Snapshot.GroupId
			if id == "" {
				continue
			}
			g, ok := byID[id]
			if !ok {
				g = &group{ID: id, Time: e.Snapshot.Datetime, Entries: map[string][]*collect.Entry{}}
				byID[id] = g
			}
			if e.Snapshot.Datetime.Before(g.Time) {
				g.Time = e.Snapshot.Datetime
			}
			g.Entries[typ] = append(g.Entries[typ], e)
			if g.Commit == nil && e.Snapshot.Repository != nil && e.Snapshot.Repository.Hash != "" {
				g.Commit = &Commit{Hash: e.Snapshot.Repository.Hash, Message: oneLine(e.Snapshot.Repository.Message)}
			}
		}
	}

	memos := h.src.Memo.List()
	sort.Slice(memos, func(i, j int) bool { return memos[i].Snapshot.Datetime.Before(memos[j].Snapshot.Datetime) })
	for _, m := range memos {
		g, ok := byID[m.Snapshot.GroupId]
		if !ok {
			continue
		}
		text := h.memoText(m)
		if text == "" {
			continue
		}
		g.Memos = append(g.Memos, text)
		if sm := scoreRe.FindStringSubmatch(text); sm != nil {
			if v, err := strconv.ParseInt(sm[1], 10, 64); err == nil {
				g.Score = &v // the latest score memo wins
			}
		}
	}

	var list []*group
	for _, g := range byID {
		for _, es := range g.Entries {
			sort.Slice(es, func(i, j int) bool { return es[i].Snapshot.Label < es[j].Snapshot.Label })
		}
		list = append(list, g)
	}
	sort.Slice(list, func(i, j int) bool {
		if !list[i].Time.Equal(list[j].Time) {
			return list[i].Time.After(list[j].Time)
		}
		return list[i].ID > list[j].ID
	})
	return list
}

func (h *handler) memoText(e *collect.Entry) string {
	r, err := h.src.Memo.Get(e.Snapshot.ID)
	if err != nil {
		return ""
	}
	defer r.Close()
	buf, err := io.ReadAll(r)
	if err != nil {
		return ""
	}
	var v struct{ Text string }
	if err := json.Unmarshal(buf, &v); err != nil {
		return ""
	}
	return strings.TrimSpace(v.Text)
}

// resolve finds a group by ID or by the aliases "latest" and "prev".
func (h *handler) resolve(id string) (*group, error) {
	list := h.groups()
	switch id {
	case "latest":
		if len(list) > 0 {
			return list[0], nil
		}
		return nil, fmt.Errorf("there are no groups yet")
	case "prev":
		if len(list) > 1 {
			return list[1], nil
		}
		return nil, fmt.Errorf("there is no group before the latest one")
	}
	for _, g := range list {
		if g.ID == id {
			return g, nil
		}
	}
	return nil, fmt.Errorf("no group %q (use /ai/groups to list them)", id)
}

func baseURL(c echo.Context) string {
	return c.Scheme() + "://" + c.Request().Host
}

func intParam(c echo.Context, name string, def int) int {
	if v, err := strconv.Atoi(c.QueryParam(name)); err == nil && v > 0 {
		return v
	}
	return def
}

func queryLen(c echo.Context) int {
	if c.QueryParam("full") == "1" {
		return 0
	}
	return defaultQueryLen
}

func respond(c echo.Context, d *Doc) error {
	if c.QueryParam("format") == "json" {
		b, err := d.JSON()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.JSONBlob(http.StatusOK, b)
	}
	return c.Blob(http.StatusOK, "text/markdown; charset=utf-8", []byte(d.Markdown()))
}

func errorDoc(c echo.Context, title string, err error) error {
	return respond(c, &Doc{
		Title: title,
		Notes: []string{"error: " + err.Error()},
		Links: []Link{{"groups", baseURL(c) + "/ai/groups"}},
	})
}

func groupLinks(base, id string) []Link {
	var links []Link
	for _, t := range viewTypes {
		links = append(links, Link{t, fmt.Sprintf("%s/ai/groups/%s/%s", base, id, t)})
	}
	return links
}

func (h *handler) getGroups(c echo.Context) error {
	list := h.groups()
	limit := intParam(c, "limit", defaultGroupLimit)
	base := baseURL(c)

	d := &Doc{
		Title: "pprotein groups",
		Notes: []string{
			"A group is one benchmark run: pprof, httplog and slowlog collected together.",
			"Views: /ai/groups/{id}/{pprof|httplog|slowlog}, diffs: /ai/groups/{a}/diff/{b}[/{type}]. {id} also accepts latest and prev.",
			"Add ?format=json for JSON.",
		},
		Links: []Link{
			{"latest_vs_prev", base + "/ai/groups/prev/diff/latest"},
		},
	}

	rows := []map[string]any{}
	for _, g := range list {
		row := map[string]any{
			"group_id": g.ID,
			"time":     g.Time.Format(time.RFC3339),
			"memo":     strings.Join(g.Memos, " / "),
			"views":    fmt.Sprintf("/ai/groups/%s/{pprof,httplog,slowlog}", g.ID),
		}
		if g.Commit != nil {
			row["commit"] = shortHash(g.Commit.Hash) + " " + g.Commit.Message
		}
		if g.Score != nil {
			row["score"] = *g.Score
		}
		for _, t := range viewTypes {
			var st []string
			for _, e := range g.Entries[t] {
				st = append(st, e.Snapshot.Label+":"+string(e.Status))
			}
			row[t] = strings.Join(st, ", ")
		}
		rows = append(rows, row)
	}

	t := &Table{
		Title: "Groups (newest first)",
		Columns: []Column{
			{Key: "group_id", Header: "group_id"},
			{Key: "time", Header: "time"},
			{Key: "commit", Header: "commit"},
			{Key: "score", Header: "score", Right: true},
			{Key: "pprof", Header: "pprof"},
			{Key: "httplog", Header: "httplog"},
			{Key: "slowlog", Header: "slowlog"},
			{Key: "memo", Header: "memo"},
			{Key: "views", Header: "views"},
		},
		Rows: rows,
	}
	if len(rows) > limit {
		t.Rows = rows[:limit]
		t.Omitted = &Omitted{Rows: len(rows) - limit}
	}
	d.Sections = []Section{{Title: "groups", Tables: []*Table{t}}}
	return respond(c, d)
}

func (h *handler) getView(c echo.Context) error {
	typ := c.Param("type")
	if h.collector(typ) == nil {
		return errorDoc(c, "unknown view", fmt.Errorf("unknown view %q; use pprof, httplog or slowlog", typ))
	}
	g, err := h.resolve(c.Param("id"))
	if err != nil {
		return errorDoc(c, typ, err)
	}

	d := h.newGroupDoc(c, typ+" of "+g.ID, g)
	d.Notes = append(d.Notes, viewNote(typ))
	d.Sections = h.sections(c, g, typ, c.QueryParam("label"))
	return respond(c, d)
}

func (h *handler) newGroupDoc(c echo.Context, title string, g *group) *Doc {
	base := baseURL(c)
	return &Doc{
		Title:   title,
		GroupID: g.ID,
		Commit:  g.Commit,
		Score:   g.Score,
		Links:   groupLinks(base, g.ID),
	}
}

func viewNote(typ string) string {
	switch typ {
	case "pprof":
		return "CPU profile of the app. flat = time in the function itself; cum = including callees. App code = package main. Add ?focus=<substring> for callers/callees, ?cores=N for utilization."
	case "httplog":
		return "nginx access log aggregated by alp (method + URI), sorted by total response time."
	case "slowlog":
		return "MySQL slow query log aggregated by slp (normalized queries), sorted by total time. Queries are cut to 300 chars; add ?full=1 for full text."
	}
	return ""
}

// sections builds one section per label of typ in g. Entries that are not
// ready become sections carrying their status instead of data.
func (h *handler) sections(c echo.Context, g *group, typ, label string) []Section {
	entries := g.Entries[typ]
	if len(entries) == 0 {
		return []Section{{Title: typ, Status: "missing", Message: fmt.Sprintf("this group has no %s snapshot", typ)}}
	}

	var out []Section
	for _, e := range entries {
		if label != "" && e.Snapshot.Label != label {
			continue
		}
		s, ok := statusSection(typ, e)
		if !ok {
			out = append(out, s)
			continue
		}
		out = append(out, h.buildSection(c, typ, e))
	}
	if len(out) == 0 {
		return []Section{{Title: typ, Status: "missing", Message: fmt.Sprintf("no %s snapshot with label %q", typ, label)}}
	}
	return out
}

func statusSection(typ string, e *collect.Entry) (Section, bool) {
	s := Section{Title: typ, Label: e.Snapshot.Label, Status: string(e.Status)}
	switch e.Status {
	case collect.StatusOk:
		return s, true
	case collect.StatusPending:
		s.Message = e.Message + "; still collecting or processing, retry in about 30 seconds"
	default:
		s.Message = e.Message
	}
	return s, false
}

func (h *handler) buildSection(c echo.Context, typ string, e *collect.Entry) Section {
	label := e.Snapshot.Label
	fail := func(err error) Section {
		return Section{Title: typ, Label: label, Status: "fail", Message: err.Error()}
	}
	switch typ {
	case "pprof":
		d, err := h.loadPProf(e)
		if err != nil {
			return fail(err)
		}
		return buildPProfSection(label, d, pprofOptions{
			Limit: intParam(c, "limit", defaultLimit),
			Cores: intParam(c, "cores", defaultCores),
			Focus: c.QueryParam("focus"),
		})
	case "httplog":
		raw, err := h.loadProcessed(h.src.HTTPLog, e)
		if err != nil {
			return fail(err)
		}
		return buildHTTPLogSection(label, parseHTTPLog(raw), intParam(c, "limit", defaultLimit))
	case "slowlog":
		raw, err := h.loadProcessed(h.src.SlowLog, e)
		if err != nil {
			return fail(err)
		}
		return buildSlowLogSection(label, parseSlowLog(raw), intParam(c, "limit", defaultLimit), queryLen(c))
	}
	return fail(fmt.Errorf("unknown type %q", typ))
}

func (h *handler) loadPProf(e *collect.Entry) (*pprofData, error) {
	path, err := e.Snapshot.BodyPath()
	if err != nil {
		return nil, fmt.Errorf("failed to find profile: %w", err)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open profile: %w", err)
	}
	defer f.Close()
	return parsePProf(f)
}

func (h *handler) loadProcessed(col *collect.Collector, e *collect.Entry) (string, error) {
	r, err := col.Get(e.Snapshot.ID)
	if err != nil {
		return "", fmt.Errorf("failed to process snapshot: %w", err)
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("failed to read processed snapshot: %w", err)
	}
	return string(b), nil
}
