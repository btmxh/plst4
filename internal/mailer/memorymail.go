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
	inboxes map[string][]Mail
}

func InitMemoryMailer() {
	DefaultMailer = &MemoryMailer{inboxes: make(map[string][]Mail)}
}

func (mailer *MemoryMailer) SendMail(to *mail.Address, subject string, body template.HTML) error {
	mail := Mail{Subject: subject, Body: body}
	mailer.inboxes[to.Address] = append(mailer.inboxes[to.Address], mail)
	return nil
}
