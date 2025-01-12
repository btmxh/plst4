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
		var lastMail mailer.Mail
		var hasMail bool
		mail.GetInbox(email, func(inbox []mailer.Mail) {
			if len(inbox) > 0 {
				lastMail = inbox[len(inbox)-1]
				hasMail = true
			}
		})
		if !hasMail {
			c.AbortWithStatus(http.StatusNotFound)
		} else {
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.Header("Subject", lastMail.Subject)
			c.String(http.StatusOK, string(lastMail.Body))
		}
	})
}
