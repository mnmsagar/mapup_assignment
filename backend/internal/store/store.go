// Package store contains all SQL access. Coordinates are (lat, lng) in Go and
// converted to PostGIS (lng, lat) order only inside SQL.
package store

import (
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("conflict")
	ErrInvalidGeometry = errors.New("invalid geometry")
)

// NotFoundError says which referenced entity is missing.
type NotFoundError struct{ Entity, ID string }

func (e *NotFoundError) Error() string { return e.Entity + " " + e.ID + " not found" }
func (e *NotFoundError) Unwrap() error { return ErrNotFound }

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func pgCode(err error) (code, constraint string) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code, pgErr.ConstraintName
	}
	return "", ""
}

const (
	codeUniqueViolation     = "23505"
	codeForeignKeyViolation = "23503"
)

// whereBuilder accumulates "col = $n" style predicates.
type whereBuilder struct {
	parts []string
	args  []any
}

func (w *whereBuilder) add(expr string, arg any) {
	w.args = append(w.args, arg)
	w.parts = append(w.parts, strings.ReplaceAll(expr, "?", "$"+strconv.Itoa(len(w.args))))
}

func (w *whereBuilder) sql() string {
	if len(w.parts) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(w.parts, " AND ")
}

// nextArg returns the placeholder for an argument appended after the filters.
func (w *whereBuilder) nextArg(arg any) string {
	w.args = append(w.args, arg)
	return "$" + strconv.Itoa(len(w.args))
}
