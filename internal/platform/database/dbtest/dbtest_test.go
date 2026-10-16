package dbtest

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestFilesAreOrderedAndPaired(t *testing.T) {
	up, err := Files("up")
	if err != nil {
		t.Fatalf("Files(up): %v", err)
	}
	down, err := Files("down")
	if err != nil {
		t.Fatalf("Files(down): %v", err)
	}

	if len(up) == 0 || len(up) != len(down) {
		t.Fatalf("got %d up and %d down migrations; want the same non-zero number", len(up), len(down))
	}
	for i := 1; i < len(up); i++ {
		if up[i-1] >= up[i] {
			t.Errorf("up migrations out of order: %s before %s", up[i-1], up[i])
		}
	}
	if up[0][:6] != down[len(down)-1][:6] {
		t.Errorf("down order is not the reverse of up: first up %s, last down %s", up[0], down[len(down)-1])
	}
}

// TestMigrationsAreReversible applies every migration, reverts all of them,
// and applies them again. It catches down migrations that forget an object
// (the second up then fails with "already exists") or that fail outright.
func TestMigrationsAreReversible(t *testing.T) {
	t.Parallel()
	pool := New(t) // already migrated up
	ctx := t.Context()

	if err := MigrateDown(ctx, pool); err != nil {
		t.Fatalf("migrate down: %v", err)
	}
	if n := countPublicObjects(ctx, t, pool); n != 0 {
		t.Errorf("after migrating down, %d objects remain in schema public; want 0", n)
	}
	if err := MigrateUp(ctx, pool); err != nil {
		t.Fatalf("migrate up again: %v", err)
	}
}

// TestDatabasesAreIsolated proves two tests get separate databases, which
// is what lets database tests run with t.Parallel.
func TestDatabasesAreIsolated(t *testing.T) {
	t.Parallel()
	a, b := New(t), New(t)

	var nameA, nameB string
	if err := a.QueryRow(t.Context(), "SELECT current_database()").Scan(&nameA); err != nil {
		t.Fatal(err)
	}
	if err := b.QueryRow(t.Context(), "SELECT current_database()").Scan(&nameB); err != nil {
		t.Fatal(err)
	}
	if nameA == nameB {
		t.Errorf("both pools use database %q", nameA)
	}
}

func countPublicObjects(ctx context.Context, t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	// pg_class covers tables, indexes, sequences, and views.
	err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public'`).Scan(&n)
	if err != nil {
		t.Fatalf("count objects: %v", err)
	}
	return n
}
