package routes

import (
	"fmt"
	"html/template"
	"log/slog"
	"maps"
	"net/http"
	"time"

	"github.com/btmxh/plst4/internal/middlewares"
	"github.com/gin-gonic/gin"
)

func getTemplate(name string, paths ...string) *template.Template {
	paths = append(paths, "templates/layout.tmpl")
	return template.Must(template.New(name).Funcs(template.FuncMap{
		"HasUsername": func(c *gin.Context) bool {
			_, exists := c.Get(middlewares.AUTH_OBJECT_KEY)
			return exists
		},
		"GetUsername": func(c *gin.Context) string {
			username, _ := c.Get(middlewares.AUTH_OBJECT_KEY)
			return username.(string)
		},
		"FormatTimestampUTC": func(t time.Time) template.HTML {
			return template.HTML("<span class=\"timestamp\" data-value=\"" + t.In(time.UTC).Format(time.RFC3339) + "\"></span>")
		},
		"FormatDuration": func(d time.Duration) string {
			hours := int(d / time.Hour)
			minutes := int((d % time.Hour) / time.Minute)
			seconds := int((d % time.Minute) / time.Second)

			return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
		},
	}).ParseFiles(paths...))
}

func CreateMainRouter() http.Handler {
	router := gin.Default()
	router.Use(middlewares.AuthMiddleware())

	router.GET("/", HomeRouter)
	AuthRouter(router.Group("/auth"))
	ToastRouter(router.Group("/toast"))
	WatchRouter(router.Group("/watch"))
	PlaylistRouter(router.Group("/playlists"))
	router.Static("/scripts", "./dist/scripts")
	router.Static("/styles", "./dist/styles")
	router.Static("/assets", "./dist/assets")
	router.StaticFile("/libs/htmx.min.js", "./node_modules/htmx.org/dist/htmx.esm.js")
	// for source map only, may be disabled in prod
	router.Static("/www", "./www")

	return router
}

func SSR(tmpl *template.Template, c *gin.Context, block string, arg gin.H) {
	if err := tmpl.ExecuteTemplate(c.Writer, block, Combine(gin.H{"Context": c}, arg)); err != nil {
		slog.Warn("error rendering SSR page", "err", err, "name", tmpl.Name())
		Error(c.Writer, "SSR error")
		return
	}

	c.Header("Content-Type", "text/html")
}

func SSRRoute(tmpl *template.Template, block string, arg gin.H) gin.HandlerFunc {
	return func(c *gin.Context) {
		SSR(tmpl, c, block, arg)
	}
}

func Error(w http.ResponseWriter, msg string, args ...any) {
	w.Write([]byte(fmt.Sprintf(msg, args...)))
	w.WriteHeader(http.StatusInternalServerError)
	slog.Debug("Error handling request", "msg", msg, "args", args)
}

func Redirect(c *gin.Context, route string) {
	c.Header("Hx-Redirect", route)
}

func PushURL(c *gin.Context, route string) {
	c.Header("Hx-Push-Url", route)
}

func Refresh(c *gin.Context) {
	c.Header("Hx-Refresh", "true")
}

func Combine(args ...gin.H) gin.H {
	all := gin.H{}
	for _, arg := range args {
		maps.Copy(all, arg)
	}
	return all
}
