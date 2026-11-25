package api

import (
	"database/sql"
	"fmt"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/hansjlachmann/goeasyerp/backend/api/handlers"
	"github.com/hansjlachmann/goeasyerp/backend/api/middleware"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
)

// Version is set at build time via -ldflags
var Version = "dev"

func init() {
	// Allow override via environment variable
	if v := os.Getenv("APP_VERSION"); v != "" {
		Version = v
	}
}

// Server represents the API server
type Server struct {
	app          *fiber.App
	db           *sql.DB
	dbType       database.DBType
	companyInit  handlers.CompanyInitializer
	jwtConfig    middleware.JWTConfig
	sessionCache *middleware.SessionCache
}

// NewServer creates a new API server (defaults to SQLite)
func NewServer(db *sql.DB) *Server {
	return NewServerWithDBType(db, database.DBTypeSQLite)
}

// NewServerWithDBType creates a new API server with explicit database type
func NewServerWithDBType(db *sql.DB, dbType database.DBType) *Server {
	return NewServerFull(db, dbType, nil)
}

// defaultTrustedProxies: loopback and the private ranges (Docker and Kubernetes networks), where
// nginx or an ingress runs
var defaultTrustedProxies = []string{"127.0.0.1", "::1", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"}

// trustedProxies reads TRUSTED_PROXIES: addresses or CIDR ranges separated by commas; blank:
// defaultTrustedProxies
func trustedProxies(value string) []string {
	var proxies []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			proxies = append(proxies, part)
		}
	}
	if len(proxies) == 0 {
		return defaultTrustedProxies
	}
	return proxies
}

// appConfig is the Fiber configuration of the API server (trusted: TRUSTED_PROXIES)
func appConfig(trusted string) fiber.Config {
	return fiber.Config{
		AppName:      "GoEasyERP API v1.0",
		ServerHeader: "GoEasyERP",
		ErrorHandler: customErrorHandler,
		// c.IP() (the rate limiters' key) is the client's address: X-Forwarded-For is read only
		// on connections from a trusted proxy (nginx, which overwrites the header with the
		// address it sees) — from any other address the header is ignored, so a client cannot
		// pick a new address for every request. TRUSTED_PROXIES lists the proxies (addresses or
		// CIDR ranges, commas); default loopback and the private ranges of Docker and Kubernetes
		// networks. The first valid address of the header is taken.
		ProxyHeader:             fiber.HeaderXForwardedFor,
		EnableTrustedProxyCheck: true,
		TrustedProxies:          trustedProxies(trusted),
		EnableIPValidation:      true,
		// Request size and time (S34): the largest upload is a picture (6 MB) or an app package
		// (2 MB, base64 in JSON); nginx allows the same (client_max_body_size 8m). A request must
		// arrive within ReadTimeout and an idle keep-alive connection closes after IdleTimeout.
		// No WriteTimeout: a job's progress stream (/api/jobs/:id/events) stays open while the
		// job runs.
		BodyLimit:   maxRequestBody,
		ReadTimeout: 2 * time.Minute,
		IdleTimeout: 2 * time.Minute,
	}
}

// maxRequestBody: the largest request body the API takes
const maxRequestBody = 8 << 20

// NewServerFull creates a new API server with company initializer support
func NewServerFull(db *sql.DB, dbType database.DBType, companyInit handlers.CompanyInitializer) *Server {
	app := fiber.New(appConfig(os.Getenv("TRUSTED_PROXIES")))

	jwtConfig := middleware.DefaultJWTConfig()
	sessionCache := middleware.NewSessionCache()

	return &Server{
		app:          app,
		db:           db,
		dbType:       dbType,
		companyInit:  companyInit,
		jwtConfig:    jwtConfig,
		sessionCache: sessionCache,
	}
}

