// Package mail provides a minimal, best-effort SMTP email sender.
//
// A mailer is built from an explicit Config (the caller supplies it — e.g. the
// Job Queue scheduler loads it from the SMTP_Setup table). A mailer that is not
// enabled is a no-op, so unconfigured environments never attempt delivery.
// Sending is best-effort: callers should log a send error but must not let it
// fail the surrounding work.
package mail

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net"
	netmail "net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Sender sends a plain-text email. Implemented by SMTPMailer and by test doubles.
type Sender interface {
	// Send delivers one plain-text message to all recipients (addresses as in an
	// address list: "ops@example.com" or "Ops <ops@example.com>"). Returns nil when
	// sending is disabled; a *RecipientsError when some recipients were rejected
	// while the others got the message.
	Send(to []string, subject, body string) error
	// Enabled reports whether the sender will actually attempt delivery.
	Enabled() bool
}

// Attachment is a file sent with a message (a scheduled report's PDF)
type Attachment struct {
	Name        string // the file name the recipient sees
	ContentType string // e.g. application/pdf
	Data        []byte
}

// AttachmentSender is a Sender that can send files with a message (SMTPMailer); a caller with
// files to send checks for it and sends the plain message otherwise
type AttachmentSender interface {
	Sender
	SendWithAttachments(to []string, subject, body string, attachments []Attachment) error
}

// Config holds SMTP settings.
type Config struct {
	Enabled  bool
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

// SMTPMailer sends mail via net/smtp.
type SMTPMailer struct {
	cfg Config
}

// NewSMTPMailer builds a mailer from the given config.
func NewSMTPMailer(cfg Config) *SMTPMailer {
	return &SMTPMailer{cfg: cfg}
}

// Enabled reports whether delivery is configured and turned on.
func (m *SMTPMailer) Enabled() bool {
	return m.cfg.Enabled && m.cfg.Host != "" && m.cfg.From != ""
}

// dialTimeout and sessionTimeout keep an unreachable or stalled server from blocking
// the caller (the scheduler runs jobs one after another).
const (
	dialTimeout    = 30 * time.Second
	sessionTimeout = 2 * time.Minute
)

// RecipientError is a recipient (as passed to Send) the message could not be sent to.
type RecipientError struct {
	Recipient string
	Err       error
}

// RecipientsError reports the recipients that did not get the message. Sent counts
// those that did (0: nothing was sent).
type RecipientsError struct {
	Rejected []RecipientError
	Sent     int
}

func (e *RecipientsError) Error() string {
	parts := make([]string, len(e.Rejected))
	for i, r := range e.Rejected {
		parts[i] = fmt.Sprintf("%s: %v", r.Recipient, r.Err)
	}
	return "mail: not sent to " + strings.Join(parts, "; ")
}

// Send delivers one plain-text message to all recipients in one SMTP session (one
// RCPT TO each, all in the To header). A recipient the server rejects (or that is not
// a valid address) does not stop the others: Send returns a *RecipientsError naming
// it. When disabled it is a no-op and returns nil.
func (m *SMTPMailer) Send(to []string, subject, body string) error {
	return m.SendWithAttachments(to, subject, body, nil)
}

// SendWithAttachments sends a message as Send does, with the files attached (a multipart
// message; none: plain text)
func (m *SMTPMailer) SendWithAttachments(to []string, subject, body string, attachments []Attachment) error {
	if !m.Enabled() {
		return nil
	}
	var rejected []RecipientError
	var envelope, header, typed []string
	for _, item := range to {
		addr, err := netmail.ParseAddress(item)
		if err != nil {
			rejected = append(rejected, RecipientError{item, err})
			continue
		}
		envelope = append(envelope, addr.Address)
		header = append(header, headerAddress(addr))
		typed = append(typed, item)
	}
	if len(envelope) == 0 {
		if len(rejected) > 0 {
			return &RecipientsError{Rejected: rejected}
		}
		return fmt.Errorf("mail: no recipient")
	}

	c, err := m.dial()
	if err != nil {
		return fmt.Errorf("mail: %w", err)
	}
	defer func() { _ = c.Close() }()

	if err := c.Mail(m.cfg.From); err != nil {
		return fmt.Errorf("mail: sender %s refused: %w", m.cfg.From, err)
	}
	sent := 0
	for i, rcpt := range envelope {
		if err := c.Rcpt(rcpt); err != nil {
			rejected = append(rejected, RecipientError{typed[i], err})
			continue
		}
		sent++
	}
	if sent == 0 {
		return &RecipientsError{Rejected: rejected}
	}

	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("mail: send failed: %w", err)
	}
	if _, err := w.Write(buildMessage(m.cfg.From, strings.Join(header, ", "), subject, body, attachments...)); err != nil {
		return fmt.Errorf("mail: send failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mail: send failed: %w", err)
	}
	_ = c.Quit()

	if len(rejected) > 0 {
		return &RecipientsError{Rejected: rejected, Sent: sent}
	}
	return nil
}

