package middlewares

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/btmxh/plst4/internal/auth"
	"github.com/btmxh/plst4/internal/errs"
	"github.com/btmxh/plst4/internal/stores"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const AUTH_COOKIE_NAME = "Authorization"

var unauthorizedError = errors.New("You must be logged in to do this.")

func AuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		tokenStr, err := ctx.Cookie(AUTH_COOKIE_NAME)
		if err == nil {
			claims := &jwt.RegisteredClaims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, auth.JwtKeyFunc)

			if err == nil && token.Valid {
				if time.Now().After(claims.ExpiresAt.Time) {
					slog.Warn("Token expired", "username", claims.Subject)
				} else {
					stores.SetUsername(ctx, claims.Subject)
				}
			} else {
				slog.Warn("Failed to validate token", "token", token, "error", err)
			}
		} else if err != http.ErrNoCookie {
			slog.Warn("Failed to get auth cookie", "error", err)
		}

		ctx.Next()
	}
}

func SetAuthCookie(c *gin.Context, signedToken string, timeout time.Duration) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(AUTH_COOKIE_NAME, signedToken, int(timeout.Seconds()), "", "", true, true)
}

func Logout(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(AUTH_COOKIE_NAME, "", -1, "/", "", false, true)
}

func MustAuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		handler := errs.NewGinErrorHandler(ctx, "Error")
		if !stores.IsLoggedIn(ctx) {
			handler.PublicError(http.StatusUnauthorized, unauthorizedError)
			return
		}

		ctx.Next()
	}
}
