package database

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsUniqueViolation(t *testing.T) {
	unique := &pgconn.PgError{Code: codeUniqueViolation, ConstraintName: "users_email_key"}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"matching constraint", unique, true},
		{"wrapped by fmt.Errorf", fmt.Errorf("create user: %w", unique), true},
		{"other unique constraint", &pgconn.PgError{Code: codeUniqueViolation, ConstraintName: "users_username_key"}, false},
		{"same constraint, other code", &pgconn.PgError{Code: codeCheckViolation, ConstraintName: "users_email_key"}, false},
		{"not a database error", errors.New("boom"), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUniqueViolation(tt.err, "users_email_key"); got != tt.want {
				t.Errorf("IsUniqueViolation = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsCheckViolation(t *testing.T) {
	check := &pgconn.PgError{Code: codeCheckViolation, ConstraintName: "users_email_normalized"}

	if !IsCheckViolation(fmt.Errorf("wrapped: %w", check), "users_email_normalized") {
		t.Error("wrapped check violation not recognised")
	}
	if IsCheckViolation(check, "users_email_length") {
		t.Error("check violation matched the wrong constraint")
	}
}
