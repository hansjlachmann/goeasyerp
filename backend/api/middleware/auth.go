package middleware

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
)

// Claims represents the JWT claims for a user session
type Claims struct {
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`
	Company  string `json:"company"`
	Language string `json:"language"`
	Menu     string `json:"menu"`
	// LoginAt: when the user logged in to the company (Unix seconds) — kept when the token is
	// re-issued for the same company, for the user time register at log out
	LoginAt int64 `json:"login_at,omitempty"`
	jwt.RegisteredClaims
}

// LoginTime is when the user logged in to the token's company (tokens without login_at: when
// the token was issued)
func (c *Claims) LoginTime() time.Time {
	if c.LoginAt != 0 {
		return time.Unix(c.LoginAt, 0)
	}
	if c.IssuedAt != nil {
		return c.IssuedAt.Time
	}
	return time.Time{}
}

// PermissionLoader loads permissions for a user in a company.
// Returns (hasRoles, isSuper, permissions).
type PermissionLoader func(db *sql.DB, dbType database.DBType, userID, company string) (bool, bool, map[string]session.TablePermission)

// JWTConfig holds JWT authentication configuration
type JWTConfig struct {
	SecretKey    []byte
	CookieName   string
	TokenExpiry  time.Duration
	SecureCookie bool
}

// DefaultJWTConfig returns a JWTConfig populated from environment variables
func DefaultJWTConfig() JWTConfig {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		// Generate random secret for dev — log warning
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			log.Fatal("Failed to generate random JWT secret:", err)
		}
		secret = hex.EncodeToString(b)
		log.Println("WARNING: No JWT_SECRET set — using random key. Sessions will not survive restarts.")
	}

	// SecureCookie when DB_HOST is set (production/Docker = PostgreSQL = HTTPS)
	secureCookie := os.Getenv("DB_HOST") != ""

	return JWTConfig{
		SecretKey:    []byte(secret),
		CookieName:   "openerp_session",
		TokenExpiry:  24 * time.Hour,
		SecureCookie: secureCookie,
	}
}

// GenerateToken creates a signed JWT with user claims; loginAt is when the user logged in to the
// company (zero: now)
func GenerateToken(config JWTConfig, userID, userName, company, language, menu string, loginAt time.Time) (string, error) {
	now := time.Now()
	if loginAt.IsZero() {
		loginAt = now
	}
	claims := &Claims{
		UserID:   userID,
		UserName: userName,
		Company:  company,
		Language: language,
		Menu:     menu,
		LoginAt:  loginAt.Unix(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(config.TokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(config.SecretKey)
}

// ParseToken validates a JWT string and returns the claims
func ParseToken(config JWTConfig, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return config.SecretKey, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

// SessionChangedHeader is set on every response that re-issues the session cookie.
const SessionChangedHeader = "X-Session-Changed"

// SetAuthCookie sets the JWT as an HTTP-only cookie
func SetAuthCookie(c *fiber.Ctx, config JWTConfig, token string) {
	// The session (company, user) of this response differs from the request's: the frontend
	// reloads its session state (e.g. the menu bar after renaming the current company)
	c.Set(SessionChangedHeader, "1")
	c.Cookie(&fiber.Cookie{
		Name:     config.CookieName,
		Value:    token,
		Path:     "/",
		HTTPOnly: true,
		Secure:   config.SecureCookie,
		SameSite: "Lax",
		MaxAge:   int(config.TokenExpiry.Seconds()),
	})
}

// ClearAuthCookie removes the auth cookie
func ClearAuthCookie(c *fiber.Ctx, config JWTConfig) {
	c.Cookie(&fiber.Cookie{
		Name:     config.CookieName,
		Value:    "",
		Path:     "/",
		HTTPOnly: true,
		Secure:   config.SecureCookie,
		SameSite: "Lax",
		MaxAge:   -1,
	})
}

// The company of a browser tab: each tab keeps its company and sends it with every request
// (the header; the query parameter where a request cannot carry one, e.g. an image), so tabs
// in different companies do not move each other. Without it the request runs in the cookie's
// company — the one the user chose last, where a new tab starts.
const (
	TabCompanyHeader = "X-Company"
	TabCompanyParam  = "company"
	// TabCompanyUnavailableHeader marks the answer to a tab whose company is gone or no longer
	// allowed (409): the tab goes back to the cookie's company — never runs elsewhere silently
	TabCompanyUnavailableHeader = "X-Tab-Company"
)

// CompanyAccess reports whether the user may work in the company (role memberships)
type CompanyAccess func(userID, company string) bool

// tabCompany is the company the request's tab sends ("" none). A copy: Fiber's header and
// query strings point into the request's buffer, which the next request reuses — the session
// (cached) keeps this one.
func tabCompany(c *fiber.Ctx) string {
	company := strings.TrimSpace(c.Get(TabCompanyHeader))
	if company == "" {
		company = strings.TrimSpace(c.Query(TabCompanyParam))
	}
	return strings.Clone(company)
}

// AuthMiddleware reads the JWT cookie, loads/creates a session, and stores it in c.Locals("session").
// The session's company is the tab's when it sends one (checked with access the first time),
// else the token's.
func AuthMiddleware(config JWTConfig, db *sql.DB, dbType database.DBType, cache *SessionCache, permLoader PermissionLoader, access CompanyAccess) fiber.Handler {
	return func(c *fiber.Ctx) error {
		cookie := c.Cookies(config.CookieName)
		if cookie == "" {
			// No cookie — unauthenticated request
			c.Locals("session", (*session.Session)(nil))
			return c.Next()
		}

		claims, err := ParseToken(config, cookie)
		if err != nil {
			// Invalid/expired token — clear cookie and continue unauthenticated
			ClearAuthCookie(c, config)
			c.Locals("session", (*session.Session)(nil))
			return c.Next()
		}

		company, loginTime := claims.Company, claims.LoginTime()
		tab := tabCompany(c)
		if tab != "" && tab != claims.Company {
			// the tab works in another company: its own session, from now on
			company, loginTime = tab, time.Now()
		}

		// Try cache first (a cached session's company was checked when it was made)
		cacheKey := claims.UserID + ":" + company
		if cached := cache.Get(cacheKey); cached != nil {
			c.Locals("session", cached)
			return c.Next()
		}
		if company != claims.Company {
			if !companyExists(db, company) || access != nil && !access(claims.UserID, company) {
				c.Set(TabCompanyUnavailableHeader, "unavailable")
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{
					"success": false,
					"error":   apperrors.CompanyAccessDenied(company).Message(claims.Language),
				})
			}
		} else if company != "" && !companyExists(db, company) {
			// The token's company no longer exists (renamed or deleted by someone else): the
			// session would read tables that are gone — continue unauthenticated (log in again)
			ClearAuthCookie(c, config)
			c.Locals("session", (*session.Session)(nil))
			return c.Next()
		}

		// Build session from claims + DB reference
		dbWrapper := database.WrapConnection(db, dbType)
		sess := session.NewSession(dbWrapper, company, nil)
		sess.SetUser(claims.UserID, claims.UserName, claims.Language, claims.Menu)
		sess.SetLoginTime(loginTime)

		// Load permissions (e.g., after server restart when cache is empty)
		if permLoader != nil {
			hasRoles, isSuper, perms := permLoader(db, dbType, claims.UserID, company)
			sess.SetPermissions(hasRoles, isSuper, perms)
		}

		// Cache it
		cache.Set(cacheKey, sess, config.TokenExpiry)

		c.Locals("session", sess)
		return c.Next()
	}
}

// companyExists reports whether a Company record named name exists. On a database error it
// answers true, so a short outage does not log everybody out.
func companyExists(db *sql.DB, name string) bool {
	var found int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "Company" WHERE name = $1`, name).Scan(&found); err != nil {
		return true
	}
	return found > 0
}
