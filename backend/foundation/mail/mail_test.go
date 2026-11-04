package mail

import (
	"bufio"
	"encoding/base64"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
)

func TestDisabledMailerIsNoop(t *testing.T) {
	m := NewSMTPMailer(Config{Enabled: false, Host: "smtp.example.com", From: "a@b.com"})
	if m.Enabled() {
		t.Fatal("mailer should be disabled")
	}
	if err := m.Send([]string{"x@y.com"}, "subj", "body"); err != nil {
		t.Fatalf("disabled Send should be a no-op, got %v", err)
	}
}

func TestEnabledRequiresHostAndFrom(t *testing.T) {
	if NewSMTPMailer(Config{Enabled: true, From: "a@b.com"}).Enabled() {
		t.Error("missing host should not be enabled")
	}
	if NewSMTPMailer(Config{Enabled: true, Host: "h"}).Enabled() {
		t.Error("missing from should not be enabled")
	}
	if !NewSMTPMailer(Config{Enabled: true, Host: "h", From: "a@b.com"}).Enabled() {
		t.Error("host+from+enabled should be enabled")
	}
}

func TestBuildMessageSanitizesHeaders(t *testing.T) {
	// A subject with CRLF must not be able to inject an extra header line.
	msg := string(buildMessage("from@x.com", "to@x.com", "hi\r\nBcc: evil@x.com", "body"))
	if strings.Contains(msg, "\r\nBcc:") {
		t.Errorf("header injection not prevented (Bcc appears as a header line):\n%s", msg)
	}
	if !strings.Contains(msg, "Subject: hiBcc: evil@x.com") {
		t.Errorf("subject not sanitized as expected:\n%s", msg)
	}
	if !strings.HasSuffix(msg, "\r\nbody") {
		t.Errorf("body not appended correctly:\n%s", msg)
	}
}

func TestBuildMessageEncodesSubject(t *testing.T) {
	msg := string(buildMessage("from@x.com", "to@x.com", "Jobbkø feilet", "Kø"))
	if !strings.Contains(msg, "Subject: =?utf-8?q?Jobbk=C3=B8_feilet?=\r\n") {
		t.Errorf("subject not RFC 2047 encoded:\n%s", msg)
	}
	if !strings.HasSuffix(msg, "\r\n\r\nKø") {
		t.Errorf("body not kept as UTF-8:\n%s", msg)
	}
}

func TestParseAddressList(t *testing.T) {
	items, err := ParseAddressList(" ops@example.com;hans@example.com , Bjørn <bjorn@example.com>; ")
	if err != nil {
		t.Fatal(err)
	}
	if got := JoinAddressList(items); got != "ops@example.com; hans@example.com; Bjørn <bjorn@example.com>" {
		t.Errorf("items = %q", got)
	}
	if items, err := ParseAddressList("  ; "); err != nil || len(items) != 0 {
		t.Errorf("blank list = %v, %v, want no items", items, err)
	}

	for _, tc := range []struct {
		list, bad string
		duplicate bool
	}{
		{"ops@example.com; not-an-address", "not-an-address", false},
		{"ops@example.com; ops@", "ops@", false},
		{"ops@example.com; Hans <OPS@example.com>", "OPS@example.com", true},
	} {
		_, err := ParseAddressList(tc.list)
		var listErr *AddressListError
		if !errors.As(err, &listErr) || listErr.Address != tc.bad || listErr.Duplicate != tc.duplicate {
			t.Errorf("%q: error %v, want %q (duplicate %v)", tc.list, err, tc.bad, tc.duplicate)
		}
	}
}

// fakeSMTP is an SMTP server that accepts every recipient except those in reject.
type fakeSMTP struct {
	addr   string
	reject map[string]bool

	mu   sync.Mutex
	rcpt []string
	data string
}

func startFakeSMTP(t *testing.T, reject ...string) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	s := &fakeSMTP{addr: ln.Addr().String(), reject: map[string]bool{}}
	for _, r := range reject {
		s.reject[r] = true
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(conn)
		}
	}()
	return s
}

