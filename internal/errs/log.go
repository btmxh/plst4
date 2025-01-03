package errs

import "log/slog"

type LogErrorHandler struct {
	title              string
	publicErrorHandler func(err error) error
}

func NewLogErrorHandler(title string, publicErrorHandler func(err error) error) *LogErrorHandler {
	return &LogErrorHandler{title: title, publicErrorHandler: publicErrorHandler}
}

func (e *LogErrorHandler) RenderError(err error) {
	slog.Warn("Render error", "title", e.title, "err", err)
}

func (e *LogErrorHandler) PublicError(_ int, err error) {
	slog.Warn("Public error", "title", e.title, "err", err)
	if handleErr := e.publicErrorHandler(err); handleErr != nil {
		slog.Warn("Error handling public error while "+e.title, "err", err, "handleErr", handleErr)
	}
}

func (e *LogErrorHandler) PrivateError(err error) {
	slog.Warn("Private error", "title", e.title, "err", err)
}
