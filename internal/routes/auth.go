package routes

import (
	"database/sql"
	"fmt"
	"html/template"
	"log/slog"
	_ "log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/btmxh/plst4/internal/db"
	"github.com/btmxh/plst4/internal/mailer"
	"github.com/btmxh/plst4/internal/middlewares"
	"github.com/dchest/uniuri"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/gomail.v2"
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
	_, loggedIn := middlewares.GetAuthUsername(c)
	if loggedIn {
		if c.Request.Method == "POST" {
			c.Header("Hx-Redirect", "/")
		} else {
			c.Redirect(http.StatusTemporaryRedirect, "/")
		}
	}
	return loggedIn
}

func authSSR(tmpl *template.Template, c *gin.Context, block string, arg gin.H) {
	if !redirectIfLoggedIn(c) {
		SSR(tmpl, c, block, arg)
	}
}

func authSSRRoute(tmpl *template.Template, block string, arg gin.H) gin.HandlerFunc {
	return func(c *gin.Context) {
		authSSR(tmpl, c, block, arg)
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

func sendMail(dest *mail.Address, tmpl *template.Template, data any) error {
	m := gomail.NewMessage()
	m.SetHeader("From", mailer.Dealer.Username)
	m.SetHeader("To", dest.Address)
	var writer strings.Builder
	err := tmpl.ExecuteTemplate(&writer, "layout", data)
	if err != nil {
		return err
	}
	m.SetBody("text/html", writer.String())
	err = mailer.Dealer.DialAndSend(m)
	if err != nil {
		return err
	}

	return nil
}

func register(c *gin.Context) {
	fail := func(msg template.HTML, err error) {
		slog.Warn("Register error", "msg", msg, "err", err)
		SSR(registerTmpl, c, "form", defaultErrorMsg(msg))
	}

	_, loggedIn := middlewares.GetAuthUsername(c)
	if loggedIn {
		fail("Already logged in, please sign out before proceeding.", nil)
		return
	}

	username := strings.TrimSpace(c.PostForm("username"))
	// TODO: move compile out
	if !regexp.MustCompile("^[a-zA-Z0-9_-]{3,50}$").MatchString(username) {
		fail("Username must be between 3 and 50 characters long and can only contain letters, numbers, hyphens (-), and underscores (_).", nil)
		return
	}

	password := c.PostForm("password")
	if len(password) < 8 {
		fail("Password must be at least 8 characters long", nil)
		return
	}

	password_hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fail("", err)
		return
	}

	if password != c.PostForm("password-confirm") {
		fail("Password does not match.", nil)
		return
	}

	email, err := mail.ParseAddress(c.PostForm("email"))
	if err != nil {
		fail("Invalid email", err)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	sqlCheck := func(msg template.HTML, q string, args ...any) bool {
		row := tx.QueryRow(q, args...)
		var dummy int
		err = row.Scan(&dummy)
		if err == sql.ErrNoRows {
			return true
		} else if err != nil {
			fail("", err)
			return false
		} else {
			fail(msg, err)
			return false
		}
	}

	if !sqlCheck("Username is already taken", "SELECT 1 FROM users WHERE username = $1", username) {
		return
	}
	if !sqlCheck("Username is already taken", "SELECT 1 FROM pending_users WHERE username = $1", username) {
		return
	}
	if !sqlCheck("Email is already taken", "SELECT 1 FROM users WHERE email = $1", email.Address) {
		return
	}
	if !sqlCheck("Email is already taken", "SELECT 1 FROM pending_users WHERE email = $1", email.Address) {
		return
	}

	identifier := uniuri.New()
	go sendMail(email, confirmEmailTmpl, identifier)

	_, err = tx.Exec("INSERT INTO pending_users (identifier, username, password_hashed, email) VALUES ($1, $2, $3, $4)", identifier, username, password_hashed, email.Address)
	if err != nil {
		fail("", err)
		return
	}

	err = tx.Commit()
	if err != nil {
		fail("", err)
		return
	}

	PushURL(c, "/auth/confirmmail?username="+url.QueryEscape(username))
	SSR(confirmMailTmpl, c, "form", gin.H{"FormUsername": username})
}

func login(c *gin.Context) {
	fail := func(msg template.HTML, err error) {
		slog.Warn("Login error", "msg", msg, "err", err)
		SSR(loginTmpl, c, "form", defaultErrorMsg(msg))
	}

	_, loggedIn := middlewares.GetAuthUsername(c)
	if loggedIn {
		fail("Already logged in, please sign out before proceeding.", nil)
		return
	}

	username := strings.TrimSpace(c.PostForm("username"))
	password := c.PostForm("password")

	if len(username) == 0 {
		fail("Username can't be empty", nil)
		return
	}
	if len(password) == 0 {
		fail("Username can't be empty", nil)
		return
	}

	row := db.DB.QueryRow("SELECT password_hashed FROM users WHERE username = $1", username)

	var hashed string
	err := row.Scan(&hashed)

	if err == nil {
		if err = bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)); err != nil {
			fail("Wrong password.", err)
			return
		}
	} else if err == sql.ErrNoRows {
		fail("No such account exists.", err)
		return
	} else {
		fail("", err)
		return
	}

	now := time.Now()
	timeout := 12 * time.Hour
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &jwt.RegisteredClaims{
		Issuer:    "plst4-web",
		Subject:   username,
		Audience:  []string{"plst4.dev"},
		ExpiresAt: &jwt.NumericDate{Time: now.Add(timeout)},
		IssuedAt:  &jwt.NumericDate{Time: now},
	})
	signedToken, err := middlewares.Authorize(token)
	if err != nil {
		fail("Unable to generate login token. Please try again", err)
		return
	}

	middlewares.SetAuthCookie(c, signedToken, timeout)
	Redirect(c, "/")
}

