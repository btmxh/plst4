package mailer

import (
	"encoding/json"
	"html/template"
	"log/slog"
	"net/mail"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type FSMailer struct{}

func InitFSMailer() {
	DefaultMailer = &FSMailer{}
}

func (mailer *FSMailer) SendMail(to *mail.Address, subject string, body template.HTML) error {
	dirName := strings.ReplaceAll(to.Address, "@", "_at_")

	if err := os.MkdirAll(".mail/"+dirName, 0755); err != nil {
		return err
	}

	filename := ".mail/" + dirName + "/" + uuid.NewString() + ".html"
	content, err := json.Marshal(gin.H{
		"subject": subject,
		"body":    string(body),
	})
	if err != nil {
		return err
	}

	if err := os.WriteFile(filename, content, 0644); err != nil {
		return err
	}

	slog.Info("FS mail sent", slog.String("to", to.Address), slog.String("subject", subject), slog.String("path", filename))
	return nil
}