// dial opens the SMTP session as smtp.SendMail does (STARTTLS when the server offers
// it, then AUTH when a user is set), with timeouts.
func (m *SMTPMailer) dial() (*smtp.Client, error) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(m.cfg.Host, m.cfg.Port), dialTimeout)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(sessionTimeout))
	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: m.cfg.Host}); err != nil {
			_ = c.Close()
			return nil, err
		}
	}
	if m.cfg.Username != "" {
		if ok, _ := c.Extension("AUTH"); !ok {
			_ = c.Close()
			return nil, errors.New("server does not support authentication")
		}
		if err := c.Auth(smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)); err != nil {
			_ = c.Close()
			return nil, err
		}
	}
	return c, nil
}

// buildMessage assembles a minimal RFC 5322 plain-text message — with attachments a
// multipart/mixed one: the text, then each file base64-encoded. Header values are
// stripped of CR/LF to prevent header injection from job-supplied subjects; the
// subject and file names are RFC 2047 encoded (æ, ø, å).
func buildMessage(from, to, subject, body string, attachments ...Attachment) []byte {
	var b strings.Builder
	b.WriteString("From: " + sanitizeHeader(from) + "\r\n")
	b.WriteString("To: " + sanitizeHeader(to) + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", sanitizeHeader(subject)) + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	text := strings.ReplaceAll(body, "\n", "\r\n")
	if len(attachments) == 0 {
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		b.WriteString("\r\n")
		b.WriteString(text)
		return []byte(b.String())
	}
	boundary := "openerp-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	b.WriteString("Content-Type: multipart/mixed; boundary=\"" + boundary + "\"\r\n")
	b.WriteString("\r\n")
	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	b.WriteString("\r\n")
	b.WriteString(text + "\r\n")
	for _, a := range attachments {
		contentType := a.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		name := mime.QEncoding.Encode("utf-8", sanitizeHeader(a.Name))
		b.WriteString("--" + boundary + "\r\n")
		b.WriteString("Content-Type: " + sanitizeHeader(contentType) + "; name=\"" + name + "\"\r\n")
		b.WriteString("Content-Transfer-Encoding: base64\r\n")
		b.WriteString("Content-Disposition: attachment; filename=\"" + name + "\"\r\n")
		b.WriteString("\r\n")
		encoded := base64.StdEncoding.EncodeToString(a.Data)
		for len(encoded) > 76 {
			b.WriteString(encoded[:76] + "\r\n")
			encoded = encoded[76:]
		}
		b.WriteString(encoded + "\r\n")
	}
	b.WriteString("--" + boundary + "--\r\n")
	return []byte(b.String())
}

// sanitizeHeader removes CR/LF so a value cannot inject additional headers.
func sanitizeHeader(v string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(v)
}
