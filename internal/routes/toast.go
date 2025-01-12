package routes

import (
	"html/template"
	"log/slog"

	"github.com/btmxh/plst4/internal/html"
	"github.com/btmxh/plst4/internal/middlewares"
	"github.com/gin-gonic/gin"
)

func Toast(c *gin.Context, kind html.ToastKind, title template.HTML, description template.HTML) {
	HxNoswap(c)
	if err := html.RenderToast(c.Writer, kind, title, description); err != nil {
		slog.Warn("Unable to render toast notification", "err", err)
	}
}

func ToastRouter(g *gin.RouterGroup) {
	g.GET("/error", func(c *gin.Context) {
		Toast(c, html.ToastError, "Test error message", "Hello, World!")
	})
	g.GET("/info", func(c *gin.Context) {
		Toast(c, html.ToastInfo, "Test info message", "Hello, World!")
	})
	g.GET("/error/long", func(c *gin.Context) {
		Toast(c, html.ToastError, "Test error message", "Hello, World!Hello, World!Hello, World!Hello, World!Hello, World!Hello, World!")
	})
	g.GET("/info/long", func(c *gin.Context) {
		Toast(c, html.ToastInfo, "Test info message", "Hello, World!Hello, World!Hello, World!Hello, World!Hello, World!Hello, World!")
	})
}

func ToastErrorMiddleware() gin.HandlerFunc {
	return middlewares.ErrorMiddleware(func(c *gin.Context, title, desc template.HTML) {
		Toast(c, html.ToastError, title, desc)
	})
}

func RenderErrorMiddleware() gin.HandlerFunc {
	return middlewares.ErrorMiddleware(func(c *gin.Context, title, desc template.HTML) {
		html.RenderError(c, title, desc)
	})
}
