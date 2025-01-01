package mailer

import (
	"html/template"
	"net/mail"
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
