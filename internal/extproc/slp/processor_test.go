package slp

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteNormalizedPGLog(t *testing.T) {
	in := strings.Join([]string{
		"2024-01-01 00:00:00.000 UTC [1] app@app LOG:  duration: 0.050 ms  parse stmtcache_1: SELECT *",
		"\tFROM users WHERE id = $1",
		"2024-01-01 00:00:00.001 UTC [1] app@app LOG:  duration: 0.010 ms  bind stmtcache_1: SELECT *",
		"\tFROM users WHERE id = $1",
		"2024-01-01 00:00:00.002 UTC [1] app@app LOG:  duration: 1.500 ms  execute stmtcache_1: SELECT *",
		"\tFROM users WHERE id = $1",
		"2024-01-01 00:00:00.003 UTC [1] app@app LOG:  duration: 2.000 ms  execute <unnamed>: DELETE FROM users WHERE id = $1",
		"2024-01-01 00:00:00.004 UTC [1] app@app LOG:  duration: 3.000 ms  statement: UPDATE users SET name = 'a'",
		"2024-01-01 00:00:00.005 UTC [2] LOG:  checkpoint complete: wrote 1 buffers",
		"",
	}, "\n")
	want := strings.Join([]string{
		"2024-01-01 00:00:00.002 UTC [1] app@app LOG:  duration: 1.500 ms  statement: SELECT *",
		"\tFROM users WHERE id = $1",
		"2024-01-01 00:00:00.003 UTC [1] app@app LOG:  duration: 2.000 ms  statement: DELETE FROM users WHERE id = $1",
		"2024-01-01 00:00:00.004 UTC [1] app@app LOG:  duration: 3.000 ms  statement: UPDATE users SET name = 'a'",
		"",
	}, "\n")

	var out bytes.Buffer
	if err := writeNormalizedPGLog(&out, strings.NewReader(in)); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
