package routes

import (
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strings"

	"github.com/btmxh/plst4/internal/db"
	"github.com/btmxh/plst4/internal/errs"
	"github.com/btmxh/plst4/internal/html"
	"github.com/btmxh/plst4/internal/middlewares"
	"github.com/btmxh/plst4/internal/services"
	"github.com/btmxh/plst4/internal/stores"
	"github.com/gin-gonic/gin"
)

func redirectIfLoggedIn(c *gin.Context) bool {
	if stores.IsLoggedIn(c) {
		if c.Request.Method == "POST" {
			c.Header("Hx-Redirect", "/")
		} else {
			c.Redirect(http.StatusTemporaryRedirect, "/")
		}

		return true
	}

	return false
}

func getAuthTemplate(name string) *template.Template {
	return getTemplate(name, fmt.Sprintf("templates/auth/%s.tmpl", name), "templates/auth/common.tmpl")
}

var confirmMailTmpl = getAuthTemplate("confirmmail")
var registerTmpl = getAuthTemplate("register")
var loginTmpl = getAuthTemplate("login")
var recoverTmpl = getAuthTemplate("recover")
var recoverDoneTmpl = getAuthTemplate("recoverdone")
var newPasswordTmpl = getAuthTemplate("resetpassword")
var newPasswordInvalidTmpl = getAuthTemplate("resetpassword_invalid")

func authRender(tmpl *template.Template, c *gin.Context, block string, arg gin.H) {
	if !redirectIfLoggedIn(c) {
		html.RenderGin(tmpl, c, block, arg)
	}
}

func authSSRRoute(tmpl *template.Template, block string, arg gin.H) gin.HandlerFunc {
	return func(c *gin.Context) {
		authRender(tmpl, c, block, arg)
	}
}

func AuthRouter(r *gin.RouterGroup) http.Handler {
	handler := http.NewServeMux()

	get := r.Group("")
	get.Use(middlewares.ErrorMiddleware(func(c *gin.Context, title, desc template.HTML) {
		html.RenderError(c, title, desc)
	}))

	get.GET("/register", authSSRRoute(registerTmpl, "layout", gin.H{}))
	get.GET("/login", authSSRRoute(loginTmpl, "layout", gin.H{}))
	get.GET("/recover", authSSRRoute(recoverTmpl, "layout", gin.H{}))
	get.GET("/confirmmail", authSSRRoute(confirmMailTmpl, "layout", gin.H{}))
	get.GET("/recoverdone", authSSRRoute(recoverDoneTmpl, "layout", gin.H{}))
	get.GET("/resetpassword", resetPassword)

	post := r.Group("")
	post.Use(middlewares.ErrorMiddleware(func(c *gin.Context, title, desc template.HTML) {
		Toast(c, html.ToastError, title, desc)
	}))
	post.POST("/register/submit", register)
	post.POST("/login/submit", login)
	post.POST("/recover/submit", recoverFunc)
	post.POST("/confirmmail/submit", confirmMail)
	post.POST("/logout", logout)
	post.POST("/resetpassword/submit", resetPasswordSubmit)
	return handler
}

var invalidUsernameError = errors.New("Username must contain 3-50 characters, including lowercase letters, uppercase letters, numbers, hyphens (-), and underscores (_).")
var emptyUsernameError = errors.New("Username must not be empty.")
var usernameRegex = regexp.MustCompile("^[a-zA-Z0-9_-]{3,50}$")
var invalidPasswordError = errors.New("Password must contain 8-64	characters, including lowercase letters, uppercase letters, numbers and special characters.")
var emptyPasswordError = errors.New("Password must not be empty.")
var passwordRegex = regexp.MustCompile("^[a-zA-Z0-9_!@#$%^&*()\\-+=]{8,64}$")
var passwordNotMatchError = errors.New("Passwords do not match.")
var invalidEmailError = errors.New("Invalid email.")
var longEmailError = errors.New("Email must not be longer than 100 characters.")
var invalidLinkError = errors.New("This link is either invalid or expired. Please request a new one.")
var invalidCodeError = errors.New("This code is either invalid or expired. Please request a new one.")

