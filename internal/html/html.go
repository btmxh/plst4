package html

import (
	"fmt"
	"html/template"
	"maps"
	"time"

	"github.com/btmxh/plst4/internal/auth"
	"github.com/gin-gonic/gin"
)

func StringAsHTML(s string) template.HTML {
	return template.HTML(template.HTMLEscapeString(s))
}

func CombineArgs(args ...gin.H) gin.H {
	all := gin.H{}
	for _, arg := range args {
		maps.Copy(all, arg)
	}
	return all
}

func Render(tmpl *template.Template, c *gin.Context, block string, arg gin.H) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, block, CombineArgs(gin.H{"Context": c}, arg)); err != nil {
		c.Error(err).SetType(gin.ErrorTypeRender)
		return
	}
}

func RenderFunc(tmpl *template.Template, block string, arg gin.H) gin.HandlerFunc {
	return func(c *gin.Context) {
		Render(tmpl, c, block, arg)
	}
}

func DefaultFuncMap() template.FuncMap {
	return template.FuncMap{
		"HasUsername": func(c *gin.Context) bool {
			return auth.IsLoggedIn(c)
		},
		"GetUsername": func(c *gin.Context) string {
			return auth.GetUsername(c)
		},
		"FormatTimestampUTC": func(t time.Time) template.HTML {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.Local)
			return template.HTML("<span class=\"timestamp\" data-value=\"" + t.Local().UTC().Format(time.RFC3339) + "\"></span>")
		},
		"FormatDuration": func(d time.Duration) string {
			hours := int(d / time.Hour)
			minutes := int((d % time.Hour) / time.Minute)
			seconds := int((d % time.Minute) / time.Second)

			return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
		},
		"HumanIndex": func(i int) int {
			return i + 1
		},
		"Get": func(c *gin.Context, name string) string {
			if c.Request.Method == "POST" {
				return c.PostForm(name)
			} else {
				return c.Query(name)
			}
		},
	}
}
