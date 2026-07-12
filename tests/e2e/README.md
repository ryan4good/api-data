# System API MySQL E2E

The harness creates and later drops the dedicated `bizdevops_e2e` database,
applies the repository migration and development seed, builds and starts the Go
API on a free loopback port, then exercises the System API over HTTP.

Set `MYSQL_PASSWORD` in the invoking process; the value is never stored in a
file or passed on a command line. Optional connection variables are
`MYSQL_HOST`, `MYSQL_PORT`, and `MYSQL_USER`.

```powershell
python -m unittest tests.e2e.test_system_api_mysql -v
```

The runner terminates the API and drops the E2E database on success and failure.
