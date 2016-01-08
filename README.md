# fluSQL
An Influxdb to SQL Bridge
idea from https://github.com/sysadminmike/postgres-influx-mimic/
Supported databases: Mysql and PostgreSQL for now.

## Run

copy the config file
```bash
cp config.toml{.dist,}
```

```bash
go build
./str
```

## Usage

Then add to grafana the following query
The query must have:
- as "time" column as a unix micro timestamp
- another column castable to uint64
- in grafana you can use translateTimePart($timeFilter) to filter your queries by time: translateTimePart($timeFilter) will be replaced by the appropriate expression for your database. (ie: `time > now()-'2days'::interval AND time < now()` )

```sql
select UNIX_TIMESTAMP(time)*1000 as time, job_count Count from (
    select start_time as time, count(id) as 'job_count'
    from job_queue  
    group by YEAR(start_time), MONTH(start_time) ,  DAY(start_time) ) as x
where translateTimePart($timeFilter) order by time asc
```

```sql
select UNIX_TIMESTAMP(created_time)*1000 as time , count(*) as post_count from post
group by YEAR(created_time), MONTH(created_time) ,  DAY(created_time)
order by time asc
 ```

```sql
select ct, cast(EXTRACT(EPOCH FROM time)*1000 as numeric) from (select count(*) as ct, date_trunc('day',created_time) as time
from comment group by time order by time desc) as x
where time > (now() - '24 week'::interval)
```

## Todo

- [x] configuration file with db type, db url / or cmd lines args
- [x] Simple Auth
- [x] other type of fields than uint ...
  [x] try to work with "time > now() - 1y"
  [ ] cleanup
- [ ] Tests
- [ ] doc!
