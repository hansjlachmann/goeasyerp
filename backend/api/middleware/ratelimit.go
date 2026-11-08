package middleware

import (
	"sort"
	"strconv"
	"strings"
	"time"

	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

// RateLimit returns a generous, global rate limiter applied to every request.
// It is keyed by client IP (c.IP()); when running behind a reverse proxy (nginx),
// the app's ProxyHeader must be set so c.IP() reflects the real client address.
// State is kept in-memory per process — for multi-instance deployments this would
// need a shared store (e.g. Redis).
//
// Health checks are skipped so liveness/readiness probes are never throttled.
func RateLimit() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          300,
		Expiration:   1 * time.Minute,
		LimitReached: rateLimitReached,
		Next: func(c *fiber.Ctx) bool {
			return c.Path() == "/health"
		},
	})
}

// LoginRateLimit returns a strict limiter for the login endpoint. Login runs a
// deliberately slow bcrypt password check, so an unbounded endpoint invites both
// brute-force/credential-stuffing and CPU exhaustion; capping attempts per IP
// blunts both.
func LoginRateLimit() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          10,
		Expiration:   1 * time.Minute,
		LimitReached: rateLimitReached,
	})
}

// ThemeInstallRateLimit returns a strict limiter for installing/refreshing themes: each
// request downloads up to 30 MB from GitHub and decodes the images.
func ThemeInstallRateLimit() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          5,
		Expiration:   1 * time.Minute,
		LimitReached: rateLimitReached,
	})
}

// rateLimitReached responds with 429 in the app's standard {success, error} envelope. The
// limiters run before the session is read, so the message is in the browser's language
// (Accept-Language), else the default language.
func rateLimitReached(c *fiber.Ctx) error {
	ts := i18n.GetInstance()
	language := acceptedLanguage(c.Get(fiber.HeaderAcceptLanguage), ts.GetSupportedLanguages())
	if language == "" {
		language = ts.GetDefaultLanguage()
	}
	return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
		"success": false,
		"error":   apperrors.TooManyRequests().Message(language),
	})
}

// acceptedLanguage returns the first language of an Accept-Language header that is supported
// (nb-NO, or nb for nb-NO), "" when none is (the default language)
func acceptedLanguage(header string, supported []string) string {
	type tag struct {
		name string
		q    float64
	}
	var tags []tag
	for _, part := range strings.Split(header, ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		q := 1.0
		if v, ok := strings.CutPrefix(strings.TrimSpace(params), "q="); ok {
			parsed, err := strconv.ParseFloat(v, 64)
			if err == nil {
				q = parsed
			}
		}
		if name != "" && name != "*" && q > 0 {
			tags = append(tags, tag{name, q})
		}
	}
	sort.SliceStable(tags, func(i, j int) bool { return tags[i].q > tags[j].q })
	sort.Strings(supported)
	for _, t := range tags {
		for _, lang := range supported {
			if strings.EqualFold(lang, t.name) {
				return lang
			}
		}
		primary, _, _ := strings.Cut(t.name, "-")
		if strings.EqualFold(primary, "no") || strings.EqualFold(primary, "nn") {
			primary = "nb" // Norwegian: Bokmål
		}
		for _, lang := range supported {
			langPrimary, _, _ := strings.Cut(lang, "-")
			if strings.EqualFold(langPrimary, primary) {
				return lang
			}
		}
	}
	return ""
}
