package db

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"github.com/btmxh/plst4/internal/errs"
	_ "github.com/lib/pq"
)

var DB *sql.DB
var GenericError = errors.New("Unable to access database")

type Tx struct {
	transaction *sql.Tx
	handler     errs.ErrorHandler
}

func (tx *Tx) PublicError(statusCode int, err error) {
	tx.handler.PublicError(statusCode, err)
}

func (tx *Tx) PrivateError(err error) {
	tx.handler.PrivateError(err)
}

type QueryRow struct {
	row *sql.Row
	tx  *Tx
}

func InitDB(connStr string) error {
	var err error
	DB, err = sql.Open("postgres", connStr)
	return err
}

func DatabaseError(handler errs.ErrorHandler, err error) {
	handler.PrivateError(err)
	handler.PublicError(http.StatusInternalServerError, GenericError)
}

func BeginTx(handler errs.ErrorHandler) *Tx {
	tx, err := DB.Begin()
	if err != nil {
		DatabaseError(handler, err)
		return nil
	}

	return &Tx{transaction: tx, handler: handler}
}

func (tx *Tx) Exec(result *sql.Result, query string, args ...any) (hasErr bool) {
	res, err := tx.transaction.Exec(query, args...)
	if err != nil {
		DatabaseError(tx.handler, err)
		return true
	}

	if result != nil {
		*result = res
	}

	return false
}

func (tx *Tx) Query(rows **sql.Rows, query string, args ...any) (hasErr bool) {
	r, err := tx.transaction.Query(query, args...)
	if err != nil {
		DatabaseError(tx.handler, err)
		return true
	}

	if rows != nil {
		*rows = r
	}

	return false
}

func (tx *Tx) QueryRow(query string, args ...any) *QueryRow {
	return &QueryRow{row: tx.transaction.QueryRow(query, args...), tx: tx}
}

func (row *QueryRow) Scan(hasRow *bool, dest ...any) (hasErr bool) {
	err := row.row.Scan(dest...)
	hasErr = err != nil && (hasRow == nil || err != sql.ErrNoRows)
	if hasErr {
		DatabaseError(row.tx.handler, err)
	}
	if hasRow != nil {
		*hasRow = err == nil
	}
	return hasErr
}

func (tx *Tx) Rollback() {
	tx.transaction.Rollback()
}

func (tx *Tx) Commit() (hasErr bool) {
	err := tx.transaction.Commit()
	if err != nil {
		DatabaseError(tx.handler, err)
		return true
	}

	return false
}

func CloseDB() {
	if err := DB.Close(); err != nil {
		slog.Warn("error while closing database", "err", err)
	}
}
