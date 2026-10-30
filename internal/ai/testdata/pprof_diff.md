# pprof diff g1 -> g2


## pprof

### flat (all functions): changed (largest absolute change first)

| function | before | after | delta | change | share_before | share_after |
|---|---:|---:|---:|---:|---:|---:|
| syscall.Syscall | 6.000s | 3.000s | -3.000s | -50.0% | 75.0% | 66.7% |
| runtime.gcBgMarkWorker | 1.000s | 1.000s | +0.000s | +0.0% | 12.5% | 22.2% |

### flat (all functions): new (only after)

| function | after | share_after |
|---|---:|---:|
| main.cacheLookup | 0.500s | 11.1% |

### flat (all functions): gone (only before)

| function | before | share_before |
|---|---:|---:|
| encoding/json.Marshal | 1.000s | 12.5% |

### cum (app code): changed (largest absolute change first)

| function | before | after | delta | change | share_before | share_after |
|---|---:|---:|---:|---:|---:|---:|
| main.getRides | 5.000s | 1.500s | -3.500s | -70.0% | 62.5% | 33.3% |
| main.postCoordinate | 2.000s | 2.000s | +0.000s | +0.0% | 25.0% | 44.4% |

### cum (app code): new (only after)

| function | after | share_after |
|---|---:|---:|
| main.cacheLookup | 0.500s | 11.1% |

### cum (app code): gone (only before)

(no rows)
