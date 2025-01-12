package errs

type ErrorHandler interface {
	RenderError(err error)
	PublicError(statusCode int, err error)
	PrivateError(err error)
}
