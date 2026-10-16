// Package dbtest gives each test its own freshly migrated PostgreSQL
// database.
//
// Database tests run against a real PostgreSQL server, not a mock.
// Constraints, locking, and transactions are exactly what these tests must
// exercise, and a mock would re-implement them, wrongly.
//
// Set TEST_DATABASE_URL to any database on a server where that user may
// create databases (the docker compose database works). Every call to New
// creates a database named test_<random>, applies all up migrations, and
// drops it when the test ends. Tests therefore never see each other's rows
// and can run in parallel.
package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Kaezaen/ai-interview-platform-remake/db/migrations"
)

// EnvURL names the environment variable that points at the test server.
const EnvURL = "TEST_DATABASE_URL"

// New returns a pool connected to a brand-new database with every up
// migration applied. The database is dropped when the test finishes.
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()

	adminURL := os.Getenv(EnvURL)
	if adminURL == "" {
		// GitHub Actions and most CI systems set CI=true. There a missing
		// database must fail loudly: a skipped test shows up green while
		// testing nothing.
		if os.Getenv("CI") != "" {
			t.Fatalf("%s must be set in CI", EnvURL)
		}
		t.Skipf("database test skipped: set %s to run it (see README)", EnvURL)
	}

	ctx := t.Context()
	name := "test_" + randomHex(8)

	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("dbtest: connect to %s: %v", EnvURL, err)
	}
	_, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize())
	_ = admin.Close(ctx)
	if err != nil {
		t.Fatalf("dbtest: create database %s: %v", name, err)
	}

	// Cleanups run last-in first-out. Registering the drop first means it
	// runs last, after the pool below has been closed, and it runs even if
	// a later step fails the test.
	t.Cleanup(func() { dropDatabase(t, adminURL, name) })

	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		t.Fatalf("dbtest: parse %s: %v", EnvURL, err)
	}
	cfg.ConnConfig.Database = name

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("dbtest: connect to %s: %v", name, err)
	}
	t.Cleanup(pool.Close)

	if err := MigrateUp(ctx, pool); err != nil {
		t.Fatalf("dbtest: %v", err)
	}
	return pool
}

// Execer is the part of pgx that applying a migration needs. Both
// *pgxpool.Pool and *pgx.Conn satisfy it.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// MigrateUp applies every up migration in version order.
func MigrateUp(ctx context.Context, db Execer) error {
	return apply(ctx, db, "up")
}

// MigrateDown reverts every migration, newest first.
func MigrateDown(ctx context.Context, db Execer) error {
	return apply(ctx, db, "down")
}

// fileName matches golang-migrate's naming: 000001_create_users.up.sql.
var fileName = regexp.MustCompile(`^(\d{6})_[a-z0-9_]+\.(up|down)\.sql$`)

// Files returns the migration file names for one direction, in the order
// they must run. Every .sql file must follow the naming scheme and every up
// migration must have a down migration; anything else is an error, so a
// typo in a file name fails tests instead of being silently skipped.
func Files(direction string) ([]string, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}

	versions := map[string]map[string]string{} // version -> direction -> file
	for _, e := range entries {
		m := fileName.FindStringSubmatch(e.Name())
		if m == nil {
			return nil, fmt.Errorf("migration %q does not match NNNNNN_name.(up|down).sql", e.Name())
		}
		version, dir := m[1], m[2]
		if versions[version] == nil {
			versions[version] = map[string]string{}
		}
		if prev, dup := versions[version][dir]; dup {
			return nil, fmt.Errorf("migrations %q and %q share version %s", prev, e.Name(), version)
		}
		versions[version][dir] = e.Name()
	}

	keys := make([]string, 0, len(versions))
	for v, files := range versions {
		if files["up"] == "" || files["down"] == "" {
			return nil, fmt.Errorf("migration version %s needs both an .up.sql and a .down.sql file", v)
		}
		keys = append(keys, v)
	}
	// Six-digit, zero-padded versions sort correctly as strings.
	slices.Sort(keys)
	if direction == "down" {
		slices.Reverse(keys)
	}

	out := make([]string, 0, len(keys))
	for _, v := range keys {
		out = append(out, versions[v][direction])
	}
	return out, nil
}

func apply(ctx context.Context, db Execer, direction string) error {
	files, err := Files(direction)
	if err != nil {
		return err
	}
	for _, f := range files {
		sql, err := fs.ReadFile(migrations.FS, f)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}
		// Exec without arguments uses PostgreSQL's simple query protocol,
		// which accepts several statements in one call, the same way the
		// migrate CLI runs a file.
		if _, err := db.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("apply migration %s: %w", f, err)
		}
	}
	return nil
}

func dropDatabase(t testing.TB, adminURL, name string) {
	// t.Context() is already cancelled while cleanups run, so use a fresh
	// context with its own deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Errorf("dbtest: connect to drop %s: %v", name, err)
		return
	}
	defer func() { _ = admin.Close(ctx) }()

	// WITH (FORCE) disconnects anything still attached, e.g. a connection a
	// failing test leaked.
	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
		t.Errorf("dbtest: drop database %s: %v", name, err)
	}
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b) // never fails since Go 1.24
	return hex.EncodeToString(b)
}
