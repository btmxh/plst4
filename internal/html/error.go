package html

import (
	"html/template"

	"github.com/gin-gonic/gin"
)

var errorTemplate = GetTemplate("error", "templates/error.tmpl")

func RenderError(c *gin.Context, title, description template.HTML) {
	RenderGin(errorTemplate, c, "layout", gin.H{
		"Title": title, "Description": description,
	})
}
