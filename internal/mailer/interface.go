package mailer

import (
	"fmt"
	"html/template"
	"net/mail"
	"os"
	"strings"
)

var DefaultMailer Mailer

type Mailer interface {
	SendMail(to *mail.Address, subject string, body template.HTML) error
}

func SendMail(to *mail.Address, subject, body string) error {
	return DefaultMailer.SendMail(to, subject, template.HTML(body))
}

func SendMailTemplated(to *mail.Address, subject string, tmpl *template.Template, data any) error {
	var writer strings.Builder
	err := tmpl.ExecuteTemplate(&writer, "layout", data)
	if err != nil {
		return err
	}

	return DefaultMailer.SendMail(to, subject, template.HTML(writer.String()))
}

func InitMailer() error {
	mode := os.Getenv("MAIL_MODE")
	if mode == "" {
		mode = "netmail"
	}

	switch mode {
	case "netmail":
		return InitNetMailer()
	case "memorymail":
		InitMemoryMailer()
		return nil
	default:
		panic(fmt.Sprintf("Invalid mail mode: %s", mode))
	}
}
