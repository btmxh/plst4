package middlewares

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const AUTH_COOKIE_NAME = "Authorization"
const AUTH_OBJECT_KEY = "auth_data"

var jwtSecret string

func InitJWT() error {
	var ok bool
	jwtSecret, ok = os.LookupEnv("JWT_SECRET")
	if !ok {
		return fmt.Errorf("JWT_SECRET not specified")
	}

	return nil
}

func AuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		tokenStr, err := ctx.Cookie(AUTH_COOKIE_NAME)
		if err == nil {
			claims := &jwt.RegisteredClaims{}

			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				return []byte(jwtSecret), nil
			})

			if err == nil && token.Valid && time.Now().Before(claims.ExpiresAt.Time) {
				ctx.Set(AUTH_OBJECT_KEY, claims.Subject)
			}
		}

		ctx.Next()
	}
}

func GetAuthUsername(c *gin.Context) (string, bool) {
	value, ok := c.Get(AUTH_OBJECT_KEY)
	if ok {
		return value.(string), ok
	}

	return "", ok
}

func Authorize(token *jwt.Token) (string, error) {
	return token.SignedString([]byte(jwtSecret))
}

func SetAuthCookie(c *gin.Context, signedToken string, timeout time.Duration) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(AUTH_COOKIE_NAME, signedToken, int(timeout.Seconds()), "", "", true, true)
}

func Logout(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(AUTH_COOKIE_NAME, "", -1, "/", "", false, true)
}
