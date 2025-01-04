package mailer

import (
	"html/template"
	"net/mail"
)

type Mail struct {
	Subject string
	Body    template.HTML
}

type MemoryMailer struct {
	Inboxes map[string][]Mail
}

func InitMemoryMailer() {
	DefaultMailer = &MemoryMailer{Inboxes: make(map[string][]Mail)}
}

func (mailer *MemoryMailer) SendMail(to *mail.Address, subject string, body template.HTML) error {
	mail := Mail{Subject: subject, Body: body}
	mailer.Inboxes[to.Address] = append(mailer.Inboxes[to.Address], mail)
	return nil
}
