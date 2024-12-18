package routes

import (
	"html/template"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
)

var toastTemplate = template.Must(template.ParseFiles("templates/notifications/toast.tmpl"))

type ToastKind string

const (
	ToastError ToastKind = "error"
	ToastInfo  ToastKind = "info"
)

func Toast(c *gin.Context, kind ToastKind, title template.HTML, description template.HTML) {
	// c.Header("Hx-Reswap", "afterbegin")
	// c.Header("Hx-Retarget", ".toast-notification-box")
	SSR(toastTemplate, c, "content", gin.H{
		"Title":       title,
		"Description": description,
		"Kind":        kind,
	})
}

func WebSocketToast(socketId string, kind ToastKind, title template.HTML, description template.HTML) {
	var str strings.Builder
	if err := toastTemplate.ExecuteTemplate(&str, "content", gin.H{
		"Title":       title,
		"Description": description,
		"Kind":        kind,
	}); err != nil {
		slog.Warn("error rendering SSR page", "err", err, "name", toastTemplate.Name())
		return
	}

	WebSocketSwap(socketId, template.HTML(str.String()))
}

func ToastRouter(g *gin.RouterGroup) {
	g.GET("/error", func(c *gin.Context) {
		Toast(c, ToastError, "Test error message", "Hello, World!")
	})
	g.GET("/info", func(c *gin.Context) {
		Toast(c, ToastInfo, "Test info message", "Hello, World!")
	})
	g.GET("/error/long", func(c *gin.Context) {
		Toast(c, ToastError, "Test error message", "Hello, World!Hello, World!Hello, World!Hello, World!Hello, World!Hello, World!")
	})
	g.GET("/info/long", func(c *gin.Context) {
		Toast(c, ToastInfo, "Test info message", "Hello, World!Hello, World!Hello, World!Hello, World!Hello, World!Hello, World!")
	})
}
