# httplog of g1

- group_id: `g1`

## httplog [app]

- requests: 155
- total_time_s: 60
- endpoints: 3
- error_rate_4xx_5xx: 0.0194
- rate_5xx: 0.0065

### Endpoints by total response time

| count | 2xx | 4xx | 5xx | method | uri | sum | avg | p99 | max | share |
|---:|---:|---:|---:|---|---|---:|---:|---:|---:|---:|
| 100 | 98 | 2 | 0 | GET | /api/app/notification | 40.000s | 0.400s | 0.900s | 1.000s | 66.7% |
| 5 | 4 | 0 | 1 | GET | /api/owner/chairs | 10.000s | 2.000s | 3.000s | 3.000s | 16.7% |

(1 more rows omitted, 16.7% of total; use ?limit= to see more)
