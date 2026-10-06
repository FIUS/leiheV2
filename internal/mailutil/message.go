package mailutil

import (
	"bytes"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
)

func FormatMessage(fromName, fromAddress, to, subject, body string) ([]byte, string, error) {
	if strings.TrimSpace(to) == "" {
		return nil, "", fmt.Errorf("email recipient is empty")
	}
	from, err := mail.ParseAddress(fromAddress)
	if err != nil || from.Address != fromAddress {
		return nil, "", fmt.Errorf("invalid SMTP sender address")
	}
	recipient, err := mail.ParseAddress(to)
	if err != nil || recipient.Address != to {
		return nil, "", fmt.Errorf("invalid email recipient address")
	}
	var message bytes.Buffer
	headers := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n",
		(&mail.Address{Name: fromName, Address: from.Address}).String(),
		recipient.String(),
		mime.QEncoding.Encode("UTF-8", subject),
	)
	message.WriteString(headers)
	writer := quotedprintable.NewWriter(&message)
	if _, err := writer.Write([]byte(body)); err != nil {
		return nil, "", fmt.Errorf("encode email body: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("encode email body: %w", err)
	}
	return message.Bytes(), recipient.Address, nil
}
