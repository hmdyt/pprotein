package ai

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/pprof/profile"
)

var update = flag.Bool("update", false, "rewrite golden files")

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden file (run go test -update): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s mismatch\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func goldenDoc(t *testing.T, name string, d *Doc) {
	t.Helper()
	golden(t, name+".md", []byte(d.Markdown()))
	j, err := d.JSON()
	if err != nil {
		t.Fatal(err)
	}
	golden(t, name+".json", append(j, '\n'))
}

// buildProfile makes a small CPU profile: stacks are root-first lists of
// function names, values in nanoseconds.
func buildProfile(t *testing.T, stacks map[string][]string, values map[string]int64) *pprofData {
	t.Helper()
	p := &profile.Profile{
		SampleType:    []*profile.ValueType{{Type: "samples", Unit: "count"}, {Type: "cpu", Unit: "nanoseconds"}},
		DurationNanos: 10e9,
	}
	funcs := map[string]*profile.Function{}
	locs := map[string]*profile.Location{}
	fn := func(name string) *profile.Location {
		if l, ok := locs[name]; ok {
			return l
		}
		f := &profile.Function{ID: uint64(len(funcs) + 1), Name: name, Filename: "/src/app/" + name + ".go", StartLine: 10}
		funcs[name] = f
		p.Function = append(p.Function, f)
		l := &profile.Location{ID: uint64(len(locs) + 1), Line: []profile.Line{{Function: f, Line: 12}}}
		locs[name] = l
		p.Location = append(p.Location, l)
		return l
	}
	for _, key := range sortedKeys(stacks) {
		var ls []*profile.Location
		frames := stacks[key]
		for i := len(frames) - 1; i >= 0; i-- { // leaf first
			ls = append(ls, fn(frames[i]))
		}
		p.Sample = append(p.Sample, &profile.Sample{Location: ls, Value: []int64{1, values[key]}})
	}

	var buf bytes.Buffer
	if err := p.Write(&buf); err != nil {
		t.Fatal(err)
	}
	d, err := parsePProf(&buf)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func sortedKeys(m map[string][]string) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	for i := range ks {
		for j := i + 1; j < len(ks); j++ {
			if ks[j] < ks[i] {
				ks[i], ks[j] = ks[j], ks[i]
			}
		}
	}
	return ks
}

var (
	stacksBefore = map[string][]string{
		"a": {"net/http.serve", "main.getRides", "database/sql.Query", "syscall.Syscall"},
		"b": {"net/http.serve", "main.getRides", "encoding/json.Marshal"},
		"c": {"net/http.serve", "main.postCoordinate", "database/sql.Exec", "syscall.Syscall"},
		"d": {"runtime.gcBgMarkWorker"},
	}
	valuesBefore = map[string]int64{"a": 4e9, "b": 1e9, "c": 2e9, "d": 1e9}

	stacksAfter = map[string][]string{
		"a": {"net/http.serve", "main.getRides", "database/sql.Query", "syscall.Syscall"},
		"c": {"net/http.serve", "main.postCoordinate", "database/sql.Exec", "syscall.Syscall"},
		"d": {"runtime.gcBgMarkWorker"},
		"e": {"net/http.serve", "main.getRides", "main.cacheLookup"},
	}
	valuesAfter = map[string]int64{"a": 1e9, "c": 2e9, "d": 1e9, "e": 5e8}
)

const httplogBefore = "Count\t1xx\t2xx\t3xx\t4xx\t5xx\tMethod\tUri\tMin\tMax\tSum\tAvg\tP99\n" +
	"100\t0\t98\t0\t2\t0\tGET\t/api/app/notification\t0.010\t1.000\t40.000\t0.400\t0.900\n" +
	"50\t0\t50\t0\t0\t0\tPOST\t/api/chair/coordinate\t0.020\t0.500\t10.000\t0.200\t0.450\n" +
	"5\t0\t4\t0\t0\t1\tGET\t/api/owner/chairs\t1.000\t3.000\t10.000\t2.000\t3.000\n"

const httplogAfter = "Count\t1xx\t2xx\t3xx\t4xx\t5xx\tMethod\tUri\tMin\tMax\tSum\tAvg\tP99\n" +
	"120\t0\t120\t0\t0\t0\tGET\t/api/app/notification\t0.010\t0.300\t12.000\t0.100\t0.250\n" +
	"60\t0\t60\t0\t0\t0\tPOST\t/api/chair/coordinate\t0.020\t0.500\t11.000\t0.183\t0.450\n" +
	"10\t0\t10\t0\t0\t0\tGET\t/api/app/nearby-chairs\t0.100\t0.900\t3.000\t0.300\t0.900\n"

