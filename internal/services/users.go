package services

import "github.com/btmxh/plst4/internal/db"

func CheckUserExists(tx *db.Tx, username string) (hasRow bool, hasErr bool) {
  var dummy int
	hasErr = tx.QueryRow("SELECT 1 FROM users WHERE username = $1", username).Scan(&hasRow, &dummy)
	return hasRow, hasErr
}
