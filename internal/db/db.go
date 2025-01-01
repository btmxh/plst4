package db

import (
	"database/sql"
	"errors"
	"log/slog"

	"github.com/btmxh/plst4/internal/errs"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

var DB *sql.DB
var GenericError = errors.New("Unable to access database")

type Tx struct {
	transaction *sql.Tx
	context     *gin.Context
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

func DatabaseError(c *gin.Context, err error) {
	errs.PrivateError(c, err)
	errs.PublicError(c, GenericError)
}

func BeginTx(c *gin.Context) *Tx {
	tx, err := DB.Begin()
	if err != nil {
		DatabaseError(c, err)
		return nil
	}

	return &Tx{transaction: tx, context: c}
}

func (tx *Tx) Exec(result *sql.Result, query string, args ...any) (hasErr bool) {
	res, err := tx.transaction.Exec(query, args...)
	if err != nil {
		DatabaseError(tx.context, err)
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
		DatabaseError(tx.context, err)
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
	hasErr = err != nil && err != sql.ErrNoRows
	if hasErr {
		DatabaseError(row.tx.context, err)
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
		DatabaseError(tx.context, err)
		return true
	}

	return false
}

func CloseDB() {
	if err := DB.Close(); err != nil {
		slog.Warn("error while closing database", "err", err)
	}
}
