package mailer

import (
	"context"
	"strings"
	"testing"

	"opslira/backend/internal/config"
)

func TestBuildMessageHeadersAndBody(t *testing.T) {
	message := string(buildMessage("Opslira Monitoring", "opslira@example.id",
		[]string{"a@example.id", "b@example.id"},
		"[Opslira] DAILY report, Daily Operations",
		"Report 01 Oct 2026 to 02 Oct 2026\nGlobal SLA: 100%\nIncidents: 0"))

	for _, want := range []string{
		// mail.Address quotes display names containing spaces (RFC 5322).
		"From: \"Opslira Monitoring\" <opslira@example.id>\r\n",
		"To: a@example.id, b@example.id\r\n",
		"Subject: [Opslira] DAILY report, Daily Operations\r\n",
		"MIME-Version: 1.0\r\n",
		"Content-Type: text/plain; charset=UTF-8\r\n",
		"Content-Transfer-Encoding: base64\r\n",
	} {
		if !strings.Contains(message, want) {
			t.Errorf("message missing %q:\n%s", want, message)
		}
	}
	// The body travels base64-encoded: the raw newline text must not appear.
	if strings.Contains(message, "Global SLA") {
		t.Error("body was sent raw instead of base64")
	}
}

func TestEncodeHeaderKeepsAsciiAndEncodesUtf8(t *testing.T) {
	if got := encodeHeader("plain subject"); got != "plain subject" {
		t.Errorf("ascii header = %q", got)
	}
	encoded := encodeHeader("Laporan Harian ✓")
	if !strings.HasPrefix(encoded, "=?UTF-8?") && !strings.HasPrefix(encoded, "=?utf-8?") {
		t.Errorf("utf8 header not RFC 2047 encoded: %q", encoded)
	}
}

func TestSendWithoutHostFailsFast(t *testing.T) {
	m := New(config.Mail{})
	err := m.Send(context.Background(), []string{"a@example.id"}, "s", "b")
	if err == nil || !strings.Contains(err.Error(), "MAIL_HOST") {
		t.Errorf("err = %v, want MAIL_HOST configuration error", err)
	}
}

func TestSendWithoutRecipientsFailsFast(t *testing.T) {
	m := New(config.Mail{Host: "smtp.example.id", FromAddress: "x@example.id"})
	if err := m.Send(context.Background(), nil, "s", "b"); err == nil {
		t.Error("expected error for zero recipients")
	}
}
