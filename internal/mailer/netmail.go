package mailer

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/mail"
	"os"
	"strconv"

	"gopkg.in/gomail.v2"
)

type NetMailer struct {
	Dealer *gomail.Dialer
}

func InitNetMailer() error {
	port, err := strconv.Atoi(os.Getenv("MAIL_PORT"))
	if err != nil {
		return fmt.Errorf("Invalid port: %w", err)
	}
	Dealer := gomail.NewDialer(os.Getenv("MAIL_HOST"), port, os.Getenv("MAIL_EMAIL"), os.Getenv("MAIL_PASSWORD"))
	DefaultMailer = &NetMailer{Dealer: Dealer}
	return nil
}

func (mailer *NetMailer) SendMail(to *mail.Address, subject string, body template.HTML) error {
	m := gomail.NewMessage()
	m.SetHeader("From", "plst4@plst.dev")
	m.SetHeader("To", to.Address)
	m.SetBody("text/html", string(body))
	go func() {
		if err := mailer.Dealer.DialAndSend(m); err != nil {
			slog.Error("Failed to send netmail", "err", err, slog.String("to", to.Address), slog.String("subject", subject))
		} else {
			slog.Info("Net mail sent", slog.String("to", to.Address), slog.String("subject", subject))
		}
	}()
	return nil
}