func recoverFunc(c *gin.Context) {
	fail := func(msg template.HTML, err error) {
		slog.Warn("Login error", "msg", msg, "err", err)
		SSR(recoverTmpl, c, "form", defaultErrorMsg(msg))
	}

	_, loggedIn := middlewares.GetAuthUsername(c)
	if loggedIn {
		fail("Already logged in, please sign out before proceeding.", nil)
		return
	}

	email, err := mail.ParseAddress(c.PostForm("email"))
	if err != nil {
		fail("Invalid email", err)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}

	defer tx.Rollback()

	row := tx.QueryRow("SELECT 1 FROM users WHERE email = $1", email.Address)
	var dummy int
	err = row.Scan(&dummy)
	if err == sql.ErrNoRows {
		fail("No such account with that email address", err)
		return
	} else if err != nil {
		fail("", err)
		return
	}

	identifier := uniuri.New()
	go sendMail(email, recoverEmailTmpl, "http://localhost:6972/auth/resetpassword?code="+identifier+"&email="+url.QueryEscape(email.Address))

	_, err = tx.Exec("INSERT INTO password_reset (email, identifier) VALUES ($1, $2) ON CONFLICT (email) DO UPDATE SET identifier = EXCLUDED.identifier", email.Address, identifier)
	if err != nil {
		fail("", err)
		return
	}

	err = tx.Commit()
	if err != nil {
		fail("", err)
		return
	}

	SSR(recoverDoneTmpl, c, "form", gin.H{})
}

func resetPassword(c *gin.Context) {
	email := c.Query("email")
	identifier := c.Query("code")

	fail := func(msg template.HTML, err error) {
		slog.Warn("Confirming email error", "msg", msg, "err", err)
		SSR(newPasswordInvalidTmpl, c, "layout", Combine(defaultErrorMsg(msg)))
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
	}
	defer tx.Rollback()

	row := tx.QueryRow("SELECT 1 FROM password_reset WHERE email = $1 AND identifier = $2", email, identifier)
	slog.Info("email", "email", email, "id", identifier)
	var dummy int
	err = row.Scan(&dummy)
	if err == sql.ErrNoRows {
		fail("This link is invalid or expired. Please request a new one.", err)
		return
	} else if err != nil {
		fail("", err)
		return
	}

	err = tx.Commit()
	if err != nil {
		fail("", err)
	}

	SSR(newPasswordTmpl, c, "layout", gin.H{"Identifier": identifier, "Email": email})
}

func resetPasswordSubmit(c *gin.Context) {
	email := c.PostForm("email")
	identifier := c.PostForm("code")
	password := c.PostForm("password")
	passwordConfirm := c.PostForm("password-confirm")

	fail := func(msg template.HTML, err error) {
		slog.Warn("Confirming email error", "msg", msg, "err", err)
		SSR(newPasswordTmpl, c, "form", Combine(defaultErrorMsg(msg)))
	}

	if password != passwordConfirm {
		fail("Mismatched passwords", nil)
		return
	}

	if len(password) < 8 {
		fail("Password must be at least 8 characters long", nil)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}

	defer tx.Rollback()

	result, err := tx.Exec("DELETE FROM password_reset WHERE email = $1 AND identifier = $2", email, identifier)
	if err != nil {
		fail("", err)
		return
	}

	if affected, err := result.RowsAffected(); affected == 0 || err != nil {
		fail("This link is invalid or expired. Please request a new one.", err)
		return
	}

	password_hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fail("", err)
		return
	}

	_, err = tx.Exec("UPDATE users SET password_hashed = $1 WHERE email = $2", password_hashed, email)
	if err != nil {
		fail("", err)
		return
	}

	err = tx.Commit()
	if err != nil {
		fail("", err)
		return
	}

	var msg template.HTML = "Password reset successfully.<br>Please log in with your new password."
	SSR(loginTmpl, c, "form", gin.H{"MessageString": &msg})
}

func confirmMail(c *gin.Context) {
	code := c.PostForm("code")
	username := c.PostForm("username")

	fail := func(msg template.HTML, err error) {
		slog.Warn("Confirming email error", "msg", msg, "err", err)
		SSR(confirmMailTmpl, c, "form", Combine(defaultErrorMsg(msg), gin.H{"FormUsername": username}))
	}

	_, loggedIn := middlewares.GetAuthUsername(c)
	if loggedIn {
		fail("Already logged in, please sign out before proceeding.", nil)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}

	defer tx.Rollback()

	row := tx.QueryRow("SELECT password_hashed, email FROM pending_users WHERE username = $1 AND identifier = $2", username, code)
	var password_hashed string
	var email string
	err = row.Scan(&password_hashed, &email)
	if err != nil {
		fail("Please recheck your code.", err)
		return
	}

	result, err := tx.Exec("DELETE FROM pending_users WHERE username = $1 AND identifier = $2", username, code)
	if err != nil {
		fail("", err)
		return
	}

	if affected, err := result.RowsAffected(); affected == 0 || err != nil {
		fail("Invalid email confirmation state.", err)
		return
	}

	_, err = tx.Exec("INSERT INTO users (username, password_hashed, email) VALUES ($1, $2, $3)", username, password_hashed, email)
	if err != nil {
		fail("", err)
		return
	}

	err = tx.Commit()
	if err != nil {
		fail("", err)
		return
	}

	var msg template.HTML = "Email confirmed successfully.<br>Please log in with your credentials."
	SSR(loginTmpl, c, "form", gin.H{"MessageString": &msg})
}

func logout(c *gin.Context) {
	middlewares.Logout(c)
	Redirect(c, "/")
}
