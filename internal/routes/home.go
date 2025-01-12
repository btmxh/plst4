package routes

import (
	"github.com/btmxh/plst4/internal/html"
	"github.com/gin-gonic/gin"
)

var homeTemplate = getTemplate("home", "templates/home.tmpl")

func HomeRouter(c *gin.Context) {
	html.RenderGin(homeTemplate, c, "layout", gin.H{})
}
