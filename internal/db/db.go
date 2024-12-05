package db

import (
	_ "github.com/lib/pq"
	"database/sql"
	"log/slog"
)

var DB *sql.DB

func InitDB(connStr string) error {
	var err error
	DB, err = sql.Open("postgres", connStr)
	return err
}

func CloseDB() {
	if err := DB.Close(); err != nil {
		slog.Warn("error while closing database", "err", err)
	}
}
