# AI Interview Platform (remake)

Text-based mock interviews for backend engineers, scored against explicit
rubrics, with feedback grounded in quotes from your own answers.

> **Status: walking skeleton.** The service starts, connects to PostgreSQL,
> serves health checks, and shuts down gracefully. Interview features are not
> built yet; see [Roadmap](#roadmap).

## Why this exists

Practising technical interviews alone gives no feedback, human mock
interviews are hard to schedule, and general-purpose chatbots grade
inconsistently and tend to flatter. This project aims for feedback that is
rubric-based, cites evidence from the answer, and whose consistency is
measured rather than assumed.

## Stack

Go 1.27 (`net/http`, `log/slog`) · PostgreSQL 18 · pgx v5 · Docker Compose ·
GitHub Actions · golangci-lint

## Prerequisites

| Tool | Version | Check |
|---|---|---|
| Go | 1.27.2 or newer | `go version` |
| Docker | any recent | `docker version` |
| golangci-lint (optional, for local linting) | v2.14.0 | `golangci-lint version` |

## Run locally

**1. Start PostgreSQL**

```sh
docker compose up -d
```

**2. Create your `.env`**

```powershell
# PowerShell
Copy-Item .env.example .env
```

```sh
# bash / zsh
cp .env.example .env
```

**3. Run the API**

```sh
go run ./cmd/api
```

**4. Check it**

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

## Development

```sh
go test -race ./...     # tests with the race detector
golangci-lint run       # lint + formatting checks (same as CI)
```

CI runs both on every push to `main` and on every pull request.

## Project layout

```
cmd/api/                 entry point: config → logger → database → server
internal/
  platform/config/       environment variables → validated Config
  platform/logging/      slog setup + request-scoped logger in context
  platform/database/     pgx connection pool
  httpx/                 JSON envelope, error codes, middleware
  health/                /healthz and /readyz
  server/                routes, middleware chain, http.Server timeouts
```

## Roadmap

- [x] M1 — Walking skeleton: config, logging, graceful shutdown, health checks, CI
- [ ] M2 — Accounts and sessions: auth, interview sessions, question bank
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
