package middleware

import (
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// CORSOrigins: the other origins (scheme://host[:port], commas) whose pages may call the API
// with the user's session — CORS_ORIGINS, none by default: the app's pages are served from the
// same origin as the API (nginx, the frontend's /api proxy), so no other site needs it (S26)
func CORSOrigins() []string {
	var origins []string
	for _, origin := range strings.Split(os.Getenv("CORS_ORIGINS"), ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" && origin != "*" {
			origins = append(origins, origin)
		}
	}
	return origins
}

// CORS answers other origins' requests: only those of CORSOrigins, with credentials; without
// any no cross-origin answers at all (the browser then refuses another site's scripts the
// answer)
func CORS(origins []string) fiber.Handler {
	if len(origins) == 0 {
		return func(c *fiber.Ctx) error { return c.Next() }
	}
	return cors.New(cors.Config{
		AllowOrigins:     strings.Join(origins, ","),
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Company,X-App-Answers",
		ExposeHeaders:    "X-App-Notes,X-App-Question",
		AllowCredentials: true,
		MaxAge:           86400, // 24 hours
	})
}
