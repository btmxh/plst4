package routes

import (
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/btmxh/plst4/internal/html"
	"github.com/btmxh/plst4/internal/middlewares"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func getTemplate(name string, paths ...string) *template.Template {
	return html.GetTemplate(name, paths...)
}

func CreateMainRouter() http.Handler {
	router := gin.Default()

	gzipMode, err := strconv.Atoi(os.Getenv("GZIP_MODE"))
	if err != nil {
		slog.Warn("Invalid value for GZIP_MODE environment variable", "err", err)
		gzipMode = 0
	}

	router.Use(gzip.Gzip(gzipMode))
	router.Use(middlewares.AuthMiddleware())

	router.GET("/", HomeRouter)
	AuthRouter(router.Group("/auth"))
	ToastRouter(router.Group("/toast"))
	WatchRouter(router.Group("/watch"))
	PlaylistRouter(router.Group("/playlists"))
	WebSocketRouter(router.Group("/ws"))
	// only enabled when using memorymail
	MailRouter(router.Group("/mail"))

	router.Static("/scripts", "./dist/scripts")
	router.Static("/styles", "./dist/styles")
	router.Static("/assets", "./dist/assets")
	router.StaticFile("/libs/htmx.min.js", "./node_modules/htmx.org/dist/htmx.esm.js")

	// for source map only
	if gin.Mode() != gin.ReleaseMode {
		router.Static("/www", "./www")
		router.Static("/testmedias", "./www/testmedias")
	}

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

func HxNoswap(c *gin.Context) {
	c.Header("Hx-Reswap", "none")
}

func UpdateTitle(c *gin.Context, title string) {
	title = template.HTMLEscapeString(title)
	c.Writer.WriteString("<title>")
	c.Writer.WriteString(title)
	c.Writer.WriteString("</title>")
}