func register(c *gin.Context) {
	if redirectIfLoggedIn(c) {
		return
	}

	handler := errs.NewGinErrorHandler(c, "Register error")

	username := strings.TrimSpace(c.PostForm("username"))
	if !usernameRegex.MatchString(username) {
		handler.PublicError(http.StatusUnprocessableEntity, invalidUsernameError)
		return
	}

	password := c.PostForm("password")
	if !passwordRegex.MatchString(password) {
		handler.PublicError(http.StatusUnprocessableEntity, invalidPasswordError)
		return
	}

	if password != c.PostForm("password-confirm") {
		handler.PublicError(http.StatusUnprocessableEntity, passwordNotMatchError)
		return
	}

	email, err := mail.ParseAddress(c.PostForm("email"))
	if err != nil {
		handler.PrivateError(err)
		handler.PublicError(http.StatusUnprocessableEntity, invalidEmailError)
		return
	}

	if len(email.Address) > 100 {
		handler.PublicError(http.StatusUnprocessableEntity, longEmailError)
		return
	}

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	slog.Info("Registering user",
		slog.String("username", username),
		slog.String("email", email.Address))
	if services.Register(tx, email, username, password) {
		return
	}

	if tx.Commit() {
		return
	}

	HxPushURL(c, "/auth/confirmmail?username="+url.QueryEscape(username))
	html.RenderGin(confirmMailTmpl, c, "form", gin.H{"FormUsername": username})
}

func login(c *gin.Context) {
	if redirectIfLoggedIn(c) {
		return
	}

	handler := errs.NewGinErrorHandler(c, "Login error")

	username := strings.TrimSpace(c.PostForm("username"))
	if len(username) == 0 {
		handler.PublicError(http.StatusUnprocessableEntity, emptyUsernameError)
		return
	}

	password := c.PostForm("password")
	if len(password) == 0 {
		handler.PublicError(http.StatusUnprocessableEntity, emptyPasswordError)
		return
	}

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	slog.Info("Logging in user", slog.String("username", username))
	signedToken, timeout, hasErr := services.LogIn(tx, username, password)
	if hasErr {
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

	handler := errs.NewGinErrorHandler(c, "Recover error")

	email, err := mail.ParseAddress(c.PostForm("email"))
	if err != nil {
		handler.PrivateError(err)
		handler.PublicError(http.StatusUnprocessableEntity, invalidEmailError)
		return
	}

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	slog.Info("Sending recovery email", slog.String("email", email.Address))
	if services.SendRecoveryEmail(tx, email) {
		return
	}

	if tx.Commit() {
		return
	}

	html.RenderGin(recoverDoneTmpl, c, "form", gin.H{})
}

func resetPassword(c *gin.Context) {
	handler := errs.NewGinErrorHandler(c, "Reset password error")
	email := c.Query("email")
	identifier := c.Query("code")

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	slog.Info("Resetting password", slog.String("email", email), slog.String("code", identifier))
	if services.ResetPasswordRequestValid(tx, identifier, email) {
		return
	}

	if tx.Commit() {
		return
	}

	html.RenderGin(newPasswordTmpl, c, "layout", gin.H{"Identifier": identifier, "Email": email})
}

func resetPasswordSubmit(c *gin.Context) {
	handler := errs.NewGinErrorHandler(c, "Reset password error")
	email := c.PostForm("email")
	identifier := c.PostForm("code")
	password := c.PostForm("password")
	passwordConfirm := c.PostForm("password-confirm")

	if !passwordRegex.MatchString(password) {
		handler.PublicError(http.StatusUnprocessableEntity, invalidPasswordError)
		return
	}

	if password != passwordConfirm {
		handler.PrivateError(fmt.Errorf("Password doesn't match: %s != %s", password, passwordConfirm))
		handler.PublicError(http.StatusUnprocessableEntity, passwordNotMatchError)
		return
	}

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	slog.Info("Resetting password", slog.String("email", email), slog.String("code", identifier))
	if services.ResetPassword(tx, identifier, email, password) {
		return
	}

	if tx.Commit() {
		return
	}

	var msg template.HTML = "Password reset successfully.<br>Please log in with your new password."
	html.RenderGin(loginTmpl, c, "form", gin.H{"MessageString": &msg})
}

func confirmMail(c *gin.Context) {
	if redirectIfLoggedIn(c) {
		return
	}

	handler := errs.NewGinErrorHandler(c, "Mail confirmation error")
	identifier := c.PostForm("code")
	username := c.PostForm("username")

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	slog.Info("Confirming mail", slog.String("code", identifier), slog.String("username", username))
	if services.ConfirmMail(tx, identifier, username) {
		return
	}

	if tx.Commit() {
		return
	}

	var msg template.HTML = "Email confirmed successfully.<br>Please log in with your credentials."
	html.RenderGin(loginTmpl, c, "form", gin.H{"MessageString": &msg})
}

func logout(c *gin.Context) {
	middlewares.Logout(c)
	HxRedirect(c, "/")
}
