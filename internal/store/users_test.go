package store_test

// These tests pin down the users table contract: what the database accepts,
// what it rejects, and under which constraint name. The auth service relies
// on those names to turn database errors into API errors, so a migration
// that renames or drops a constraint must break a test here first.

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kaezaen/ai-interview-platform-remake/internal/platform/database"
	"github.com/Kaezaen/ai-interview-platform-remake/internal/platform/database/dbtest"
	"github.com/Kaezaen/ai-interview-platform-remake/internal/store"
)

// fakeHash has the argon2id PHC prefix the schema requires. Hashing real
// passwords is the auth package's job, not this test's.
const fakeHash = "$argon2id$v=19$m=65536,t=1,p=4$c2FsdHNhbHQ$aGFzaGhhc2g"

func validUser() store.CreateUserParams {
	return store.CreateUserParams{
		Email:        "ada@example.com",
		DisplayName:  "Ada",
		PasswordHash: fakeHash,
	}
}

func TestCreateUserAndReadBack(t *testing.T) {
	t.Parallel()
	q := store.New(dbtest.New(t))
	ctx := t.Context()

	before := time.Now()
	created, err := q.CreateUser(ctx, validUser())
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if created.ID.Version() != 7 {
		t.Errorf("id %s is UUID version %d, want 7 (uuidv7 default)", created.ID, created.ID.Version())
	}
	// Allow for clock skew between the test process and the database.
	if created.CreatedAt.Before(before.Add(-time.Minute)) || created.CreatedAt.After(time.Now().Add(time.Minute)) {
		t.Errorf("created_at %v is not close to now", created.CreatedAt)
	}

	byEmail, err := q.GetUserByEmail(ctx, "ada@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if byEmail.ID != created.ID || byEmail.PasswordHash != fakeHash {
		t.Errorf("GetUserByEmail = %+v, want id %s with the stored hash", byEmail, created.ID)
	}

	byID, err := q.GetUserByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if byID.Email != "ada@example.com" || byID.DisplayName != "Ada" {
		t.Errorf("GetUserByID = %+v", byID)
	}
}

func TestGetUserMissingReturnsErrNoRows(t *testing.T) {
	t.Parallel()
	q := store.New(dbtest.New(t))

	_, err := q.GetUserByEmail(t.Context(), "nobody@example.com")
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("GetUserByEmail(missing) error = %v, want pgx.ErrNoRows", err)
	}
	_, err = q.GetUserByID(t.Context(), uuid.New())
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("GetUserByID(missing) error = %v, want pgx.ErrNoRows", err)
	}
}

func TestDuplicateEmailViolatesUsersEmailKey(t *testing.T) {
	t.Parallel()
	q := store.New(dbtest.New(t))
	ctx := t.Context()

	if _, err := q.CreateUser(ctx, validUser()); err != nil {
		t.Fatalf("first CreateUser: %v", err)
	}
	second := validUser()
	second.DisplayName = "Someone Else"

	_, err := q.CreateUser(ctx, second)
	if !database.IsUniqueViolation(err, "users_email_key") {
		t.Errorf("duplicate email error = %v, want unique violation on users_email_key", err)
	}
}

func TestUsersCheckConstraints(t *testing.T) {
	t.Parallel()
	q := store.New(dbtest.New(t))

	tests := []struct {
		name       string
		modify     func(*store.CreateUserParams)
		constraint string
	}{
		{"uppercase email", func(p *store.CreateUserParams) { p.Email = "Ada@Example.com" }, "users_email_normalized"},
		{"email with surrounding spaces", func(p *store.CreateUserParams) { p.Email = " ada@example.com" }, "users_email_normalized"},
		{"email too short", func(p *store.CreateUserParams) { p.Email = "a@" }, "users_email_length"},
		{"empty display name", func(p *store.CreateUserParams) { p.DisplayName = "" }, "users_display_name_trimmed"},
		{"display name with trailing space", func(p *store.CreateUserParams) { p.DisplayName = "Ada " }, "users_display_name_trimmed"},
		{"display name over 100 characters", func(p *store.CreateUserParams) { p.DisplayName = strings.Repeat("a", 101) }, "users_display_name_trimmed"},
		// Length is counted in characters, not bytes: 100 multi-byte
		// characters must still fit.
		{"100 multi-byte characters are allowed", func(p *store.CreateUserParams) { p.DisplayName = strings.Repeat("é", 100) }, ""},
		{"plaintext password", func(p *store.CreateUserParams) { p.PasswordHash = "hunter2" }, "users_password_hash_argon2id"},
		{"bcrypt hash", func(p *store.CreateUserParams) {
			p.PasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
		}, "users_password_hash_argon2id"},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validUser()
			// Distinct emails, so a row accepted by one case cannot make a
			// later case fail with a duplicate-email error instead.
			p.Email = fmt.Sprintf("case%d@example.com", i)
			tt.modify(&p)

			_, err := q.CreateUser(t.Context(), p)
			if tt.constraint == "" {
				if err != nil {
					t.Errorf("valid input rejected: %v", err)
				}
				return
			}
			if !database.IsCheckViolation(err, tt.constraint) {
				t.Errorf("error = %v, want check violation on %s", err, tt.constraint)
			}
		})
	}
}
