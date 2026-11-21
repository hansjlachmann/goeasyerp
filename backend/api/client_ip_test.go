package api

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// X-Forwarded-For is believed only from a trusted proxy, so a client cannot give the rate
// limiters a new address with every request (S20). app.Test connects from 0.0.0.0; the
// addresses are documentation ones.
func TestClientIPFromTrustedProxyOnly(t *testing.T) {
	ip := func(trusted, forwarded string) string {
		t.Helper()
		app := fiber.New(appConfig(trusted))
		app.Get("/", func(c *fiber.Ctx) error { return c.SendString(c.IP()) })
		req := httptest.NewRequest("GET", "/", nil)
		if forwarded != "" {
			req.Header.Set("X-Forwarded-For", forwarded)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		return string(body)
	}

	if got := ip("", "203.0.113.7"); got != "0.0.0.0" {
		t.Errorf("a client that is no trusted proxy chose its address: %q", got)
	}
	if got := ip("192.0.2.1", "203.0.113.7"); got != "0.0.0.0" {
		t.Errorf("a client that is not the listed proxy chose its address: %q", got)
	}
	if got := ip("0.0.0.0", "203.0.113.7"); got != "203.0.113.7" {
		t.Errorf("the trusted proxy's client address: %q", got)
	}
	if got := ip("0.0.0.0/32, 192.0.2.1", "203.0.113.7, 198.51.100.2"); got != "203.0.113.7" {
		t.Errorf("a list of proxies and of addresses: %q, want the first address", got)
	}
	if got := ip("0.0.0.0", "not-an-address"); got != "0.0.0.0" {
		t.Errorf("an invalid header: %q, want the connection's address", got)
	}
	if got := ip("0.0.0.0", ""); got != "0.0.0.0" {
		t.Errorf("no header from the proxy: %q, want the connection's address", got)
	}
}

// TRUSTED_PROXIES: blank gives loopback and the private ranges
func TestTrustedProxies(t *testing.T) {
	if got := trustedProxies(" "); len(got) != len(defaultTrustedProxies) {
		t.Errorf("blank: %v", got)
	}
	if got := trustedProxies("192.0.2.1, 198.51.100.0/24,"); len(got) != 2 || got[1] != "198.51.100.0/24" {
		t.Errorf("list: %v", got)
	}
}
