package handlers

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"strings"

	"leiheV2/db"
)

type mailConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	FromName string
	StartTLS bool
}

var mail mailConfig

func InitMail() error {
	host, err := requiredEnv("SMTP_HOST")
	if err != nil {
		return err
	}
	if strings.Contains(host, "@") || strings.ContainsAny(host, " /") {
		return fmt.Errorf("SMTP_HOST must be a server hostname, not an email address")
	}
	portText, err := requiredEnv("SMTP_PORT")
	if err != nil {
		return err
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("SMTP_PORT must be a valid port")
	}
	username, err := firstConfiguredEnv("SMTP_USERNAME", "SMTP_USER")
	if err != nil {
		return err
	}
	password, err := firstConfiguredEnv("SMTP_PASSWORD", "SMTP_PWD")
	if err != nil {
		return err
	}
	from, err := firstConfiguredEnv("SMTP_FROM", "SUPPORT_EMAIL")
	if err != nil {
		return err
	}
	fromName, err := requiredEnv("SMTP_FROM_NAME")
	if err != nil {
		return err
	}
	startTLS, err := requiredBool("SMTP_STARTTLS")
	if err != nil {
		return err
	}
	if _, err := requiredEnv("APP_PUBLIC_URL"); err != nil {
		return err
	}
	mail = mailConfig{Host: host, Port: port, Username: username, Password: password,
		From: from, FromName: fromName, StartTLS: startTLS}
	return nil
}

func firstConfiguredEnv(keys ...string) (string, error) {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value, nil
		}
	}
	return "", fmt.Errorf("%s must be configured", strings.Join(keys, " or "))
}

func sendMail(to, subject, body string) error {
	if strings.TrimSpace(to) == "" {
		return fmt.Errorf("email recipient is empty")
	}
	address := net.JoinHostPort(mail.Host, strconv.Itoa(mail.Port))
	headers := fmt.Sprintf("From: %s <%s>\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n",
		mail.FromName, mail.From, to, subject)
	message := []byte(headers + body)

	var auth smtp.Auth
	if mail.Username != "" {
		auth = smtp.PlainAuth("", mail.Username, mail.Password, mail.Host)
	}
	if !mail.StartTLS {
		if err := smtp.SendMail(address, auth, mail.From, []string{to}, message); err != nil {
			return fmt.Errorf("SMTP send failed: %w", err)
		}
		log.Printf("SMTP accepted email for delivery (subject=%q)", subject)
		return nil
	}

	conn, err := smtp.Dial(address)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.StartTLS(&tls.Config{ServerName: mail.Host, MinVersion: tls.VersionTLS12}); err != nil {
		return err
	}
	if auth != nil {
		if err := conn.Auth(auth); err != nil {
			return err
		}
	}
	if err := conn.Mail(mail.From); err != nil {
		return err
	}
	if err := conn.Rcpt(to); err != nil {
		return err
	}
	writer, err := conn.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("SMTP send failed: %w", err)
	}
	log.Printf("SMTP accepted email for delivery (subject=%q)", subject)
	return nil
}

func sendVerificationEmail(user db.User, token string) error {
	body, err := renderMailTemplate("templates/email_verification.txt", map[string]string{
		"FirstName": user.FirstName, "VerificationURL": verificationURL(token), "ExpiresIn": "24 Stunden",
	})
	if err != nil {
		return err
	}
	return sendMail(user.Email, "E-Mail-Adresse bestätigen", body)
}

func sendOrderEmail(to, firstName string, details db.BookingEmailDetails) error {
	body, err := renderMailTemplate("templates/order_confirmation.txt", struct {
		FirstName    string
		BookingToken string
		StartDate    string
		EndDate      string
		Items        []string
		Status       string
	}{
		FirstName: firstName, BookingToken: details.Token,
		StartDate: details.StartDate.Format("02.01.2006"), EndDate: details.EndDate.Format("02.01.2006"),
		Items: details.ItemNames, Status: details.Status,
	})
	if err != nil {
		return err
	}
	subject := "Reservierungsanfrage erhalten"
	if details.Status == "confirmed" {
		subject = "Ausleihe bestätigt"
	} else if details.Status == "active" {
		subject = "Ausleihe"
	}
	return sendMail(to, subject, body)
}

func sendBookingUpdateEmail(details db.BookingEmailDetails, change string) error {
	body, err := renderMailTemplate("templates/booking_update.txt", struct {
		FirstName    string
		BookingToken string
		StartDate    string
		EndDate      string
		Items        []string
		Status       string
		Change       string
	}{
		FirstName: details.FirstName, BookingToken: details.Token,
		StartDate: details.StartDate.Format("02.01.2006"), EndDate: details.EndDate.Format("02.01.2006"),
		Items: details.ItemNames, Status: bookingStatusLabel(details.Status), Change: change,
	})
	if err != nil {
		return err
	}
	subject := "Änderung deiner Ausleihe"
	if details.Status == "active" && change == "ausgegeben" {
		subject = "Ausleihe"
	}
	return sendMail(details.Email.String, subject, body)
}

func bookingStatusLabel(status string) string {
	switch status {
	case "pending":
		return "Ausstehend"
	case "confirmed":
		return "Bestätigt"
	case "active":
		return "Ausgegeben"
	case "returned":
		return "Zurückgegeben"
	case "cancelled":
		return "Storniert"
	default:
		return status
	}
}

func renderMailTemplate(path string, data any) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read mail template %s: %w", path, err)
	}
	tmpl, err := template.New(path).Parse(string(content))
	if err != nil {
		return "", fmt.Errorf("parse mail template %s: %w", path, err)
	}
	var output bytes.Buffer
	if err := tmpl.Execute(&output, data); err != nil {
		return "", fmt.Errorf("render mail template %s: %w", path, err)
	}
	return output.String(), nil
}

func verificationURL(token string) string {
	return strings.TrimRight(os.Getenv("APP_PUBLIC_URL"), "/") + "/auth/local/verify?token=" + token
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func logMailError(kind string, err error) {
	if err != nil {
		log.Printf("%s email could not be sent: %v", kind, err)
	}
}
