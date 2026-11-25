package mail

import (
	"errors"
	"net"
	"testing"
	"time"
)

// allowLoopback lets a test's fake mail server on 127.0.0.1 through the address rule
func allowLoopback(t *testing.T) {
	t.Helper()
	saved := checkAddress
	checkAddress = func(ip net.IP) string {
		if ip.IsLoopback() {
			return ""
		}
		return saved(ip)
	}
	t.Cleanup(func() { checkAddress = saved })
}

// The server never connects to itself, the link-local network (cloud metadata), unspecified or
// multicast addresses; private addresses only when SMTP_BLOCK_PRIVATE is not set (S49)
func TestCheckAddress(t *testing.T) {
	for _, tc := range []struct {
		ip           string
		allowed      bool // with private addresses allowed (the default)
		blockPrivate bool // refused with SMTP_BLOCK_PRIVATE
	}{
		{"127.0.0.1", false, true},
		{"127.10.0.5", false, true},
		{"::1", false, true},
		{"0.0.0.0", false, true},
		{"0.1.2.3", false, true},
		{"::", false, true},
		{"169.254.169.254", false, true},
		{"fe80::1", false, true},
		{"224.0.0.1", false, true},
		{"ff02::1", false, true},
		{"::ffff:127.0.0.1", false, true},
		{"10.1.2.3", true, true},
		{"172.20.0.4", true, true},
		{"192.168.1.20", true, true},
		{"100.64.0.1", true, true},
		{"fd00::25", true, true},
		{"203.0.113.25", true, false},
		{"2001:db8::25", true, false},
	} {
		ip := net.ParseIP(tc.ip)
		if got := CheckAddress(ip, false) == ""; got != tc.allowed {
			t.Errorf("%s: allowed %v, want %v", tc.ip, got, tc.allowed)
		}
		if got := CheckAddress(ip, true) != ""; got != tc.blockPrivate {
			t.Errorf("%s with SMTP_BLOCK_PRIVATE: refused %v, want %v", tc.ip, got, tc.blockPrivate)
		}
	}
}

// A host entered as a refused address is refused at once; a name is checked when connecting
func TestCheckHost(t *testing.T) {
	t.Setenv(EnvBlockPrivate, "")
	if err := CheckHost("169.254.169.254"); err == nil {
		t.Error("the metadata address accepted")
	}
	if err := CheckHost("[::1]"); err == nil {
		t.Error("::1 accepted")
	}
	if err := CheckHost("192.168.1.20"); err != nil {
		t.Errorf("a private relay refused by default: %v", err)
	}
	t.Setenv(EnvBlockPrivate, "true")
	if err := CheckHost("192.168.1.20"); err == nil {
		t.Error("a private relay accepted with SMTP_BLOCK_PRIVATE")
	}
	if err := CheckHost("mail.example.com"); err != nil {
		t.Errorf("a name refused before connecting: %v", err)
	}
}

// Connecting refuses a name that resolves to loopback, without opening a connection
func TestDialRefusesLoopback(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	accepted := make(chan struct{}, 1)
	go func() {
		conn, err := l.Accept()
		if err == nil {
			accepted <- struct{}{}
			_ = conn.Close()
		}
	}()
	_, port, _ := net.SplitHostPort(l.Addr().String())
	_, err = dialChecked("localhost", port, 2*time.Second)
	var refused *AddressRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("localhost: %v, want an AddressRefusedError", err)
	}
	select {
	case <-accepted:
		t.Error("a connection was opened to the refused address")
	case <-time.After(100 * time.Millisecond):
	}
	mailer := NewSMTPMailer(Config{Enabled: true, Host: "127.0.0.1", Port: port, From: "jobs@example.com"})
	if err := mailer.Send([]string{"ops@example.com"}, "s", "b"); !errors.As(err, &refused) {
		t.Errorf("sending to 127.0.0.1: %v", err)
	}
}
