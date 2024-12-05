package routes

import "github.com/gin-gonic/gin"

var homeTemplate = getTemplate("home", "templates/home.tmpl")

func HomeRouter(c *gin.Context) {
	SSR(homeTemplate, c, "layout", gin.H{})
}
