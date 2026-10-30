# pprof of g1

- group_id: `g1`

## pprof [app]

- total_cpu_s: 8
- duration_s: 10
- cores_assumed: 2
- cpu_utilization: 0.4

### Top functions by flat (time in the function itself)

| function | location | flat | flat% | cum | cum% |
|---|---|---:|---:|---:|---:|
| syscall.Syscall | app/syscall.Syscall.go:10 | 6.000s | 75.0% | 6.000s | 75.0% |
| encoding/json.Marshal | encoding/json.Marshal.go:10 | 1.000s | 12.5% | 1.000s | 12.5% |
| runtime.gcBgMarkWorker | app/runtime.gcBgMarkWorker.go:10 | 1.000s | 12.5% | 1.000s | 12.5% |

(5 more rows omitted, 0.0% of total; use ?limit= to see more)

### Top functions by cum (including callees)

cum shares overlap and do not add up to 100%.

| function | location | flat | flat% | cum | cum% |
|---|---|---:|---:|---:|---:|
| net/http.serve | net/http.serve.go:10 | 0.000s | 0.0% | 7.000s | 87.5% |
| syscall.Syscall | app/syscall.Syscall.go:10 | 6.000s | 75.0% | 6.000s | 75.0% |
| main.getRides | app/main.getRides.go:10 | 0.000s | 0.0% | 5.000s | 62.5% |

(5 more rows omitted; use ?limit= to see more)

### App code (package main) by cum

Where the application's own handlers and helpers spend time, including the DB and libraries they call.

| function | location | flat | flat% | cum | cum% |
|---|---|---:|---:|---:|---:|
| main.getRides | app/main.getRides.go:10 | 0.000s | 0.0% | 5.000s | 62.5% |
| main.postCoordinate | app/main.postCoordinate.go:10 | 0.000s | 0.0% | 2.000s | 25.0% |

### Focus: callers

Functions matching "getRides": main.getRides. Values are time of samples passing through them.

| function | time | share |
|---|---:|---:|
| net/http.serve | 5.000s | 100.0% |

### Focus: callees

Functions matching "getRides": main.getRides. Values are time of samples passing through them.

| function | time | share |
|---|---:|---:|
| database/sql.Query | 4.000s | 80.0% |
| encoding/json.Marshal | 1.000s | 20.0% |
