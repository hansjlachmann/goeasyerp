package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func securityTestApp(allowed []string) *fiber.App {
	app := fiber.New()
	app.Use(SecurityHeaders())
	app.Use(SameOriginWrites(allowed))
	ok := func(c *fiber.Ctx) error { return c.SendString("ok") }
	app.Get("/api/x", ok)
	app.Post("/api/x", ok)
	app.Delete("/api/x", ok)
	return app
}

func TestSameOriginWrites(t *testing.T) {
	app := securityTestApp([]string{"https://other.example.com/"})
	cases := []struct {
		name, method, site, origin string
		want                       int
	}{
		{"the app itself", "POST", "same-origin", "https://erp.example.com", 200},
		{"typed by the user", "POST", "none", "", 200},
		{"another site's form", "POST", "cross-site", "https://evil.example.org", 403},
		{"a sibling subdomain", "DELETE", "same-site", "https://blog.example.com", 403},
		{"reading from another site", "GET", "cross-site", "https://evil.example.org", 200},
		{"an allowed origin", "POST", "cross-site", "https://other.example.com", 200},
		{"an old browser, same host", "POST", "", "https://erp.example.com:8443", 200},
		{"an old browser, another host", "POST", "", "https://evil.example.org", 403},
		{"an old browser, null origin", "POST", "", "null", 403},
		{"a program", "POST", "", "", 200},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, "/api/x", nil)
		req.Host = "erp.example.com"
		if tc.site != "" {
			req.Header.Set("Sec-Fetch-Site", tc.site)
		}
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != tc.want {
			t.Errorf("%s: status %d, want %d", tc.name, resp.StatusCode, tc.want)
		}
	}
}

func TestSecurityHeaders(t *testing.T) {
	resp, err := securityTestApp(nil).Test(httptest.NewRequest("GET", "/api/x", nil))
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"X-Content-Type-Options":       "nosniff",
		"X-Frame-Options":              "DENY",
		"Referrer-Policy":              "same-origin",
		"Cache-Control":                "no-store",
		"Cross-Origin-Resource-Policy": "same-origin",
	} {
		if got := resp.Header.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestCORSOnlyForConfiguredOrigins(t *testing.T) {
	for _, tc := range []struct {
		origins []string
		want    string
	}{{nil, ""}, {[]string{"https://other.example.com"}, "https://other.example.com"}} {
		app := fiber.New()
		app.Use(CORS(tc.origins))
		app.Get("/api/x", func(c *fiber.Ctx) error { return c.SendString("ok") })
		req := httptest.NewRequest("GET", "/api/x", nil)
		req.Header.Set("Origin", "https://other.example.com")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != tc.want {
			t.Errorf("origins %v: Access-Control-Allow-Origin %q, want %q", tc.origins, got, tc.want)
		}
	}
}
