package messaging

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"geekswimmers/config"
	"geekswimmers/storage"
	"geekswimmers/utils"
	"html/template"
	"log"
	"net"
	"net/mail"
	"net/smtp"
	"regexp"
	"strings"
)

var emailRegex = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

type EmailContext struct {
	ServerUrl    string
	Username     string
	CurrentEmail string
	NewEmail     string
	Confirmation string
	Name         string
	Summary      string
	Message      string
	ID           int64
}

// SendMessage uses SMTP to send an email to the recipient with the message in the body.
func SendMessage(to, username, subject, body string, db storage.Database) {
	user := config.GetConfiguration().GetString(config.EmailUsername)
	password := config.GetConfiguration().GetString(config.EmailPassword)
	server := config.GetConfiguration().GetString(config.EmailServer)
	from := config.GetConfiguration().GetString(config.EmailFrom)

	auth := smtp.PlainAuth("", user, password, server)

	msg := composeMimeMail(to, from, subject, body)

	port := config.GetConfiguration().GetInt(config.EmailPort)
	if err := smtp.SendMail(fmt.Sprintf("%s:%d", server, port), auth, from, []string{to}, msg); err != nil {
		log.Printf("Error sending an email to %v: %v", to, err)
		return
	}

	emailMessageSent := &EmailMessageSent{
		Recipient: to,
		Username:  username,
		Subject:   subject,
		Body:      body,
	}

	if err := emailMessageSent.Insert(db); err != nil {
		log.Printf("utils.SendMessage(%v, %v, %v, %v) -> %v", to, username, subject, body, err)
	} else {
		log.Printf("Email '%v' sent to %v", subject, to)
	}
}

func GetEmailTemplate(name string, context *EmailContext) string {
	emailBody, err := template.ParseFiles(fmt.Sprintf("web/templates/messages/%s.txt", name))
	utils.Check(err)

	var bodyContent bytes.Buffer
	if err = emailBody.Execute(&bodyContent, context); err != nil {
		log.Printf("email template not found: %v", err)
	}

	return bodyContent.String()
}

func composeMimeMail(to string, from string, subject string, body string) []byte {
	header := make(map[string]string)
	header["From"] = formatEmailAddress(from)
	header["To"] = formatEmailAddress(to)
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/plain; charset=\"utf-8\""
	header["Content-Transfer-Encoding"] = "base64"

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + base64.StdEncoding.EncodeToString([]byte(body))

	return []byte(message)
}

// Never fails, tries to format the address if possible
func formatEmailAddress(addr string) string {
	address, err := mail.ParseAddress(addr)
	if err != nil {
		return addr
	}
	return address.String()
}

// IsEmailAddress just verify if the address looks like an email address.
func IsEmailAddress(addr string) bool {
	if len(addr) < 3 && len(addr) > 254 {
		return false
	}

	if !emailRegex.MatchString(addr) {
		return false
	}

	return true
}

// IsEmailAddressValid in addition to check if the address look like an email address,
//it also checks if the domain has a valid MX record.
func IsEmailAddressValid(addr string) bool {
	if !IsEmailAddress(addr) {
		return false
	}

	parts := strings.Split(addr, "@")
	mx, err := net.LookupMX(parts[1])
	if err != nil || len(mx) == 0 {
		return false
	}

	return true
}
