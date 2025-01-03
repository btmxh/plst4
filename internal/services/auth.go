package services

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"time"

	"github.com/btmxh/plst4/internal/auth"
	"github.com/btmxh/plst4/internal/db"
	"github.com/btmxh/plst4/internal/mailer"
	"github.com/dchest/uniuri"
	"golang.org/x/crypto/bcrypt"
)

func getEmailTemplate(name string) *template.Template {
	return template.Must(template.ParseFiles(fmt.Sprintf("templates/email/%s.tmpl", name), "templates/email/layout.tmpl"))
}

var confirmEmailTmpl = getEmailTemplate("confirm")
var recoverEmailTmpl = getEmailTemplate("recover")

var invalidPasswordHashError = errors.New("Invalid password. Please try another one.")
var usernameAlreadyTakenError = errors.New("Username is already taken.")
var emailAlreadyTakenError = errors.New("Email is already taken.")
var noSuchAccountError = errors.New("No such account with that email address.")
var wrongCredentialsError = errors.New("Either username or password is incorrect.")
var invalidLinkError = errors.New("This link is either invalid or expired. Please request a new one.")
var invalidCodeError = errors.New("This code is either invalid or expired. Please request a new one.")
var tokenGenerationError = errors.New("Unable to generate login token. Please try again.")

var usernameRegex = regexp.MustCompile("^[a-zA-Z0-9_-]{3,50}$")
var passwordRegex = regexp.MustCompile("^[a-zA-Z0-9_!@#$%^&*()\\-+=]{8,64}$")

func Register(tx *db.Tx, email *mail.Address, username, password string) (hasErr bool) {
	password_hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		tx.PrivateError(err)
		tx.PublicError(http.StatusUnprocessableEntity, invalidPasswordHashError)
		return true
	}

	sqlCheckFails := func(takenErr error, q string, args ...any) bool {
		var dummy int
		var hasRow bool
		if tx.QueryRow(q, args...).Scan(&hasRow, &dummy) {
			return true
		}

		if hasRow {
			tx.PublicError(http.StatusUnprocessableEntity, takenErr)
			return true
		}

		return false
	}

	if sqlCheckFails(usernameAlreadyTakenError, "SELECT 1 FROM users WHERE username = $1", username) ||
		sqlCheckFails(usernameAlreadyTakenError, "SELECT 1 FROM pending_users WHERE username = $1", username) ||
		sqlCheckFails(emailAlreadyTakenError, "SELECT 1 FROM users WHERE email = $1", email.Address) ||
		sqlCheckFails(emailAlreadyTakenError, "SELECT 1 FROM pending_users WHERE email = $1", email.Address) {
		return true
	}

	identifier := uniuri.New()
	go mailer.SendMailTemplated(email, "Confirm your plst4 email", confirmEmailTmpl, identifier)

	if tx.Exec(nil, "INSERT INTO pending_users (identifier, username, password_hashed, email) VALUES ($1, $2, $3, $4)", identifier, username, password_hashed, email.Address) {
		return true
	}

	slog.Info("User registration successful",
		slog.String("username", username),
		slog.String("email", email.Address))
	return false
}

func LogIn(tx *db.Tx, username, password string) (signedToken string, timeout time.Duration, hasErr bool) {
	timeout = 12 * time.Hour

	var hasRow bool
	var hashed string
	if tx.QueryRow("SELECT password_hashed FROM users WHERE username = $1", username).Scan(&hasRow, &hashed) {
		return signedToken, timeout, true
	}

	if !hasRow {
		tx.PublicError(http.StatusUnauthorized, wrongCredentialsError)
		return signedToken, timeout, true
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)); err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			tx.PublicError(http.StatusUnauthorized, wrongCredentialsError)
		} else {
			tx.PrivateError(err)
			tx.PublicError(http.StatusInternalServerError, tokenGenerationError)
		}
		return signedToken, timeout, true
	}

	signedToken, err := auth.Authorize(username, timeout)
	if err != nil {
		tx.PrivateError(err)
		tx.PublicError(http.StatusInternalServerError, tokenGenerationError)
		return signedToken, timeout, true
	}

	slog.Info("User authentication successful", slog.String("username", username))
	return signedToken, timeout, false
}

func SendRecoveryEmail(tx *db.Tx, email *mail.Address) (hasErr bool) {
	var dummy int
	var hasRow bool
	if tx.QueryRow("SELECT 1 FROM users WHERE email = $1", email.Address).Scan(&hasRow, &dummy) {
		return true
	}

	if !hasRow {
		tx.PublicError(http.StatusUnprocessableEntity, noSuchAccountError)
		return true
	}

	identifier := uniuri.New()
	hostname := "http://localhost:6972"
	go func() {
		err := mailer.SendMailTemplated(email, "Recover your plst4 account", recoverEmailTmpl, hostname+"/auth/resetpassword?code="+identifier+"&email="+url.QueryEscape(email.Address))
		if err != nil {
			slog.Error("Failed to send recovery email", "err", err, slog.String("email", email.Address))
		}
	}()

	if tx.Exec(nil, "INSERT INTO password_reset (email, identifier) VALUES ($1, $2) ON CONFLICT (email) DO UPDATE SET identifier = EXCLUDED.identifier", email.Address, identifier) {
		return true
	}

	return false
}

func ResetPasswordRequestValid(tx *db.Tx, identifier, email string) (hasErr bool) {
	var dummy int
	var hasRow bool
	if tx.QueryRow("SELECT 1 FROM password_reset WHERE email = $1 AND identifier = $2", email, identifier).Scan(&hasRow, &dummy) {
		return true
	}

	if !hasRow {
		tx.PublicError(http.StatusBadRequest, invalidLinkError)
		return true
	}

	return false
}

func ResetPassword(tx *db.Tx, identifier, email, password string) (hasErr bool) {
	var result sql.Result
	if tx.Exec(&result, "DELETE FROM password_reset WHERE email = $1 AND identifier = $2", email, identifier) {
		return true
	}

	if affected, err := result.RowsAffected(); affected == 0 || err != nil {
		tx.PublicError(http.StatusUnprocessableEntity, invalidLinkError)
		return true
	}

	password_hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		tx.PrivateError(err)
		tx.PublicError(http.StatusInternalServerError, invalidPasswordHashError)
		return true
	}

	return tx.Exec(nil, "UPDATE users SET password_hashed = $1 WHERE email = $2", password_hashed, email)
}

func ConfirmMail(tx *db.Tx, identifier, username string) (hasErr bool) {
	var hasRow bool
	var password_hashed string
	var email string
	if tx.QueryRow("SELECT password_hashed, email FROM pending_users WHERE username = $1 AND identifier = $2", username, identifier).Scan(&hasRow, &password_hashed, &email) {
		return true
	}

	if !hasRow {
		tx.PublicError(http.StatusUnprocessableEntity, invalidCodeError)
		return true
	}

	if tx.Exec(nil, "DELETE FROM pending_users WHERE username = $1 AND identifier = $2", username, identifier) ||
		tx.Exec(nil, "INSERT INTO users (username, password_hashed, email) VALUES ($1, $2, $3)", username, password_hashed, email) {
		return true
	}

	slog.Info("Mail confirmation successful", slog.String("username", username), slog.String("email", email))
	return false
}
