package mailer

import (
	"html/template"
	"log/slog"
	"net/mail"
	"sync"
)

type Mail struct {
	Subject string
	Body    template.HTML
}

type MemoryMailer struct {
	mutex   sync.Mutex
	Inboxes map[string][]Mail
}

func InitMemoryMailer() {
	DefaultMailer = &MemoryMailer{Inboxes: make(map[string][]Mail)}
}

func (mailer *MemoryMailer) SendMail(to *mail.Address, subject string, body template.HTML) error {
	mailer.mutex.Lock()
	defer mailer.mutex.Unlock()

	mail := Mail{Subject: subject, Body: body}
	mailer.Inboxes[to.Address] = append(mailer.Inboxes[to.Address], mail)
	slog.Info("Memory mail sent", slog.String("to", to.Address), slog.String("subject", subject))
	return nil
}

func (mailer *MemoryMailer) GetInbox(email string, callback func([]Mail)) {
	mailer.mutex.Lock()
	defer mailer.mutex.Unlock()

	callback(mailer.Inboxes[email])
}