func (s *fakeSMTP) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	r := bufio.NewReader(conn)
	reply := func(line string) { _, _ = conn.Write([]byte(line + "\r\n")) }
	reply("220 fake ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		cmd := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			reply("250 fake")
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			reply("250 OK")
		case strings.HasPrefix(cmd, "RCPT TO:"):
			rcpt := strings.Trim(line[len("RCPT TO:"):], "<>")
			if s.reject[rcpt] {
				reply("550 no such user")
				continue
			}
			s.mu.Lock()
			s.rcpt = append(s.rcpt, rcpt)
			s.mu.Unlock()
			reply("250 OK")
		case cmd == "DATA":
			reply("354 go ahead")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				b.WriteString(l)
			}
			s.mu.Lock()
			s.data = b.String()
			s.mu.Unlock()
			reply("250 queued")
		case cmd == "QUIT":
			reply("221 bye")
			return
		default:
			reply("250 OK")
		}
	}
}

func (s *fakeSMTP) mailer(t *testing.T) *SMTPMailer {
	host, port, err := net.SplitHostPort(s.addr)
	if err != nil {
		t.Fatal(err)
	}
	return NewSMTPMailer(Config{Enabled: true, Host: host, Port: port, From: "jobs@example.com"})
}

func TestSendToAllRecipients(t *testing.T) {
	s := startFakeSMTP(t)
	err := s.mailer(t).Send([]string{"ops@example.com", "Bjørn <bjorn@example.com>"}, "Job failed", "Details")
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.Join(s.rcpt, ",") != "ops@example.com,bjorn@example.com" {
		t.Errorf("recipients = %v", s.rcpt)
	}
	if !strings.Contains(s.data, "To: ops@example.com, =?utf-8?q?Bj=C3=B8rn?= <bjorn@example.com>\r\n") {
		t.Errorf("To header missing all recipients:\n%s", s.data)
	}
}

func TestSendRejectedRecipientDoesNotStopOthers(t *testing.T) {
	s := startFakeSMTP(t, "gone@example.com")
	err := s.mailer(t).Send([]string{"gone@example.com", "not an address", "ops@example.com"}, "Job failed", "Details")
	var rcptErr *RecipientsError
	if !errors.As(err, &rcptErr) {
		t.Fatalf("error = %v, want *RecipientsError", err)
	}
	if rcptErr.Sent != 1 || len(rcptErr.Rejected) != 2 {
		t.Errorf("sent %d, rejected %v; want 1 sent, 2 rejected", rcptErr.Sent, rcptErr.Rejected)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.Join(s.rcpt, ",") != "ops@example.com" || s.data == "" {
		t.Errorf("recipients = %v, message sent %v; want the message sent to ops@example.com", s.rcpt, s.data != "")
	}
}

func TestSendAllRecipientsRejected(t *testing.T) {
	s := startFakeSMTP(t, "gone@example.com")
	err := s.mailer(t).Send([]string{"gone@example.com"}, "Job failed", "Details")
	var rcptErr *RecipientsError
	if !errors.As(err, &rcptErr) || rcptErr.Sent != 0 {
		t.Fatalf("error = %v, want *RecipientsError with nothing sent", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data != "" {
		t.Error("message sent although every recipient was rejected")
	}
}

// With attachments the message is multipart/mixed: the text, then each file base64-encoded
// with its (encoded) name
func TestBuildMessageWithAttachment(t *testing.T) {
	data := []byte("%PDF-1.4 a printout")
	msg := string(buildMessage("from@x.com", "to@x.com", "Report", "See the attachment", Attachment{Name: "Kundeliste.pdf", ContentType: "application/pdf", Data: data}))
	for _, want := range []string{"Content-Type: multipart/mixed; boundary=", "Content-Type: text/plain; charset=UTF-8", "See the attachment",
		"Content-Type: application/pdf; name=\"Kundeliste.pdf\"", "Content-Disposition: attachment; filename=\"Kundeliste.pdf\"", base64.StdEncoding.EncodeToString(data)} {
		if !strings.Contains(msg, want) {
			t.Errorf("message without %q:\n%s", want, msg)
		}
	}
	boundary := msg[strings.Index(msg, "boundary=\"")+10:]
	boundary = boundary[:strings.Index(boundary, "\"")]
	if !strings.HasSuffix(msg, "--"+boundary+"--\r\n") {
		t.Errorf("multipart not closed:\n%s", msg)
	}
	plain := string(buildMessage("from@x.com", "to@x.com", "Report", "text"))
	if strings.Contains(plain, "multipart") {
		t.Error("a message without attachments is multipart")
	}
}
