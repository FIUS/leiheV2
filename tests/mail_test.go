package tests

import (
	"bytes"
	"io"
	"mime"
	"mime/quotedprintable"
	netmail "net/mail"
	"os"
	"strings"
	"testing"
	"text/template"

	"leiheV2/internal/mailutil"
)

func TestBookingReviewEmailTemplatesIncludeRelevantInstructions(t *testing.T) {
	data := struct {
		FirstName    string
		BookingToken string
		StartDate    string
		EndDate      string
		Items        []string
	}{
		FirstName: "Erika", BookingToken: "ABC123",
		StartDate: "07.10.2026", EndDate: "10.10.2026",
		Items: []string{"Testspiel"},
	}
	tests := []struct {
		file     string
		contains []string
	}{
		{
			file: "booking_confirmed.txt",
			contains: []string{
				"V38.0.001", "Abholtag: 07.10.2026",
				"verfällt die Buchung", "Buchungsnummer bereit",
			},
		},
		{
			file:     "booking_rejected.txt",
			contains: []string{"abgelehnt und storniert", "fs@fius.de"},
		},
	}

	for _, test := range tests {
		t.Run(test.file, func(t *testing.T) {
			content, err := os.ReadFile("../templates/" + test.file)
			if err != nil {
				t.Fatalf("read template: %v", err)
			}
			tmpl, err := template.New(test.file).Parse(string(content))
			if err != nil {
				t.Fatalf("parse template: %v", err)
			}
			var output strings.Builder
			if err := tmpl.Execute(&output, data); err != nil {
				t.Fatalf("execute template: %v", err)
			}
			for _, expected := range test.contains {
				if !strings.Contains(output.String(), expected) {
					t.Errorf("rendered %s does not contain %q", test.file, expected)
				}
			}
			for _, expected := range []string{"Erika", "ABC123", "Testspiel"} {
				if !strings.Contains(output.String(), expected) {
					t.Errorf("rendered %s does not contain booking data %q", test.file, expected)
				}
			}
		})
	}
}

func TestFormatMailMessageEncodesUTF8HeadersAndBody(t *testing.T) {
	message, recipient, err := mailutil.FormatMessage(
		"FIUS Verleih",
		"noreply@example.org",
		"user@example.org",
		"Ausleihe bestätigt",
		"Grüße aus dem Verleih",
	)
	if err != nil {
		t.Fatalf("format email: %v", err)
	}
	if recipient != "user@example.org" {
		t.Fatalf("envelope recipient = %q, want user@example.org", recipient)
	}

	parsed, err := netmail.ReadMessage(bytes.NewReader(message))
	if err != nil {
		t.Fatalf("parse formatted message: %v", err)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(parsed.Header.Get("Subject"))
	if err != nil {
		t.Fatalf("decode subject: %v", err)
	}
	if subject != "Ausleihe bestätigt" {
		t.Errorf("decoded subject = %q, want UTF-8 subject", subject)
	}
	from, err := netmail.ParseAddress(parsed.Header.Get("From"))
	if err != nil {
		t.Fatalf("parse From header: %v", err)
	}
	if from.Name != "FIUS Verleih" {
		t.Errorf("From display name = %q, want FIUS Verleih", from.Name)
	}
	if parsed.Header.Get("Content-Transfer-Encoding") != "quoted-printable" {
		t.Fatalf("Content-Transfer-Encoding = %q, want quoted-printable", parsed.Header.Get("Content-Transfer-Encoding"))
	}
	body, err := io.ReadAll(quotedprintable.NewReader(parsed.Body))
	if err != nil {
		t.Fatalf("decode email body: %v", err)
	}
	if string(body) != "Grüße aus dem Verleih" {
		t.Errorf("decoded body = %q, want UTF-8 body", body)
	}
	if strings.Contains(parsed.Header.Get("Subject"), "bestätigt") {
		t.Error("non-ASCII subject should be MIME encoded in the raw header")
	}
}

func TestFormatMailMessageRejectsInvalidAddresses(t *testing.T) {
	if _, _, err := mailutil.FormatMessage("FIUS Verleih", "invalid", "user@example.org", "Test", "Body"); err == nil {
		t.Error("expected invalid sender address to be rejected")
	}
	if _, _, err := mailutil.FormatMessage("FIUS Verleih", "noreply@example.org", "invalid", "Test", "Body"); err == nil {
		t.Error("expected invalid recipient address to be rejected")
	}
}
