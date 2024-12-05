package middlewares

import (
	"log/slog"
	"net/http"
	"time"
)

type LogMiddleware struct {
	handler http.Handler
}

func NewLogMiddleware(handler http.Handler) LogMiddleware {
	return LogMiddleware{handler}
}

func (self LogMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slog.Info("Handling request",
		"method", r.Method,
		"path", r.URL.Path,
	)
	start := time.Now()
	self.handler.ServeHTTP(w, r)
	elapsed := time.Since(start)
	slog.Info("Finish handling request", "method", r.Method, "path", r.URL.Path, "time", elapsed)
}

