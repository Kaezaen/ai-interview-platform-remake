# AI Interview Platform (remake)

Text-based mock interviews for backend engineers, scored against explicit
rubrics, with feedback grounded in quotes from your own answers.

> **Status: early development.** The service starts, connects to PostgreSQL,
> serves health checks, and shuts down gracefully. The database layer
> (migrations, generated queries, tests against a real PostgreSQL) is in
> place. Interview features are not built yet; see [Roadmap](#roadmap).

## Why this exists

Practising technical interviews alone gives no feedback, human mock
interviews are hard to schedule, and general-purpose chatbots grade
inconsistently and tend to flatter. This project aims for feedback that is
rubric-based, cites evidence from the answer, and whose consistency is
measured rather than assumed.

## Stack

Go 1.27 (`net/http`, `log/slog`) · PostgreSQL 18 · pgx v5 · sqlc ·
golang-migrate · Docker Compose · GitHub Actions · golangci-lint

## Prerequisites

| Tool | Version | Check |
|---|---|---|
| Go | 1.27.2 or newer | `go version` |
| Docker | any recent | `docker version` |
| golang-migrate CLI | v4.20 or newer | `migrate -version` |
| sqlc (only to regenerate queries) | v1.31.1 | `sqlc version` |
| golangci-lint (optional, for local linting) | v2.14.0 | `golangci-lint version` |

Install the two database CLIs with Go (they land in `$(go env GOPATH)\bin`,
which must be on your `PATH`):

```sh
go install -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.20.1
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
```

## Run locally

**1. Start PostgreSQL**

```sh
docker compose up -d
```

**2. Apply database migrations**

```powershell
# PowerShell
$env:DATABASE_URL = "postgres://coach:coach_dev_password@localhost:5433/coach_dev?sslmode=disable"
migrate -path db/migrations -database $env:DATABASE_URL up
```

```sh
# bash / zsh
export DATABASE_URL="postgres://coach:coach_dev_password@localhost:5433/coach_dev?sslmode=disable"
migrate -path db/migrations -database "$DATABASE_URL" up
```

**3. Create your `.env`**

```powershell
# PowerShell
Copy-Item .env.example .env
```

```sh
# bash / zsh
cp .env.example .env
```

**4. Run the API**

```sh
go run ./cmd/api
```

**5. Check it**

```sh
curl http://localhost:8080/healthz   # {"data":{"status":"ok"}}
curl http://localhost:8080/readyz    # {"data":{"status":"ready"}}
```

On Windows PowerShell use `curl.exe` instead of `curl`.

Stop the server with Ctrl+C; it finishes in-flight requests before exiting.

## Configuration

All configuration comes from environment variables (a `.env` file is loaded
in development if present).

| Variable | Required | Default | Notes |
|---|---|---|---|
| `DATABASE_URL` | yes | — | PostgreSQL connection string |
| `APP_ENV` | no | `development` | `development` or `production` |
| `HTTP_ADDR` | no | `:8080` | Listen address |
| `LOG_LEVEL` | no | `info` | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | no | `text` in development, `json` in production | `text` or `json` |
| `SHUTDOWN_TIMEOUT` | no | `15s` | Grace period for in-flight requests |

The server refuses to start on invalid configuration and lists every problem
at once.

## HTTP API

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Liveness: the process is up. Never checks dependencies. |
| `GET` | `/readyz` | Readiness: the database answers within 2 s. `503` otherwise. |

Every JSON response uses one envelope:

```json
{ "data": { } }
{ "error": { "code": "NOT_FOUND", "message": "Route not found." } }
```

Every response carries an `X-Request-ID` header. The same id appears on
every server log line for that request.

## Testing

Database tests run against a real PostgreSQL, not mocks. Each test creates
its own throwaway database (`test_<random>`), applies all migrations, and
drops it afterwards, so tests never share rows and can run in parallel.
Point `TEST_DATABASE_URL` at the compose database; your development data
is not touched.

```powershell
# PowerShell
$env:TEST_DATABASE_URL = "postgres://coach:coach_dev_password@localhost:5433/coach_dev?sslmode=disable"
go test ./...
```

```sh
# bash / zsh
export TEST_DATABASE_URL="postgres://coach:coach_dev_password@localhost:5433/coach_dev?sslmode=disable"
go test -race ./...
```

Without `TEST_DATABASE_URL`, database tests are skipped locally. In CI they
fail instead, so a misconfigured pipeline cannot pass while testing nothing.

The race detector (`-race`) needs cgo and a C compiler. On Windows without
one, run `go test ./...`; CI always runs with `-race`.

```sh
golangci-lint run       # lint + formatting checks (same as CI)
```

CI runs on every push to `main` and on every pull request: tests with
`-race` against PostgreSQL 18, migrations up/down/up with the real
`migrate` CLI, golangci-lint, and a check that generated code is current.

## Database changes

Schema changes are SQL files in `db/migrations`, applied in order by
[golang-migrate](https://github.com/golang-migrate/migrate).

```sh
migrate create -ext sql -dir db/migrations -seq -digits 6 add_something
```

- Write both the `.up.sql` and the `.down.sql`; tests check that every
  migration can be reverted cleanly.
- **Never edit a migration that is already on `main`.** Databases that ran
  it will not run it again. Fix it with a new migration.

Queries are SQL files in `db/queries`. [sqlc](https://sqlc.dev) generates
type-safe Go from them into `internal/store`:

```sh
sqlc generate
```

Run it after changing `db/queries` or `db/migrations`, and commit the
generated files. Never edit `internal/store/*.sql.go`, `db.go`, or
`models.go` by hand.

## Project layout

```
cmd/api/                    entry point: config → logger → database → server
db/
  migrations/               SQL migrations (golang-migrate), embedded for tests
  queries/                  SQL queries → sqlc input
internal/
  platform/config/          environment variables → validated Config
  platform/logging/         slog setup + request-scoped logger in context
  platform/database/        pgx pool, PostgreSQL error helpers
  platform/database/dbtest/ throwaway migrated database per test
  store/                    sqlc-generated queries (do not edit) + their tests
  httpx/                    JSON envelope, error codes, middleware
  health/                   /healthz and /readyz
  server/                   routes, middleware chain, http.Server timeouts
```

## Roadmap

- [x] M1 — Walking skeleton: config, logging, graceful shutdown, health checks, CI
- [ ] M2 — Accounts and sessions: auth, interview sessions, question bank
  - [x] M2.1 — Database foundation: migrations, sqlc, real-PostgreSQL tests
  - [ ] M2.2 — Register and login
  - [ ] M2.3 — Refresh-token rotation
  - [ ] M2.4 — Question bank and interview sessions
- [ ] M3 — LLM gateway and interviewer follow-ups (streamed over SSE)
- [ ] M4 — Asynchronous evaluation pipeline on a PostgreSQL job queue
- [ ] M5 — Evaluation harness: golden dataset, prompt versioning, injection tests
- [ ] M6 — Web frontend
- [ ] M7 — Observability, cost tracking, rate limits
- [ ] M8 — Documentation site and deployment

## How this project is built

Code in this repository is drafted with an AI mentor (Claude) in small,
reviewable increments. I review, run, and commit every change myself, and
I keep a record of why each design decision was made. Coding agents working
in the repository are restricted to targeted fixes; see [CLAUDE.md](CLAUDE.md).
