package integration

import (
	"database/sql"
	"testing"
)

func QueryRow(t *testing.T, db interface{ QueryRow(string, ...interface{}) *sql.Row }, query string, args ...interface{}) *sql.Row {
	t.Helper()
	return db.QueryRow(query, args...)
}
