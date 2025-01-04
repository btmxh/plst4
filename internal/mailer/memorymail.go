package mailer

import (
	"html/template"
	"log/slog"
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
	slog.Info("Memory mail sent", slog.String("to", to.Address), slog.String("subject", subject))
	return nil
}