// Setup configures all routes and middleware
func (s *Server) Setup() {
	// Global middleware
	s.app.Use(recover.New()) // Panic recovery
	corsOrigins := middleware.CORSOrigins()
	s.app.Use(middleware.CORS(corsOrigins))
	s.app.Use(middleware.SecurityHeaders())             // nosniff, no framing, no-store (S25, S27)
	s.app.Use(middleware.SecurityEvents())              // every 403 answer is a security event (S48)
	s.app.Use(middleware.SameOriginWrites(corsOrigins)) // no changes sent by other web sites (S26)
	s.app.Use(middleware.RateLimit())                   // Generous global rate limit (per client IP)
	s.app.Use(middleware.Logger())
	// Create a permission loader for the auth middleware (used on cache miss / server restart)
	permLoader := handlers.NewPermissionLoader(s.db, s.dbType)
	// a token stays good while its user exists, is active and has not been logged out since
	s.sessionCache.SetUserCheck(handlers.NewUserCheck(s.db, s.dbType))
	// each browser tab may work in its own company (X-Company): checked like a company switch
	s.app.Use(middleware.AuthMiddleware(s.jwtConfig, s.db, s.dbType, s.sessionCache, permLoader, handlers.NewCompanyAccess(s.db, s.dbType)))

	// API routes; every request runs in the user's context (company: the dimension names in the
	// captions, user: work date and formats)
	api := s.app.Group("/api", middleware.UserContext())

	// Initialize handlers
	sessionHandler := handlers.NewSessionHandler()
	tablesHandler := handlers.NewTablesHandlerFull(s.db, s.dbType, s.companyInit)
	pagesHandler := handlers.NewPagesHandler()
	preferencesHandler := handlers.NewPreferencesHandlerWithDBType(s.db, s.dbType)
	themesHandler := handlers.NewThemesHandler(s.db, s.dbType)
	authHandler := handlers.NewAuthHandlerFull(s.db, s.dbType, s.companyInit, s.jwtConfig, s.sessionCache)
	handlers.InitSetupToken(s.db) // S37: the first user needs it
	codeunitsHandler := handlers.NewCodeunitsHandlerWithDBType(s.db, s.dbType)
	tablesHandler.OnCompanyChanged(authHandler.CompanyChanged) // renamed/deleted company: sessions follow

	// Translation routes
	translationsHandler := handlers.NewTranslationsHandler()
	api.Get("/translations", translationsHandler.GetTranslations)

	// Auth routes
	api.Post("/auth/login", middleware.LoginRateLimit(), authHandler.Login) // Strict limit: brute-force protection
	api.Post("/auth/logout", authHandler.Logout)
	api.Post("/auth/password", authHandler.ChangePassword) // the user's own password (S38)
	api.Get("/auth/user", authHandler.GetCurrentUser)
	api.Post("/auth/init", authHandler.CreateInitialUser) // the setup token is checked by the login guard (wrong ones lock the address)
	api.Get("/auth/companies", authHandler.ListCompanies)
	api.Post("/auth/companies", authHandler.CreateCompany)
	api.Post("/auth/language", authHandler.SetLanguage)
	api.Get("/auth/languages", authHandler.GetLanguages)
	api.Post("/auth/company", authHandler.SetCompany)

	// Session routes
	api.Get("/session", sessionHandler.GetSession)

	// Table routes
	// UserContext: table triggers see the user (USERID, execute permissions)
	tables := api.Group("/tables/:table", middleware.PermissionCheck(), middleware.UserContext())
	tables.Get("/ids", tablesHandler.GetRecordIDs)         // Lightweight IDs-only endpoint
	tables.Get("/options", tablesHandler.GetOptions)       // Fast options metadata only
	tables.Get("/lookup/:field", tablesHandler.LookupRows) // On-demand dropdown rows (large related tables)
	tables.Get("/list", tablesHandler.ListRecords)
	tables.Get("/card/:id", tablesHandler.GetRecord)
	tables.Post("/insert", tablesHandler.InsertRecord)
	tables.Post("/insert-many", tablesHandler.InsertRecords) // list paste (insert permission)
	tables.Post("/copy", tablesHandler.CopyRecords)          // list copy (read permission)
	tables.Put("/modify/:id", tablesHandler.ModifyRecord)
	tables.Delete("/delete-many", tablesHandler.DeleteRecords) // list multi-select (DELETE: delete permission)
	tables.Delete("/delete/:id", tablesHandler.DeleteRecord)
	tables.Post("/validate", tablesHandler.ValidateField)
	tables.Post("/init", tablesHandler.InitRecord)
	// Pictures (BLOB fields): served, uploaded (re-encoded) and removed one record at a time
	tables.Get("/blob/:field/:id", tablesHandler.GetBlob)
	tables.Put("/blob/:field/:id", tablesHandler.PutBlob)
	tables.Delete("/blob/:field/:id", tablesHandler.DeleteBlob)
	// the picture of a record with a blank key (a setup table: Company Information's logo)
	tables.Get("/blob/:field", tablesHandler.GetBlob)
	tables.Put("/blob/:field", tablesHandler.PutBlob)
	tables.Delete("/blob/:field", tablesHandler.DeleteBlob)
	// No-id variants for setup tables (single record with a blank primary key)
	tables.Get("/card", tablesHandler.GetRecord)
	tables.Put("/modify", tablesHandler.ModifyRecord)
	tables.Delete("/delete", tablesHandler.DeleteRecord)

	// Page routes
	api.Get("/pages", pagesHandler.GetAllPages)
	api.Get("/pages/:id", pagesHandler.GetPage)
	api.Post("/pages/:id/drilldown", handlers.PageDrilldown)                                               // a FlowField's drilldown: its page and filters
	api.Post("/pages/:id/field-states", handlers.NewPageTriggersHandler(s.db, s.dbType).FieldStates)       // page triggers (OnAfterGetRecord)
	api.Post("/pages/:id/period-grid", handlers.NewPageTriggersHandler(s.db, s.dbType).PeriodGrid)         // a card section of periods (FlowFields per period)
	api.Post("/pages/:id/matrix", handlers.NewPageTriggersHandler(s.db, s.dbType).Matrix)                  // a card section of records against periods
	api.Post("/pages/:id/matrix/validate", handlers.NewPageTriggersHandler(s.db, s.dbType).MatrixValidate) // a value entered into a matrix cell
	api.Post("/pages/:id/open", handlers.NewPageTriggersHandler(s.db, s.dbType).OpenPage)                  // OnOpenPage: the page's filters
	api.Get("/menu", pagesHandler.GetMenu)

	// Preferences routes
	api.Get("/preferences/:page_id/:type", preferencesHandler.GetPreferences)
	api.Post("/preferences/:page_id/:type", preferencesHandler.SavePreference)
	api.Delete("/preferences/:page_id/:type/:name", preferencesHandler.DeletePreference)

	// Appearance: color themes and the user's personalization (theme per mode, wallpaper, font)
	api.Get("/themes", themesHandler.ListThemes)
	api.Post("/themes/install", middleware.ThemeInstallRateLimit(), themesHandler.InstallTheme) // downloads from GitHub
	api.Post("/themes/:code/refresh", middleware.ThemeInstallRateLimit(), themesHandler.RefreshTheme)
	api.Delete("/themes/:code", themesHandler.DeleteTheme)
	api.Get("/themes/:code/assets/:name", themesHandler.GetThemeAsset)
	mySettingsHandler := handlers.NewMySettingsHandler(s.db, s.dbType)
	api.Get("/my-settings", mySettingsHandler.GetMySettings)
	api.Put("/my-settings", mySettingsHandler.SaveMySettings)
	// the company's own text at the top of the main menu (every user, no table permission)
	mainMenuTextHandler := handlers.NewMainMenuTextHandler(s.db, s.dbType)
	api.Get("/main-menu-text", mainMenuTextHandler.GetMainMenuText)
	// the code that runs for a user gets the user's work date and formats
	fcodeunits.RegionalLoader = handlers.RegionalLoader(s.db, s.dbType)
	// the captions of the dimension fields use the company's names of its dimensions
	fcodeunits.DimensionLabelsLoader = handlers.DimensionLabelsLoader(s.db, s.dbType)
	api.Get("/appearance", themesHandler.GetAppearance)
	api.Put("/appearance", themesHandler.SaveAppearance)

	// Codeunit routes (generic handler)
	api.Post("/codeunits/run", codeunitsHandler.RunCodeunit)

	// Jobs routes (codeunits with progress)
	jobsHandler := handlers.NewJobsHandlerWithDBType(s.db, s.dbType)
	api.Post("/jobs/start", jobsHandler.StartJob)
	api.Get("/jobs/:id/events", jobsHandler.GetJobEvents)
	api.Post("/jobs/:id/confirm", jobsHandler.RespondToConfirm)
	api.Post("/jobs/:id/cancel", jobsHandler.CancelJob)

	// Health check
	s.app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "goeasyerp-api",
		})
	})

	// Version endpoint
	s.app.Get("/api/version", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"version": Version,
		})
	})

	// 404 handler
	s.app.Use(func(c *fiber.Ctx) error {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"error":   "Route not found",
		})
	})
}

// Start starts the API server on the specified port (int)
func (s *Server) Start(port int) error {
	return s.StartOnPort(fmt.Sprintf("%d", port))
}

// StartOnPort starts the API server on the specified port (string)
// Binds to 0.0.0.0 for Docker compatibility
func (s *Server) StartOnPort(port string) error {
	addr := fmt.Sprintf("0.0.0.0:%s", port)
	log.Printf("🚀 API Server starting on http://localhost:%s\n", port)
	log.Printf("📡 Health check: http://localhost:%s/health\n", port)
	log.Printf("📚 API base: http://localhost:%s/api\n", port)
	return s.app.Listen(addr)
}

// StartOnListener serves on a socket the caller opened (or one handed over by the program that
// started this one, appbundle.Listen)
func (s *Server) StartOnListener(ln net.Listener) error {
	log.Printf("🚀 API Server starting on %s\n", ln.Addr())
	return s.app.Listener(ln)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	return s.app.Shutdown()
}

// customErrorHandler handles fiber errors
func customErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	return c.Status(code).JSON(fiber.Map{
		"success": false,
		"error":   err.Error(),
	})
}
