# Rules for coding agents in this repository

The owner of this repository designs, reviews, and commits every change.
Coding agents (Claude Code or any other) are allowed to do one thing here:
**fix the specific code the owner points to.**

## Hard limits

1. Change only the file, function, or lines the owner explicitly names.
2. Never create new files.
3. Never implement anything that does not exist yet: no new features,
   endpoints, migrations, queries, tests, or TODO items, even when they look
   obviously needed.
4. If the fix requires breaking rule 1, 2, or 3, stop. Explain what would
   be needed and why, and wait. Do not do it.
5. Do not refactor, rename, reformat, or "clean up" code outside the target.

## After every change

- Run `gofmt -l .`, `go vet ./...`, and `go test ./...` (add `-race` where a
  C compiler is available). Set `TEST_DATABASE_URL` first, or the database
  tests are skipped and prove nothing (see README › Testing).
- If you touched `db/queries` or `db/migrations`, run `sqlc generate`.
- Report what you changed, why it fixes the problem, and the command output.
- Never claim something works without having run it.

## Project conventions (so fixes stay consistent)

- Layering: handler → service → database. Handlers never contain SQL;
  services never touch `http.Request` or `http.ResponseWriter`.
- Every JSON response goes through `internal/httpx` (`Data`, `Error`,
  `InternalError`). Unexpected errors use `httpx.InternalError`, which logs
  the cause and hides it from the client.
- Log through `logging.FromContext(ctx)` so lines carry the request id.
  Use key-value pairs: `logger.Info("msg", "key", value)`.
- Wrap errors with context: `fmt.Errorf("doing X: %w", err)`.
- Configuration comes only from `internal/platform/config`.

## Database rules

- Never edit a migration in `db/migrations` that exists on `main`. A fix
  to the schema is a new migration, and creating one is a new file, so it
  is the owner's job (rule 2).
- Never edit generated code in `internal/store` (`db.go`, `models.go`,
  `*.sql.go`). Change the SQL in `db/queries`, then run `sqlc generate`.
- Map database errors by constraint name with
  `database.IsUniqueViolation` / `database.IsCheckViolation`, never by
  matching error message text.
