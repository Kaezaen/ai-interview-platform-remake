// Package migrations embeds the SQL migration files so Go code (today: the
// test harness in internal/platform/database/dbtest) can apply them without
// knowing where the repository sits on disk.
//
// The files follow golang-migrate naming: NNNNNN_description.up.sql and a
// matching .down.sql. Create new ones with:
//
//	migrate create -ext sql -dir db/migrations -seq -digits 6 add_something
//
// A migration that has been merged into main is never edited; changes go
// into a new migration.
package migrations

import "embed"

// FS holds every *.sql file in this directory.
//
//go:embed *.sql
var FS embed.FS
