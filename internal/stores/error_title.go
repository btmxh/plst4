package stores

import (
	"html/template"

	"github.com/gin-gonic/gin"
)

const ErrorTitle = "error-title"

func SetErrorTitle(c *gin.Context, title template.HTML) {
	c.Set(ErrorTitle, title)
}

func GetErrorTitle(c *gin.Context) template.HTML {
	if value, ok := c.Get(ErrorTitle); ok && value != nil {
		title, ok := value.(template.HTML)
		if ok {
			return title
		}
  }

	return "Error"
}
