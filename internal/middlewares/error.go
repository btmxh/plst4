package middlewares

import (
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	"github.com/btmxh/plst4/internal/html"
	"github.com/btmxh/plst4/internal/stores"
	"github.com/gin-gonic/gin"
)

const ErrorTitle = "error-title"

func ErrorMiddleware(callback func(c *gin.Context, title, desc template.HTML)) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			title := stores.GetErrorTitle(c)
			slog.Warn("Error handling request", "title", title, "errors", c.Errors)

			var descriptions []string
			for _, err := range c.Errors {
				if err.Type == gin.ErrorTypePublic {
					descriptions = append(descriptions, string(html.StringAsHTML(err.Error())))
				}
			}

			// join all descriptions with separator being <br>
			var description template.HTML
			if len(descriptions) > 0 {
				description = template.HTML(strings.Join(descriptions, "<br>"))
			} else {
				description = template.HTML("Internal server error")
				c.Status(http.StatusInternalServerError)
			}

			callback(c, title, description)
		}
	}
}
