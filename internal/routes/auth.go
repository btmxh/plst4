package routes

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/btmxh/plst4/internal/auth"
	"github.com/btmxh/plst4/internal/db"
	"github.com/btmxh/plst4/internal/errs"
	"github.com/btmxh/plst4/internal/html"
	"github.com/btmxh/plst4/internal/mailer"
	"github.com/btmxh/plst4/internal/middlewares"
	"github.com/dchest/uniuri"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func getAuthTemplate(name string) *template.Template {
	return getTemplate(name, fmt.Sprintf("templates/auth/%s.tmpl", name), "templates/auth/common.tmpl")
}

func getEmailTemplate(name string) *template.Template {
	return template.Must(template.ParseFiles(fmt.Sprintf("templates/email/%s.tmpl", name), "templates/email/layout.tmpl"))
}

func defaultErrorMsg(msg template.HTML) gin.H {
	if len(msg) == 0 {
		msg = "Internal server error. Please try again later."
	}

	return gin.H{
		"Error": msg,
	}
}

func redirectIfLoggedIn(c *gin.Context) bool {
	if auth.IsLoggedIn(c) {
		if c.Request.Method == "POST" {
			c.Header("Hx-Redirect", "/")
		} else {
			c.Redirect(http.StatusTemporaryRedirect, "/")
		}

		return true
	}

	return false
}

func authRender(tmpl *template.Template, c *gin.Context, block string, arg gin.H) {
	if !redirectIfLoggedIn(c) {
		html.Render(tmpl, c, block, arg)
	}
}

func authSSRRoute(tmpl *template.Template, block string, arg gin.H) gin.HandlerFunc {
	return func(c *gin.Context) {
		authRender(tmpl, c, block, arg)
	}
}

var confirmMailTmpl = getAuthTemplate("confirmmail")
var registerTmpl = getAuthTemplate("register")
var loginTmpl = getAuthTemplate("login")
var recoverTmpl = getAuthTemplate("recover")
var recoverDoneTmpl = getAuthTemplate("recoverdone")
var newPasswordTmpl = getAuthTemplate("resetpassword")
var newPasswordInvalidTmpl = getAuthTemplate("resetpassword_invalid")
var confirmEmailTmpl = getEmailTemplate("confirm")
var recoverEmailTmpl = getEmailTemplate("recover")

func AuthRouter(r *gin.RouterGroup) http.Handler {
	handler := http.NewServeMux()
	r.POST("/register/submit", register)
	r.GET("/register/form", authSSRRoute(registerTmpl, "form", gin.H{}))
	r.GET("/register", authSSRRoute(registerTmpl, "layout", gin.H{}))

	r.POST("/login/submit", login)
	r.GET("/login/form", authSSRRoute(loginTmpl, "form", gin.H{}))
	r.GET("/login", authSSRRoute(loginTmpl, "layout", gin.H{}))

	r.POST("/recover/submit", recoverFunc)
	r.GET("/recover/form", authSSRRoute(recoverTmpl, "form", gin.H{}))
	r.GET("/recover", authSSRRoute(recoverTmpl, "layout", gin.H{}))

	r.POST("/confirmmail/submit", confirmMail)
	r.GET("/confirmmail/form", authSSRRoute(confirmMailTmpl, "form", gin.H{}))
	r.GET("/confirmmail", authSSRRoute(confirmMailTmpl, "layout", gin.H{}))

	r.GET("/recoverdone/form", authSSRRoute(recoverDoneTmpl, "form", gin.H{}))
	r.GET("/recoverdone", authSSRRoute(recoverDoneTmpl, "layout", gin.H{}))

	r.POST("/logout", logout)

	r.GET("/resetpassword", resetPassword)
	r.POST("/resetpassword/submit", resetPasswordSubmit)
	return handler
}

