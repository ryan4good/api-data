# System overview real-data completion

## Scope

This slice removes the remaining hard-coded empty states from the single-system
overview and extends the authorized management projection with operational health
data. It does not deploy the TLS-gated login build to the current public HTTP
trial.

## Parallel TDD work

Three independent work streams ran in parallel:

- The API management read model added member, active environment, active code
  source, and latest-run metrics while retaining member/platform-admin scope.
- The system workspace added real scenario and run previews using the existing
  scoped endpoints.
- The management summary rendered the new health metrics without turning absent
  or partial values into fake zeroes.

Each stream first introduced failing contract tests. Integration review then
found that the API intentionally emits `lastRunAt: null` for a system that has
never run, while the Web type allowed only string/undefined. A TypeScript Red run
reproduced the mismatch; the type and formatter now explicitly accept null.

## Result

- Scenario and run cards independently render loading, error, empty, and ready
  states, show the real total, and preview at most three records.
- Five workspace resources load concurrently through `Promise.allSettled`; one
  failed endpoint does not erase successful cards.
- The system management summary now shows member count, environment count, code
  source count, and the latest run time.
- The MySQL query counts only active environments and code sources, counts system
  members, and obtains `MAX(scenario_runs.created_at)`.
- Missing metrics render as `—`; a system with no runs receives an explicit null
  timestamp.

## Verification

- `go test -count=1 ./...`: passed.
- `go vet ./...`: passed.
- Web: 13 files and 55 tests passed.
- Web TypeScript/Vite production build: passed.
- Python DB/deployment contracts: 21 passed.
- A Linux amd64 API ran once on Tencent Cloud loopback `127.0.0.1:18082` against
  the real MariaDB. The new management fields, scenario list, and run list all
  returned the expected JSON shapes. The temporary unit, env, and release were
  removed; the public `18080` service was unchanged.
