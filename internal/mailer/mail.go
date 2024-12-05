package mailer

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/gomail.v2"
)

var Dealer *gomail.Dialer

func InitMail() error {
	port, err := strconv.Atoi(os.Getenv("MAIL_PORT"))
	if err != nil {
		return fmt.Errorf("Invalid port: %w", err)
	}
	Dealer = gomail.NewDialer(os.Getenv("MAIL_HOST"), port, os.Getenv("MAIL_EMAIL"), os.Getenv("MAIL_PASSWORD"))
	return nil
}
