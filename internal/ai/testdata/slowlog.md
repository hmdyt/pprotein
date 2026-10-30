# slowlog of g1

- group_id: `g1`

## slowlog [db]

- queries: 5380
- total_time_s: 151.6
- distinct_queries: 3
- scan_suspects: 1

### Queries by total time

rows_examined / rows_sent are per-query averages. "scan?" marks queries that examine >=100 rows and >=10x the rows they return (likely missing index).

| count | sum | avg | max | rows_examined | rows_sent | share | flag | query |
|---:|---:|---:|---:|---:|---:|---:|---|---|
| 5000 | 150.000s | 0.030s | 0.100s | 4500 | 1 | 98.9% | scan? | SELECT `status` FROM `ride_statuses` WHE… |
| 300 | 1.200s | 0.004s | 0.020s | 0 | 0 | 0.8% |  | INSERT INTO `chair_locations` (`id`, `ch… |
| 80 | 0.400s | 0.005s | 0.010s | 7.5 | 7.5 | 0.3% |  | SELECT * FROM `chairs` WHERE `owner_id` … |
