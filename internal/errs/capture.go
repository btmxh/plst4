package errs

import "github.com/gin-gonic/gin"

type CaptureErrorHandler struct {
	Errors []gin.Error
}

func NewCapturingErrorHandler() *CaptureErrorHandler {
	return &CaptureErrorHandler{}
}

func (e *CaptureErrorHandler) RenderError(err error) {
	e.Errors = append(e.Errors, gin.Error{Err: err, Type: gin.ErrorTypeRender})
}

func (e *CaptureErrorHandler) PublicError(_ int, err error) {
	e.Errors = append(e.Errors, gin.Error{Err: err, Type: gin.ErrorTypePublic})
}

func (e *CaptureErrorHandler) PrivateError(err error) {
	e.Errors = append(e.Errors, gin.Error{Err: err, Type: gin.ErrorTypePrivate})
}
