package errs

import "github.com/gin-gonic/gin"

func PublicError(c *gin.Context, err error) *gin.Error {
	return c.Error(err).SetType(gin.ErrorTypePublic)
}

func PrivateError(c *gin.Context, err error) *gin.Error {
	return c.Error(err).SetType(gin.ErrorTypePrivate)
}
