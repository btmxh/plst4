package auth

import "github.com/gin-gonic/gin"

const AUTH_OBJECT_KEY = "auth_data"

func SetUsername(c *gin.Context, username string) {
	c.Set(AUTH_OBJECT_KEY, username)
}

func GetUsername(c *gin.Context) string {
	value, ok := c.Get(AUTH_OBJECT_KEY)
	if ok {
		return value.(string)
	}

	return ""
}

func IsLoggedIn(c *gin.Context) bool {
	return GetUsername(c) != ""
}
