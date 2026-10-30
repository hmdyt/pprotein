# httplog diff g1 -> g2


## httplog [app]

- requests: 155 -> 190 (+35)
- total_time_s: 60 -> 26 (-34)
- endpoints: 3 -> 3 (+0)
- error_rate_4xx_5xx: 0.0194 -> 0 (-0.0194)
- rate_5xx: 0.0065 -> 0 (-0.0065)

### endpoints by total time: changed (largest absolute change first)

| method | uri | count_before | count_after | before | after | delta | change | share_before | share_after |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| GET | /api/app/notification | 100 | 120 | 40.000s | 12.000s | -28.000s | -70.0% | 66.7% | 46.2% |
| POST | /api/chair/coordinate | 50 | 60 | 10.000s | 11.000s | +1.000s | +10.0% | 16.7% | 42.3% |

### endpoints by total time: new (only after)

| method | uri | count_after | after | share_after |
|---|---|---:|---:|---:|
| GET | /api/app/nearby-chairs | 10 | 3.000s | 11.5% |

### endpoints by total time: gone (only before)

| method | uri | count_before | before | share_before |
|---|---|---:|---:|---:|
| GET | /api/owner/chairs | 5 | 10.000s | 16.7% |
