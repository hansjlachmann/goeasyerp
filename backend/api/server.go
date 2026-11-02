package api

import (
	"database/sql"
	"fmt"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/hansjlachmann/openerp/backend/api/handlers"
	"github.com/hansjlachmann/openerp/backend/api/middleware"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
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

// NewServerFull creates a new API server with company initializer support
func NewServerFull(db *sql.DB, dbType database.DBType, companyInit handlers.CompanyInitializer) *Server {
	app := fiber.New(fiber.Config{
		AppName:      "OpenERP API v1.0",
		ServerHeader: "OpenERP",
		ErrorHandler: customErrorHandler,
		// Trust the reverse proxy's X-Forwarded-For so c.IP() (used by the rate
		// limiter) is the real client address rather than the proxy's. Safe only
		// behind a trusted proxy (nginx) that overwrites this header.
		ProxyHeader: fiber.HeaderXForwardedFor,
	})

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
	s.app.Use(middleware.CORS())
	s.app.Use(middleware.RateLimit()) // Generous global rate limit (per client IP)
	s.app.Use(middleware.Logger())
	// Create a permission loader for the auth middleware (used on cache miss / server restart)
	permLoader := handlers.NewPermissionLoader(s.db, s.dbType)
	s.app.Use(middleware.AuthMiddleware(s.jwtConfig, s.db, s.dbType, s.sessionCache, permLoader))

	// API routes
	api := s.app.Group("/api")

	// Initialize handlers
	sessionHandler := handlers.NewSessionHandler()
	tablesHandler := handlers.NewTablesHandlerFull(s.db, s.dbType, s.companyInit)
	pagesHandler := handlers.NewPagesHandler()
	preferencesHandler := handlers.NewPreferencesHandlerWithDBType(s.db, s.dbType)
	themesHandler := handlers.NewThemesHandler(s.db, s.dbType)
	authHandler := handlers.NewAuthHandlerFull(s.db, s.dbType, s.companyInit, s.jwtConfig, s.sessionCache)
	codeunitsHandler := handlers.NewCodeunitsHandlerWithDBType(s.db, s.dbType)
	tablesHandler.OnCompanyChanged(authHandler.CompanyChanged) // renamed/deleted company: sessions follow

	// Translation routes
	translationsHandler := handlers.NewTranslationsHandler()
	api.Get("/translations", translationsHandler.GetTranslations)

	// Auth routes
	api.Post("/auth/login", middleware.LoginRateLimit(), authHandler.Login) // Strict limit: brute-force protection
	api.Post("/auth/logout", authHandler.Logout)
	api.Get("/auth/user", authHandler.GetCurrentUser)
	api.Post("/auth/init", authHandler.CreateInitialUser)
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
	// the code that runs for a user gets the user's work date and formats
	fcodeunits.RegionalLoader = handlers.RegionalLoader(s.db, s.dbType)
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
			"service": "openerp-api",
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
