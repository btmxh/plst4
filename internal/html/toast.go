package html

import (
	"html/template"
	"io"

	"github.com/gin-gonic/gin"
)

var toastTemplate = template.Must(template.ParseFiles("templates/notifications/toast.tmpl"))

type ToastKind string

const (
	ToastError ToastKind = "error"
	ToastInfo  ToastKind = "info"
)

func RenderToast(w io.Writer, kind ToastKind, title template.HTML, description template.HTML) error {
	return toastTemplate.ExecuteTemplate(w, "content", gin.H{
		"Title":       title,
		"Description": description,
		"Kind":        kind,
	})
}
