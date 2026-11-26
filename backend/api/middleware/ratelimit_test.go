package middleware

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestAcceptedLanguage(t *testing.T) {
	supported := []string{"en-US", "nb-NO", "da-DK"}
	cases := map[string]string{
		"":                        "",
		"nb-NO,nb;q=0.9,en;q=0.8": "nb-NO",
		"de-DE,da;q=0.9":          "da-DK",
		"no":                      "nb-NO",
		"en-GB,en;q=0.9":          "en-US",
		"fr-FR":                   "",
		"en;q=0.5, da-DK;q=0.9":   "da-DK",
		"*":                       "",
		"da-dk":                   "da-DK",
	}
	for header, want := range cases {
		got := acceptedLanguage(header, append([]string(nil), supported...))
		if got != want {
			t.Errorf("acceptedLanguage(%q) = %q, want %q", header, got, want)
		}
	}
}

func TestAddressKey(t *testing.T) {
	for address, want := range map[string]string{
		"192.0.2.10":                 "192.0.2.10",
		"::ffff:192.0.2.10":          "192.0.2.10",
		"2001:db8:1:2:aaaa::1":       "2001:db8:1:2::/64",
		"2001:db8:1:2:ffff:ffff::99": "2001:db8:1:2::/64",
		"2001:db8:1:3::1":            "2001:db8:1:3::/64",
		"not an address":             "not an address",
	} {
		if got := AddressKey(address); got != want {
			t.Errorf("AddressKey(%q) = %q, want %q", address, got, want)
		}
	}
}

// limitTestApp: an app that reads the client's address from X-Forwarded-For, with the limiter
func limitTestApp(handler fiber.Handler, status int) *fiber.App {
	app := fiber.New(fiber.Config{ProxyHeader: fiber.HeaderXForwardedFor})
	app.Use(handler)
	app.All("/*", func(c *fiber.Ctx) error { return c.SendStatus(status) })
	return app
}

func limitedRequest(t *testing.T, app *fiber.App, address, cookie string) int {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/x", nil)
	req.Header.Set(fiber.HeaderXForwardedFor, address)
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

// The global limit (S35): logged-in users behind one address each have their own budget;
// requests without a session share the address's, and IPv6 addresses of one /64 count together
func TestRateLimitPerUserAndNetwork(t *testing.T) {
	config := JWTConfig{SecretKey: []byte("a test secret of some length"), CookieName: "openerp_session", TokenExpiry: time.Hour}
	cookieOf := func(user string) string {
		token, err := GenerateToken(config, user, user, "main", "en-US", "", time.Now())
		if err != nil {
			t.Fatal(err)
		}
		return config.CookieName + "=" + token
	}
	app := limitTestApp(RateLimit(config), fiber.StatusOK)
	office := "192.0.2.10"
	for i := 0; i < 300; i++ {
		if got := limitedRequest(t, app, office, cookieOf("KARI")); got != fiber.StatusOK {
			t.Fatalf("request %d of KARI: %d", i+1, got)
		}
	}
	if got := limitedRequest(t, app, office, cookieOf("KARI")); got != fiber.StatusTooManyRequests {
		t.Errorf("KARI's request 301: %d, want 429", got)
	}
	if got := limitedRequest(t, app, office, cookieOf("OLE")); got != fiber.StatusOK {
		t.Errorf("OLE behind the same address: %d, want his own budget", got)
	}
	if got := limitedRequest(t, app, office, ""); got != fiber.StatusOK {
		t.Errorf("a request without a session from the address: %d", got)
	}
	// a forged cookie counts for the address
	if got := limitedRequest(t, app, "192.0.2.11", config.CookieName+"=forged"); got != fiber.StatusOK {
		t.Errorf("forged cookie: %d", got)
	}

	// one /64: a new address per request does not give a new budget
	for i := 0; i < 300; i++ {
		limitedRequest(t, app, fmt.Sprintf("2001:db8:5:6::%x", i+1), "")
	}
	if got := limitedRequest(t, app, "2001:db8:5:6:abcd::1", ""); got != fiber.StatusTooManyRequests {
		t.Errorf("another address of the same /64: %d, want 429", got)
	}
	if got := limitedRequest(t, app, "2001:db8:5:7::1", ""); got != fiber.StatusOK {
		t.Errorf("another /64: %d", got)
	}
}

// The login limit counts failed logins only: an office's successful logins never fill it
func TestLoginRateLimitCountsFailures(t *testing.T) {
	ok := limitTestApp(LoginRateLimit(), fiber.StatusOK)
	for i := 0; i < 30; i++ {
		if got := limitedRequest(t, ok, "192.0.2.20", ""); got != fiber.StatusOK {
			t.Fatalf("successful login %d: %d", i+1, got)
		}
	}
	failing := limitTestApp(LoginRateLimit(), fiber.StatusUnauthorized)
	for i := 0; i < 10; i++ {
		limitedRequest(t, failing, fmt.Sprintf("2001:db8:9:9::%x", i+1), "")
	}
	if got := limitedRequest(t, failing, "2001:db8:9:9::ff", ""); got != fiber.StatusTooManyRequests {
		t.Errorf("failure 11 from the same /64: %d, want 429", got)
	}
}
