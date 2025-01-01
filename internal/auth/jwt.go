package auth

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret string

func InitJWT() error {
	var ok bool
	jwtSecret, ok = os.LookupEnv("JWT_SECRET")
	if !ok {
		return fmt.Errorf("JWT_SECRET not specified")
	}

	return nil
}

func JwtKeyFunc(_ *jwt.Token) (interface{}, error) {
	return []byte(jwtSecret), nil
}

func Authorize(username string, timeout time.Duration) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &jwt.RegisteredClaims{
		Issuer:    "plst4-web",
		Subject:   username,
		Audience:  []string{"plst4.dev"},
		ExpiresAt: &jwt.NumericDate{Time: now.Add(timeout)},
		IssuedAt:  &jwt.NumericDate{Time: now},
	})
	return token.SignedString([]byte(jwtSecret))
}