const slowlogSample = "Count\tQuery\tMin(QueryTime)\tMax(QueryTime)\tSum(QueryTime)\tAvg(QueryTime)\tMin(LockTime)\tMax(LockTime)\tSum(LockTime)\tAvg(LockTime)\tMin(RowsSent)\tMax(RowsSent)\tSum(RowsSent)\tAvg(RowsSent)\tMin(RowsExamined)\tMax(RowsExamined)\tSum(RowsExamined)\tAvg(RowsExamined)\n" +
	"5000\tSELECT `status` FROM `ride_statuses` WHERE `ride_id` = 'S' ORDER BY `created_at` DESC LIMIT N\t0.001\t0.100\t150.000\t0.030\t0\t0\t0\t0\t1\t1\t5000\t1.000000\t4000\t5000\t22500000\t4500.000000\n" +
	"80\tSELECT * FROM `chairs` WHERE `owner_id` = 'S'\t0.001\t0.010\t0.400\t0.005\t0\t0\t0\t0\t5\t10\t600\t7.500000\t5\t10\t600\t7.500000\n" +
	"300\tINSERT INTO `chair_locations` (`id`, `chair_id`) VALUES ('S', 'S')\t0.000\t0.020\t1.200\t0.004\t0\t0\t0\t0\t0\t0\t0\t0.000000\t0\t0\t0\t0.000000\n"

func TestPProfView(t *testing.T) {
	d := buildProfile(t, stacksBefore, valuesBefore)
	doc := &Doc{Title: "pprof of g1", GroupID: "g1", Sections: []Section{
		buildPProfSection("app", d, pprofOptions{Limit: 3, Cores: 2, Focus: "getRides"}),
	}}
	goldenDoc(t, "pprof", doc)
}

func TestHTTPLogView(t *testing.T) {
	doc := &Doc{Title: "httplog of g1", GroupID: "g1", Sections: []Section{
		buildHTTPLogSection("app", parseHTTPLog(httplogBefore), 2),
	}}
	goldenDoc(t, "httplog", doc)
}

func TestSlowLogView(t *testing.T) {
	doc := &Doc{Title: "slowlog of g1", GroupID: "g1", Sections: []Section{
		buildSlowLogSection("db", parseSlowLog(slowlogSample), 30, 40),
	}}
	goldenDoc(t, "slowlog", doc)
}

func TestHTTPLogDiff(t *testing.T) {
	a, b := parseHTTPLog(httplogBefore), parseHTTPLog(httplogAfter)
	sec := Section{
		Title:  "httplog",
		Label:  "app",
		Status: "ok",
		Totals: totalsDiff(buildHTTPLogSection("", a, 1).Totals, buildHTTPLogSection("", b, 1).Totals),
		Tables: diffTables("endpoints by total time", []Column{{Key: "method", Header: "method"}, {Key: "uri", Header: "uri"}},
			httpLogMetrics(a), httpLogMetrics(b), fmtSec, 30),
	}
	goldenDoc(t, "httplog_diff", &Doc{Title: "httplog diff g1 -> g2", Sections: []Section{sec}})
}

func TestPProfDiff(t *testing.T) {
	a := buildProfile(t, stacksBefore, valuesBefore)
	b := buildProfile(t, stacksAfter, valuesAfter)
	fa, aa := pprofMetrics(a)
	fb, ab := pprofMetrics(b)
	fn := []Column{{Key: "function", Header: "function"}}
	var tables []*Table
	tables = append(tables, diffTables("flat (all functions)", fn, fa, fb, a.timeFmt(), 30)...)
	tables = append(tables, diffTables("cum (app code)", fn, aa, ab, a.timeFmt(), 30)...)
	goldenDoc(t, "pprof_diff", &Doc{Title: "pprof diff g1 -> g2", Sections: []Section{{Title: "pprof", Status: "ok", Tables: tables}}})
}

func TestScanSuspect(t *testing.T) {
	cases := []struct {
		r    slowRow
		want bool
	}{
		{slowRow{ExaminedAvg: 4500, SentAvg: 1}, true},
		{slowRow{ExaminedAvg: 99, SentAvg: 0}, false},
		{slowRow{ExaminedAvg: 500, SentAvg: 100}, false},
		{slowRow{ExaminedAvg: 1000, SentAvg: 100}, true},
	}
	for _, c := range cases {
		if got := scanSuspect(c.r); got != c.want {
			t.Errorf("scanSuspect(%+v) = %v, want %v", c.r, got, c.want)
		}
	}
}

func TestScoreMemo(t *testing.T) {
	for text, want := range map[string]string{
		"score=1234":             "1234",
		"note\n score = 99 \nok": "99",
		"scored 5":               "",
	} {
		got := ""
		if m := scoreRe.FindStringSubmatch(text); m != nil {
			got = m[1]
		}
		if got != want {
			t.Errorf("score in %q = %q, want %q", text, got, want)
		}
	}
}