var invalidUsernameError = errors.New("Username must be between 3 and 50 characters long and can only contain letters, numbers, hyphens (-), and underscores (_).")
var emptyUsernameError = errors.New("Username must not be empty.")
var usernameRegex = regexp.MustCompile("^[a-zA-Z0-9_-]{3,50}$")
var invalidPasswordError = errors.New("Password must be at least 8 characters long and at most 64 characters long.")
var emptyPasswordError = errors.New("Password must not be empty.")
var invalidPasswordHashError = errors.New("Invalid password. Please try another one.")
var passwordRegex = regexp.MustCompile("[^a-zA-Z0-9_!@#$%^&*()-+=]{8,64}")
var passwordNotMatchError = errors.New("Passwords do not match.")
var invalidEmailError = errors.New("Invalid email.")
var usernameAlreadyTakenError = errors.New("Username is already taken.")
var emailAlreadyTakenError = errors.New("Email is already taken.")
var noSuchAccountError = errors.New("No such account with that email address.")
var wrongCredentialsError = errors.New("Either username or password is incorrect.")
var invalidLinkError = errors.New("This link is either invalid or expired. Please request a new one.")
var invalidCodeError = errors.New("This code is either invalid or expired. Please request a new one.")

func register(c *gin.Context) {
	if redirectIfLoggedIn(c) {
		return
	}

	username := strings.TrimSpace(c.PostForm("username"))
	// TODO: move compile out
	if !usernameRegex.MatchString(username) {
		errs.PublicError(c, invalidUsernameError)
		return
	}

	password := c.PostForm("password")
	if passwordRegex.MatchString(password) {
		errs.PublicError(c, invalidPasswordError)
		return
	}

	if password != c.PostForm("password-confirm") {
		errs.PublicError(c, passwordNotMatchError)
		return
	}

	password_hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		errs.PrivateError(c, err)
		errs.PublicError(c, invalidPasswordHashError)
		return
	}

	email, err := mail.ParseAddress(c.PostForm("email"))
	if err != nil {
		errs.PrivateError(c, err)
		errs.PublicError(c, invalidEmailError)
		return
	}

	tx := db.BeginTx(c)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	sqlCheckFails := func(takenErr error, q string, args ...any) bool {
		row := tx.QueryRow(q, args...)
		var dummy int
		var hasRow bool
		if row.Scan(&hasRow, &dummy) {
			return true
		}

		if hasRow {
			errs.PublicError(c, takenErr)
			return true
		}

		return false
	}

	if sqlCheckFails(usernameAlreadyTakenError, "SELECT 1 FROM users WHERE username = $1", username) {
		return
	}
	if sqlCheckFails(usernameAlreadyTakenError, "SELECT 1 FROM pending_users WHERE username = $1", username) {
		return
	}
	if sqlCheckFails(emailAlreadyTakenError, "SELECT 1 FROM users WHERE email = $1", email.Address) {
		return
	}
	if sqlCheckFails(emailAlreadyTakenError, "SELECT 1 FROM pending_users WHERE email = $1", email.Address) {
		return
	}

	identifier := uniuri.New()
	go mailer.SendMailTemplated(email, "Confirm your plst4 email", confirmEmailTmpl, identifier)

	if tx.Exec(nil, "INSERT INTO pending_users (identifier, username, password_hashed, email) VALUES ($1, $2, $3, $4)", identifier, username, password_hashed, email.Address) {
		return
	}

	if tx.Commit() {
		return
	}

	HxPushURL(c, "/auth/confirmmail?username="+url.QueryEscape(username))
	html.Render(confirmMailTmpl, c, "form", gin.H{"FormUsername": username})
}

func login(c *gin.Context) {
	if redirectIfLoggedIn(c) {
		return
	}

	username := strings.TrimSpace(c.PostForm("username"))
	password := c.PostForm("password")

	if len(username) == 0 {
		errs.PublicError(c, emptyUsernameError)
		return
	}
	if len(password) == 0 {
		errs.PublicError(c, emptyPasswordError)
		return
	}

	tx := db.BeginTx(c)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	var hasRow bool
	var hashed string
	if tx.QueryRow("SELECT password_hashed FROM users WHERE username = $1", username).Scan(&hasRow, &hashed) {
		return
	}

	if !hasRow {
		errs.PublicError(c, wrongCredentialsError)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)); err != nil {
		errs.PrivateError(c, err)
		errs.PublicError(c, wrongCredentialsError)
		return
	}

	timeout := 12 * time.Hour
	signedToken, err := auth.Authorize(username, timeout)
	if err != nil {
		errs.PrivateError(c, err)
		errs.PublicError(c, errors.New("Unable to generate login token. Please try again"))
		return
	}

	if tx.Commit() {
		return
	}

	middlewares.SetAuthCookie(c, signedToken, timeout)
	HxRedirect(c, "/")
}

