package errs

import (
	"html/template"
	"net/http"

	"github.com/btmxh/plst4/internal/stores"
	"github.com/gin-gonic/gin"
)

type GinErrorHandler struct {
	htmx    bool
	context *gin.Context
}

func NewGinErrorHandler(c *gin.Context, title template.HTML) *GinErrorHandler {
	stores.SetErrorTitle(c, title)
	return &GinErrorHandler{htmx: c.Request.Header.Get("HX-Request") == "true", context: c}
}

func (e *GinErrorHandler) RenderError(err error) {
	e.context.Error(err).SetType(gin.ErrorTypeRender)
}

func (e *GinErrorHandler) PublicError(statusCode int, err error) {
	if e.htmx {
		statusCode = http.StatusOK
	}
	e.context.Status(statusCode)
	e.context.Error(err).SetType(gin.ErrorTypePublic)
}

func (e *GinErrorHandler) PrivateError(err error) {
	e.context.Error(err).SetType(gin.ErrorTypePrivate)
}
