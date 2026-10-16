package database

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// SQLSTATE codes the application reacts to. Full list:
// https://www.postgresql.org/docs/current/errcodes-appendix.html
const (
	codeUniqueViolation = "23505"
	codeCheckViolation  = "23514"
)

// IsUniqueViolation reports whether err is a PostgreSQL unique violation on
// the named constraint.
//
// Matching the constraint name, not only the error code, matters as soon as
// a table has two unique constraints: "email already registered" and, say,
// "username taken" must become different API errors.
func IsUniqueViolation(err error, constraint string) bool {
	return isViolation(err, codeUniqueViolation, constraint)
}

// IsCheckViolation reports whether err is a PostgreSQL CHECK violation on
// the named constraint.
func IsCheckViolation(err error, constraint string) bool {
	return isViolation(err, codeCheckViolation, constraint)
}

func isViolation(err error, code, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code && pgErr.ConstraintName == constraint
}
