package ai

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/goccy/go-json"
)

type (
	// Doc is one AI-facing response. It renders to Markdown by default and
	// to JSON when requested; both carry the same information.
	Doc struct {
		Title    string    `json:"title"`
		GroupID  string    `json:"group_id,omitempty"`
		Commit   *Commit   `json:"commit,omitempty"`
		Score    *int64    `json:"score,omitempty"`
		Notes    []string  `json:"notes,omitempty"`
		Links    []Link    `json:"links,omitempty"`
		Sections []Section `json:"sections"`
	}

	Commit struct {
		Hash    string `json:"hash"`
		Message string `json:"message"`
	}

	Link struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}

	Section struct {
		Title   string   `json:"title"`
		Label   string   `json:"label,omitempty"`
		Status  string   `json:"status,omitempty"`
		Message string   `json:"message,omitempty"`
		Totals  Fields   `json:"totals,omitempty"`
		Tables  []*Table `json:"tables,omitempty"`
	}

	// Field is one entry of an ordered key/value list.
	Field struct {
		Key   string
		Value any
	}
	// Fields keeps insertion order in both Markdown and JSON.
	Fields []Field

	Column struct {
		Key    string
		Header string
		Right  bool
		Format func(any) string
	}

	Table struct {
		Title   string           `json:"title"`
		Note    string           `json:"note,omitempty"`
		Columns []Column         `json:"-"`
		Rows    []map[string]any `json:"rows"`
		Omitted *Omitted         `json:"omitted,omitempty"`
	}

	// Omitted reports rows cut by ?limit=. Share is the fraction of the
	// table's total they carried, when that total is meaningful.
	Omitted struct {
		Rows  int      `json:"rows"`
		Share *float64 `json:"share,omitempty"`
	}
)

func (f Fields) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, kv := range f {
		if i > 0 {
			buf.WriteByte(',')
		}
		k, err := json.Marshal(kv.Key)
		if err != nil {
			return nil, err
		}
		v, err := json.Marshal(kv.Value)
		if err != nil {
			return nil, err
		}
		buf.Write(k)
		buf.WriteByte(':')
		buf.Write(v)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func (d *Doc) JSON() ([]byte, error) {
	return json.MarshalIndent(d, "", "  ")
}

func (d *Doc) Markdown() string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", d.Title)
	if d.GroupID != "" {
		fmt.Fprintf(&b, "- group_id: `%s`\n", d.GroupID)
	}
	if d.Commit != nil {
		fmt.Fprintf(&b, "- commit: `%s` %s\n", shortHash(d.Commit.Hash), oneLine(d.Commit.Message))
	}
	if d.Score != nil {
		fmt.Fprintf(&b, "- score: %d\n", *d.Score)
	}
	for _, n := range d.Notes {
		fmt.Fprintf(&b, "- %s\n", n)
	}
	if len(d.Links) > 0 {
		b.WriteString("- links:\n")
		for _, l := range d.Links {
			fmt.Fprintf(&b, "  - %s: %s\n", l.Name, l.URL)
		}
	}

	for _, s := range d.Sections {
		b.WriteString("\n")
		title := s.Title
		if s.Label != "" {
			title += " [" + s.Label + "]"
		}
		fmt.Fprintf(&b, "## %s\n\n", title)
		if s.Status != "" && s.Status != "ok" {
			fmt.Fprintf(&b, "status: **%s**", s.Status)
			if s.Message != "" {
				fmt.Fprintf(&b, " (%s)", s.Message)
			}
			b.WriteString("\n\n")
		} else if s.Message != "" {
			fmt.Fprintf(&b, "%s\n\n", s.Message)
		}
		for _, kv := range s.Totals {
			fmt.Fprintf(&b, "- %s: %s\n", kv.Key, formatAny(kv.Value))
		}
		if len(s.Totals) > 0 {
			b.WriteString("\n")
		}
		for _, t := range s.Tables {
			t.writeMarkdown(&b)
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func (t *Table) writeMarkdown(b *strings.Builder) {
	fmt.Fprintf(b, "### %s\n\n", t.Title)
	if t.Note != "" {
		fmt.Fprintf(b, "%s\n\n", t.Note)
	}
	if len(t.Rows) == 0 {
		b.WriteString("(no rows)\n\n")
		return
	}

	b.WriteString("|")
	for _, c := range t.Columns {
		fmt.Fprintf(b, " %s |", c.Header)
	}
	b.WriteString("\n|")
	for _, c := range t.Columns {
		if c.Right {
			b.WriteString("---:|")
		} else {
			b.WriteString("---|")
		}
	}
	b.WriteString("\n")
	for _, row := range t.Rows {
		b.WriteString("|")
		for _, c := range t.Columns {
			v, ok := row[c.Key]
			s := ""
			if ok && v != nil {
				if c.Format != nil {
					s = c.Format(v)
				} else {
					s = formatAny(v)
				}
			}
			fmt.Fprintf(b, " %s |", escapeCell(s))
		}
		b.WriteString("\n")
	}
	if t.Omitted != nil && t.Omitted.Rows > 0 {
		if t.Omitted.Share != nil {
			fmt.Fprintf(b, "\n(%d more rows omitted, %.1f%% of total; use ?limit= to see more)\n", t.Omitted.Rows, *t.Omitted.Share*100)
		} else {
			fmt.Fprintf(b, "\n(%d more rows omitted; use ?limit= to see more)\n", t.Omitted.Rows)
		}
	}
	b.WriteString("\n")
}

func escapeCell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	return strings.ReplaceAll(s, "\n", " ")
}
