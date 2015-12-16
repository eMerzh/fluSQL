idea from https://github.com/sysadminmike/postgres-influx-mimic/

== Run ==

```bash
go build
./str
```

Then add to grafana the following query
The query must have:
- as "time" column as a unix micro timestamp
- another column castable to uint64

```sql
select UNIX_TIMESTAMP(start_time)*1000 as time,count(id) as 'Job Count'
from job_queue  group by YEAR(start_time), MONTH(start_time) ,  DAY(start_time) order by start_time asc
```

```sql
select count(*), cast(EXTRACT(EPOCH FROM date_trunc('day',created_time)) *1000 as numeric) as time
from comment group by time order by time desc
```

== Todo ==

- [x] configuration file with db type, db url / or cmd lines args
- [ ] Simple Auth
- [ ] Tests
- [ ] other type of fields than uint ...
- [ ] multiple db types?
- [ ] doc!


=== Config to manage ===
* queries???