func recoverFunc(c *gin.Context) {
	if redirectIfLoggedIn(c) {
		return
	}

	email, err := mail.ParseAddress(c.PostForm("email"))
	if err != nil {
		errs.PrivateError(c, err)
		errs.PublicError(c, invalidEmailError)
		return
	}

	tx := db.BeginTx(c)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	var dummy int
	var hasRow bool
	if tx.QueryRow("SELECT 1 FROM users WHERE email = $1", email.Address).Scan(&hasRow, &dummy) {
		return
	}

	if err == sql.ErrNoRows {
		errs.PublicError(c, noSuchAccountError)
	}

	identifier := uniuri.New()
	hostname := "http://localhost:6972"
	go mailer.SendMailTemplated(email, "Recover your plst4 account", recoverEmailTmpl, hostname+"/auth/resetpassword?code="+identifier+"&email="+url.QueryEscape(email.Address))

	if tx.Exec(nil, "INSERT INTO password_reset (email, identifier) VALUES ($1, $2) ON CONFLICT (email) DO UPDATE SET identifier = EXCLUDED.identifier", email.Address, identifier) {
		return
	}

	if tx.Commit() {
		return
	}

	html.Render(recoverDoneTmpl, c, "form", gin.H{})
}

func resetPassword(c *gin.Context) {
	email := c.Query("email")
	identifier := c.Query("code")

	tx := db.BeginTx(c)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	var dummy int
	var hasRow bool
	if tx.QueryRow("SELECT 1 FROM password_reset WHERE email = $1 AND identifier = $2", email, identifier).Scan(&hasRow, &dummy) {
		return
	}

	if !hasRow {
		errs.PublicError(c, invalidLinkError)
		return
	}

	if tx.Commit() {
		return
	}

	html.Render(newPasswordTmpl, c, "layout", gin.H{"Identifier": identifier, "Email": email})
}

func resetPasswordSubmit(c *gin.Context) {
	email := c.PostForm("email")
	identifier := c.PostForm("code")
	password := c.PostForm("password")
	passwordConfirm := c.PostForm("password-confirm")

	if password != passwordConfirm {
		errs.PublicError(c, passwordNotMatchError)
		return
	}

	if passwordRegex.MatchString(password) {
		errs.PublicError(c, invalidPasswordError)
		return
	}

	tx := db.BeginTx(c)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	var result sql.Result
	if tx.Exec(&result, "DELETE FROM password_reset WHERE email = $1 AND identifier = $2", email, identifier) {
		return
	}

	if affected, err := result.RowsAffected(); affected == 0 || err != nil {
		errs.PublicError(c, invalidLinkError)
		return
	}

	password_hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		errs.PrivateError(c, err)
		errs.PublicError(c, invalidPasswordHashError)
		return
	}

	if tx.Exec(nil, "UPDATE users SET password_hashed = $1 WHERE email = $2", password_hashed, email) {
		return
	}

	if tx.Commit() {
		return
	}

	var msg template.HTML = "Password reset successfully.<br>Please log in with your new password."
	html.Render(loginTmpl, c, "form", gin.H{"MessageString": &msg})
}

func confirmMail(c *gin.Context) {
	if redirectIfLoggedIn(c) {
		return
	}

	code := c.PostForm("code")
	username := c.PostForm("username")

	tx := db.BeginTx(c)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	var hasRow bool
	var password_hashed string
	var email string
	if tx.QueryRow("SELECT password_hashed, email FROM pending_users WHERE username = $1 AND identifier = $2", username, code).Scan(&hasRow, &password_hashed, &email) {
		return
	}

	if !hasRow {
		errs.PublicError(c, invalidCodeError)
		return
	}

	if tx.Exec(nil, "DELETE FROM pending_users WHERE username = $1 AND identifier = $2", username, code) {
		return
	}

	if tx.Exec(nil, "INSERT INTO users (username, password_hashed, email) VALUES ($1, $2, $3)", username, password_hashed, email) {
		return
	}

	if tx.Commit() {
		return
	}

	var msg template.HTML = "Email confirmed successfully.<br>Please log in with your credentials."
	html.Render(loginTmpl, c, "form", gin.H{"MessageString": &msg})
}

func logout(c *gin.Context) {
	middlewares.Logout(c)
	HxRedirect(c, "/")
}
