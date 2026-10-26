// Package mailer sends outgoing mail over SMTP (STARTTLS by default,
// implicit TLS with MAIL_SCHEME=smtps), matching Laravel's SMTP transport
// behavior for alerts and scheduled reports.
package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"opslira/backend/internal/config"
)

// Mailer sends one message per call; it is stateless and safe for
// concurrent use.
type Mailer struct {
	cfg config.Mail
}

// New wraps the SMTP configuration.
func New(cfg config.Mail) *Mailer {
	return &Mailer{cfg: cfg}
}

// Send delivers a plain-text message to every recipient.
func (m *Mailer) Send(ctx context.Context, to []string, subject, body string) error {
	if m.cfg.Host == "" {
		return errors.New("SMTP is not configured (MAIL_HOST is empty)")
	}
	if len(to) == 0 {
		return errors.New("no recipients")
	}
	if strings.TrimSpace(m.cfg.FromAddress) == "" {
		return errors.New("MAIL_FROM_ADDRESS is empty")
	}

	addr := net.JoinHostPort(m.cfg.Host, m.cfg.Port)
	implicitTLS := strings.EqualFold(m.cfg.Scheme, "smtps")

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp dial %s: %w", addr, err)
	}

	deadline := time.Now().Add(30 * time.Second)
	if d, ok := ctx.Deadline(); ok {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	defer conn.Close()

	if implicitTLS {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: m.cfg.Host})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return fmt.Errorf("smtp tls handshake: %w", err)
		}
		conn = tlsConn
	}

	client, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	// Upgrade when the server advertises STARTTLS (port 587 behavior).
	if !implicitTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			tlsConfig := &tls.Config{ServerName: m.cfg.Host}
			if err := client.StartTLS(tlsConfig); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		}
	}

	if m.cfg.Username != "" {
		if ok, _ := client.Extension("AUTH"); ok {
			auth := smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
	}

	if err := client.Mail(m.cfg.FromAddress); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("smtp RCPT %s: %w", rcpt, err)
		}
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	message := buildMessage(m.cfg.FromName, m.cfg.FromAddress, to, subject, body)
	if _, err := io.Copy(writer, bytes.NewReader(message)); err != nil {
		_ = writer.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("smtp finish: %w", err)
	}

	return client.Quit()
}

// buildMessage renders an RFC 5322 plain-text message; the body travels
// base64-encoded so any UTF-8 content survives strict servers.
func buildMessage(fromName, fromAddr string, to []string, subject, body string) []byte {
	var buf bytes.Buffer
	buf.WriteString("From: " + formatAddress(fromName, fromAddr) + "\r\n")
	buf.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	buf.WriteString("Subject: " + encodeHeader(subject) + "\r\n")
	buf.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")

	encoded := base64.StdEncoding.EncodeToString([]byte(body))
	for len(encoded) > 76 {
		buf.WriteString(encoded[:76] + "\r\n")
		encoded = encoded[76:]
	}
	if len(encoded) > 0 {
		buf.WriteString(encoded + "\r\n")
	}

	return buf.Bytes()
}

func formatAddress(name, address string) string {
	if name == "" {
		return address
	}

	addr := mail.Address{Name: name, Address: address}
	return addr.String()
}

// encodeHeader keeps pure-ASCII subjects verbatim and RFC 2047-encodes
// anything else.
func encodeHeader(value string) string {
	for _, r := range value {
		if r > 127 {
			return mime.QEncoding.Encode("UTF-8", value)
		}
	}

	return value
}
