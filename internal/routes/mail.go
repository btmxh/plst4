package routes

import (
	"net/http"

	"github.com/btmxh/plst4/internal/mailer"
	"github.com/gin-gonic/gin"
)

func MailRouter(g *gin.RouterGroup) {
	mail, ok := mailer.DefaultMailer.(*mailer.MemoryMailer)
	if !ok {
		return
	}

	g.GET("/", func(c *gin.Context) {
		email := c.Query("email")
		inbox, ok := mail.Inboxes[email]
		if !ok || len(inbox) == 0 {
			c.AbortWithStatus(http.StatusNotFound)
		} else {
			item := inbox[len(inbox)-1]
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.Header("Subject", item.Subject)
			c.String(http.StatusOK, string(item.Body))
		}
	})
}
