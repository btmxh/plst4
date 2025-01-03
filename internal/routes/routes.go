package routes

import (
	"html/template"
	"net/http"
	"net/url"

	"github.com/btmxh/plst4/internal/html"
	"github.com/btmxh/plst4/internal/middlewares"
	"github.com/gin-gonic/gin"
)

func getTemplate(name string, paths ...string) *template.Template {
	paths = append(paths, "templates/layout.tmpl")
	return template.Must(template.New(name).Funcs(html.DefaultFuncMap()).ParseFiles(paths...))
}

func CreateMainRouter() http.Handler {
	router := gin.Default()
	router.Use(middlewares.AuthMiddleware())

	router.GET("/", HomeRouter)
	AuthRouter(router.Group("/auth"))
	ToastRouter(router.Group("/toast"))
	WatchRouter(router.Group("/watch"))
	PlaylistRouter(router.Group("/playlists"))
	WebSocketRouter(router.Group("/ws"))

	router.Static("/scripts", "./dist/scripts")
	router.Static("/styles", "./dist/styles")
	router.Static("/assets", "./dist/assets")
	router.StaticFile("/libs/htmx.min.js", "./node_modules/htmx.org/dist/htmx.esm.js")
	// for source map only, may be disabled in prod
	router.Static("/www", "./www")

	return router
}

func HxRedirect(c *gin.Context, route string) {
	c.Header("Hx-Redirect", route)
}

func HxPushURL(c *gin.Context, route string) {
	c.Header("Hx-Push-Url", route)
}

func HxRefresh(c *gin.Context) {
	c.Header("Hx-Refresh", "true")
}

func HxPrompt(c *gin.Context) (string, error) {
	return url.PathUnescape(c.GetHeader("Hx-Prompt"))
}
