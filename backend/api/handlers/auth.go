package handlers

import (
	"database/sql"
	"fmt"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/usertime"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/goeasyerp/backend/api/middleware"
	apitypes "github.com/hansjlachmann/goeasyerp/backend/api/types"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/company"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// getLanguageOrDefault returns the session language or "en-US" as default
func getLanguageOrDefault(sess *session.Session) string {
	if sess != nil {
		lang := sess.GetLanguage()
		if lang != "" {
			return lang
		}
	}
	return "en-US"
}

// AuthHandler handles authentication API requests
type AuthHandler struct {
	db           *sql.DB
	dbType       database.DBType
	companyInit  CompanyInitializer
	jwtConfig    middleware.JWTConfig
	sessionCache *middleware.SessionCache
}

// NewAuthHandler creates a new auth handler (defaults to SQLite)
func NewAuthHandler(db *sql.DB) *AuthHandler {
	return NewAuthHandlerWithDBType(db, database.DBTypeSQLite)
}

// NewAuthHandlerWithDBType creates a new auth handler with explicit database type
func NewAuthHandlerWithDBType(db *sql.DB, dbType database.DBType) *AuthHandler {
	return &AuthHandler{db: db, dbType: dbType}
}

// NewAuthHandlerFull creates an auth handler with company initializer support
func NewAuthHandlerFull(db *sql.DB, dbType database.DBType, companyInit CompanyInitializer, jwtConfig middleware.JWTConfig, sessionCache *middleware.SessionCache) *AuthHandler {
	return &AuthHandler{db: db, dbType: dbType, companyInit: companyInit, jwtConfig: jwtConfig, sessionCache: sessionCache}
}

// Login authenticates a user and creates a session
// POST /api/auth/login
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var requestBody struct {
		UserID   string `json:"user_id"`
		Password string `json:"password"`
		Company  string `json:"company"`
	}

	if err := c.BodyParser(&requestBody); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message("en-US")))
	}

	if requestBody.UserID == "" || requestBody.Password == "" {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.UserRequired().Message("en-US")))
	}

	// Determine company - use provided company, or default to "cronus"
	company := requestBody.Company
	if company == "" {
		company = "cronus"
	}

	// Verify company exists
	var companyCheck string
	err := h.db.QueryRow(`SELECT name FROM "Company" WHERE name = $1`, company).Scan(&companyCheck)
	if err == sql.ErrNoRows {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.CompanyNotFound().Message("en-US")))
	}
	if err != nil {
		return c.Status(500).JSON(apitypes.NewErrorResponse(apperrors.CompanyVerifyFailed().Message("en-US")))
	}

	var user gtables.User
	user.InitWithDBType(h.db, company, h.dbType)

	if !user.Get(types.NewCode(requestBody.UserID)) {
		return c.Status(401).JSON(apitypes.NewErrorResponse(apperrors.InvalidCredentials().Message("en-US")))
	}

	// Check if user is active
	if !user.Active {
		return c.Status(401).JSON(apitypes.NewErrorResponse(apperrors.UserInactive().Message("en-US")))
	}

	// Verify password
	if !tables.CheckPassword(&user, requestBody.Password) {
		return c.Status(401).JSON(apitypes.NewErrorResponse(apperrors.InvalidCredentials().Message("en-US")))
	}

	// Check company access before proceeding (use canonical user ID from DB, not raw input)
	allowed, _ := h.userHasCompanyAccess(user.User_id.String(), company)
	if !allowed {
		return c.Status(403).JSON(apitypes.NewErrorResponse(apperrors.CompanyAccessDenied(company).Message("en-US")))
	}

	// Update last login (ignore failure - non-critical)
	tables.UpdateLastLogin(&user)
	_ = user.Modify(false)

	// Load permissions for this user/company
	hasRoles, isSuper, perms := h.loadUserPermissions(user.User_id.String(), company)

	// Create per-request session and cache it
	userMenu := user.Menu.String()
	if userMenu == "" {
		userMenu = "admin"
	}
	dbWrapper := database.WrapConnection(h.db, h.dbType)
	sess := session.NewSession(dbWrapper, company, nil)
	sess.SetUser(user.User_id.String(), user.User_name.String(), user.Language.String(), userMenu)
	sess.SetPermissions(hasRoles, isSuper, perms)
	loginTime := time.Now()
	sess.SetLoginTime(loginTime)
	// a login starts on today (the work date of My Settings lasts for the session)
	ResetWorkDate(h.db, h.dbType, user.User_id.String())

	// Generate JWT and set cookie
	token, tokenErr := middleware.GenerateToken(h.jwtConfig, user.User_id.String(), user.User_name.String(), company, user.Language.String(), userMenu, loginTime)
	if tokenErr != nil {
		return c.Status(500).JSON(apitypes.NewErrorResponse("Failed to create session token"))
	}
	middleware.SetAuthCookie(c, h.jwtConfig, token)

	// Cache the session
	cacheKey := user.User_id.String() + ":" + company
	h.sessionCache.Set(cacheKey, sess, h.jwtConfig.TokenExpiry)

	ts := i18n.GetInstance()
	userLang := user.Language.String()
	if userLang == "" {
		userLang = "en-US"
	}

	// Look up the translation_key from the Language table
	translationKey := "en-US" // Default
	var lang gtables.Language
	lang.InitWithDBType(h.db, company, h.dbType)
	if lang.Get(types.NewCode(userLang)) {
		if !lang.Translation_key.IsEmpty() {
			translationKey = lang.Translation_key.String()
		}
	}

	response := apitypes.NewSuccessResponse(map[string]interface{}{
		"user_id":         user.User_id.String(),
		"user_name":       user.User_name.String(),
		"email":           user.Email.String(),
		"language":        userLang,
		"translation_key": translationKey,
		"menu":            userMenu,
		"company":         company,
		"message":         ts.Message("MSG_LOGIN_SUCCESS", translationKey),
	})
	return c.JSON(response)
}

// Logout ends the current user session
// POST /api/auth/logout
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	sess := getSession(c)
	language := getLanguageOrDefault(sess)

	// Remove from cache
	if sess != nil && sess.GetUserID() != "" {
		h.registerTime(sess)
		h.sessionCache.RemoveByUser(sess.GetUserID())
	}

	// Clear cookie
	middleware.ClearAuthCookie(c, h.jwtConfig)

	ts := i18n.GetInstance()
	response := apitypes.NewSuccessResponse(map[string]interface{}{
		"message": ts.Message("MSG_LOGOUT_SUCCESS", language),
	})
	return c.JSON(response)
}

// registerTime adds the session's time in its company to the user's time register (when the
// user registers time; a failure only logs — logging out or changing company goes on)
func (h *AuthHandler) registerTime(sess *session.Session) {
	if sess.GetCompany() == "" {
		return
	}
	src := ftables.DBSource(h.db, sess.GetCompany(), h.dbType)
	if err := usertime.Register(src, sess.GetUserID(), sess.GetLoginTime(), time.Now()); err != nil {
		log.Printf("user time register of %s in %s: %v", sess.GetUserID(), sess.GetCompany(), err)
	}
}

// GetCurrentUser returns the currently logged in user
// GET /api/auth/user
func (h *AuthHandler) GetCurrentUser(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")

	sess := getSession(c)
	if sess == nil {
		return c.Status(401).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}

	language := getLanguageOrDefault(sess)
	userID := sess.GetUserID()
	if userID == "" {
		return c.Status(401).JSON(apitypes.NewErrorResponse(apperrors.NotLoggedIn().Message(language)))
	}

	// Get full user details
	company := sess.GetCompany()
	if company == "" {
		company = "cronus"
	}

	var user gtables.User
	user.InitWithDBType(h.db, company, h.dbType)

	if !user.Get(types.NewCode(userID)) {
		return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.UserNotFound().Message(language)))
	}

	response := apitypes.NewSuccessResponse(map[string]interface{}{
		"user_id":    user.User_id.String(),
		"user_name":  user.User_name.String(),
		"email":      user.Email.String(),
		"language":   user.Language.String(),
		"active":     user.Active,
		"created_at": user.Created_at.Time,
		"last_login": user.Last_login.Time,
	})
	return c.JSON(response)
}

// CreateInitialUser creates the first admin user (for setup)
// POST /api/auth/init
func (h *AuthHandler) CreateInitialUser(c *fiber.Ctx) error {
	var requestBody struct {
		UserID   string `json:"user_id"`
		UserName string `json:"user_name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.BodyParser(&requestBody); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message("en-US")))
	}

	// Check if any users exist
	// Use default company "cronus" for user storage
	company := "cronus"
	var user gtables.User
	user.InitWithDBType(h.db, company, h.dbType)
	count := user.Count()

	if count > 0 {
		return c.Status(403).JSON(apitypes.NewErrorResponse(apperrors.UsersExist().Message("en-US")))
	}

	// Create the initial user
	now := time.Now()
	user.User_id = types.NewCode(requestBody.UserID)
	user.User_name = types.NewText(requestBody.UserName)
	user.Email = types.NewText(requestBody.Email)
	user.Language = types.NewCode("en-US")
	user.Active = true
	user.Created_at = types.NewDateTimeFromTime(now)
	user.Last_login = types.NewDateTimeFromTime(now) // Initialize to avoid NULL

	if err := tables.SetPassword(&user, requestBody.Password); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(err.Error()))
	}

	if !user.Insert(true) {
		return c.Status(500).JSON(apitypes.NewErrorResponse(apperrors.CreateUserFailed().Message("en-US")))
	}

	ts := i18n.GetInstance()
	response := apitypes.NewSuccessResponse(map[string]interface{}{
		"message":   ts.Message("MSG_USER_CREATED", "en-US"),
		"user_id":   user.User_id.String(),
		"user_name": user.User_name.String(),
	})
	return c.JSON(response)
}

// CompanyInfo is a company as listed for the login page and the company switcher.
// Name is the technical key (prefixes the company's tables); DisplayName is what
// users see (Display Name) and may be blank.
type CompanyInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// ListCompanies returns available companies for the current user.
// If the user is logged in and has role memberships, only returns companies they have access to.
// If not logged in (login screen) or user has no roles, returns all companies.
// GET /api/auth/companies
func (h *AuthHandler) ListCompanies(c *fiber.Ctx) error {
	// Get all companies first
	rows, err := h.db.Query(`SELECT name, COALESCE(display_name, '') FROM "Company" ORDER BY name`)
	if err != nil {
		return c.Status(500).JSON(apitypes.NewErrorResponse(apperrors.CompanyListFailed().Message("en-US")))
	}
	defer rows.Close()

	allCompanies := []CompanyInfo{}
	for rows.Next() {
		var info CompanyInfo
		if err := rows.Scan(&info.Name, &info.DisplayName); err != nil {
			return c.Status(500).JSON(apitypes.NewErrorResponse(apperrors.CompanyListFailed().Message("en-US")))
		}
		allCompanies = append(allCompanies, info)
	}

	// If user is logged in, filter by allowed companies
	sess := getSession(c)
	if sess != nil && sess.GetUserID() != "" {
		allowedCompanies, hasRoles := h.getUserAllowedCompanies(sess.GetUserID())
		if hasRoles && allowedCompanies != nil {
			// Filter to only allowed companies
			filtered := make([]CompanyInfo, 0, len(allowedCompanies))
			allowedSet := make(map[string]bool, len(allowedCompanies))
			for _, c := range allowedCompanies {
				allowedSet[strings.ToLower(c)] = true
			}
			for _, c := range allCompanies {
				if allowedSet[strings.ToLower(c.Name)] {
					filtered = append(filtered, c)
				}
			}
			response := apitypes.NewSuccessResponse(filtered)
			return c.JSON(response)
		}
		// hasRoles=true but allowedCompanies=nil means blank company (all access)
		// hasRoles=false means no roles configured, show all
	}

	response := apitypes.NewSuccessResponse(allCompanies)
	return c.JSON(response)
}

// SetLanguage changes the current session language
// POST /api/auth/language
func (h *AuthHandler) SetLanguage(c *fiber.Ctx) error {
	var requestBody struct {
		Language string `json:"language"`
		Persist  bool   `json:"persist"` // If true, also update user record
	}

	if err := c.BodyParser(&requestBody); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message("en-US")))
	}

	if requestBody.Language == "" {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.LanguageRequired().Message("en-US")))
	}

	// Validate language code (must be one of supported languages)
	supportedLanguages := []string{"en-US", "nb-NO", "da-DK"}
	valid := false
	for _, lang := range supportedLanguages {
		if lang == requestBody.Language {
			valid = true
			break
		}
	}
	if !valid {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.UnsupportedLanguage(requestBody.Language).Message("en-US")))
	}

	sess := getSession(c)
	if sess == nil {
		return c.Status(401).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}

	// Capture current values before creating new session
	userID := sess.GetUserID()
	userName := sess.GetUserName()
	company := sess.GetCompany()
	menu := sess.GetMenu()

	// Optionally persist to user record
	if requestBody.Persist {
		if userID != "" && company != "" {
			var user gtables.User
			user.InitWithDBType(h.db, company, h.dbType)
			if user.Get(types.NewCode(userID)) {
				user.Language = types.NewCode(requestBody.Language)
				user.Modify(false) // Don't run triggers
			}
		}
	}

	// Create new session with updated language (avoid mutating cached session)
	dbWrapper := database.WrapConnection(h.db, h.dbType)
	newSess := session.NewSession(dbWrapper, company, nil)
	newSess.SetUser(userID, userName, requestBody.Language, menu)
	newSess.SetPermissions(sess.HasRoles, sess.IsSuper, sess.Permissions)
	newSess.SetLoginTime(sess.GetLoginTime())

	// Re-issue JWT with updated language
	token, tokenErr := middleware.GenerateToken(h.jwtConfig, userID, userName, company, requestBody.Language, menu, sess.GetLoginTime())
	if tokenErr == nil {
		middleware.SetAuthCookie(c, h.jwtConfig, token)
		cacheKey := userID + ":" + company
		h.sessionCache.Set(cacheKey, newSess, h.jwtConfig.TokenExpiry)
	}

	ts := i18n.GetInstance()
	response := apitypes.NewSuccessResponse(map[string]interface{}{
		"language": requestBody.Language,
		"message":  ts.Message("MSG_LANGUAGE_UPDATED", requestBody.Language),
	})
	return c.JSON(response)
}

// SetCompany changes the current session company
// POST /api/auth/company
func (h *AuthHandler) SetCompany(c *fiber.Ctx) error {
	var requestBody struct {
		Company string `json:"company"`
	}

	if err := c.BodyParser(&requestBody); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message("en-US")))
	}

	if requestBody.Company == "" {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.CompanyRequired().Message("en-US")))
	}

	// Verify company exists
	var companyCheck string
	err := h.db.QueryRow(`SELECT name FROM "Company" WHERE name = $1`, requestBody.Company).Scan(&companyCheck)
	if err == sql.ErrNoRows {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.CompanyNotFound().Message("en-US")))
	}
	if err != nil {
		return c.Status(500).JSON(apitypes.NewErrorResponse(apperrors.CompanyVerifyFailed().Message("en-US")))
	}

	sess := getSession(c)
	if sess == nil {
		return c.Status(401).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}

	// Check company access
	userID := sess.GetUserID()
	if userID != "" {
		allowed, _ := h.userHasCompanyAccess(userID, requestBody.Company)
		if !allowed {
			language := getLanguageOrDefault(sess)
			return c.Status(403).JSON(apitypes.NewErrorResponse(apperrors.CompanyAccessDenied(requestBody.Company).Message(language)))
		}
	}

	// Leaving the company ends the time registered in it; the new company's starts now
	h.registerTime(sess)
	loginTime := time.Now()

	// Remove old cache entry
	oldCacheKey := userID + ":" + sess.GetCompany()
	h.sessionCache.Remove(oldCacheKey)

	// Capture current values
	userName := sess.GetUserName()
	language := sess.GetLanguage()
	menu := sess.GetMenu()

	// Create new session with updated company (avoid mutating cached session)
	dbWrapper := database.WrapConnection(h.db, h.dbType)
	newSess := session.NewSession(dbWrapper, requestBody.Company, nil)
	newSess.SetUser(userID, userName, language, menu)
	newSess.SetLoginTime(loginTime)

	// Load permissions for the new company context
	if userID != "" {
		hasRoles, isSuper, perms := h.loadUserPermissions(userID, requestBody.Company)
		newSess.SetPermissions(hasRoles, isSuper, perms)
	}

	// Re-issue JWT with updated company
	token, tokenErr := middleware.GenerateToken(h.jwtConfig, userID, userName, requestBody.Company, language, menu, loginTime)
	if tokenErr == nil {
		middleware.SetAuthCookie(c, h.jwtConfig, token)
		newCacheKey := userID + ":" + requestBody.Company
		h.sessionCache.Set(newCacheKey, newSess, h.jwtConfig.TokenExpiry)
	}

	ts := i18n.GetInstance()
	language = getLanguageOrDefault(newSess)
	response := apitypes.NewSuccessResponse(map[string]interface{}{
		"company": requestBody.Company,
		"message": ts.Message("MSG_COMPANY_CHANGED", language),
	})
	return c.JSON(response)
}

// GetLanguages returns supported languages
// GET /api/auth/languages
func (h *AuthHandler) GetLanguages(c *fiber.Ctx) error {
	languages := []map[string]string{
		{"code": "en-US", "name": "English (US)"},
		{"code": "da-DK", "name": "Dansk"},
		{"code": "nb-NO", "name": "Norsk (Bokmål)"},
	}

	response := apitypes.NewSuccessResponse(languages)
	return c.JSON(response)
}

// CompanyChanged updates the sessions after a company was renamed (newName set) or deleted
// (newName ""): cached sessions of the company are dropped — the auth middleware logs
// their users out, as their token names a company that no longer exists — and the user
// who renamed the company they work in gets a new token for the new name.
func (h *AuthHandler) CompanyChanged(c *fiber.Ctx, oldName, newName string) {
	h.sessionCache.RemoveByCompany(oldName)
	sess := getSession(c)
	if newName == "" || sess == nil || sess.GetCompany() != oldName || sess.GetUserID() == "" {
		return
	}
	userID, userName, language, menu := sess.GetUserID(), sess.GetUserName(), sess.GetLanguage(), sess.GetMenu()
	newSess := session.NewSession(database.WrapConnection(h.db, h.dbType), newName, nil)
	newSess.SetUser(userID, userName, language, menu)
	newSess.SetLoginTime(sess.GetLoginTime())
	hasRoles, isSuper, perms := h.loadUserPermissions(userID, newName)
	newSess.SetPermissions(hasRoles, isSuper, perms)
	token, err := middleware.GenerateToken(h.jwtConfig, userID, userName, newName, language, menu, sess.GetLoginTime())
	if err != nil {
		return
	}
	middleware.SetAuthCookie(c, h.jwtConfig, token)
	h.sessionCache.Set(userID+":"+newName, newSess, h.jwtConfig.TokenExpiry)
}

// CreateCompany creates a new company
// POST /api/auth/companies
func (h *AuthHandler) CreateCompany(c *fiber.Ctx) error {
	var requestBody struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
	}

	if err := c.BodyParser(&requestBody); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message("en-US")))
	}

	if requestBody.Name == "" {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.CompanyRequired().Message("en-US")))
	}

	// Validate company name (a-z, 0-9, underscores, hyphens; 2-50 characters)
	name, nameErr := company.NormalizeName(requestBody.Name)
	if nameErr != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(errorMessage(nameErr, "en-US")))
	}

	// Check if company already exists
	var existingName string
	err := h.db.QueryRow(`SELECT name FROM "Company" WHERE name = $1`, name).Scan(&existingName)
	if err == nil {
		return c.Status(409).JSON(apitypes.NewErrorResponse(apperrors.CompanyAlreadyExists(name).Message("en-US")))
	}

	// Insert company
	displayName := strings.TrimSpace(requestBody.DisplayName)
	if r := []rune(displayName); len(r) > 100 {
		displayName = string(r[:100])
	}
	_, err = h.db.Exec(`INSERT INTO "Company" (name, display_name) VALUES ($1, $2)`, name, displayName)
	if err != nil {
		return c.Status(500).JSON(apitypes.NewErrorResponse(apperrors.CompanyCreateFailed().Message("en-US")))
	}

	// Initialize company-scoped tables for the new company
	if h.companyInit != nil {
		if initErr := h.companyInit.InitializeCompanyTablesWithDBType(h.db, name, h.dbType); initErr != nil {
			fmt.Printf("Warning: Failed to initialize tables for company '%s': %v\n", name, initErr)
		}
	}

	ts := i18n.GetInstance()
	response := apitypes.NewSuccessResponse(map[string]interface{}{
		"name":         name,
		"display_name": displayName,
		"message":      ts.Message("MSG_COMPANY_CREATED", "en-US"),
	})
	return c.JSON(response)
}

// getUserAllowedCompanies returns the list of companies a user is allowed to access.
// If the user has no User_Member records at all, returns (nil, false) — meaning the permission
// system is not configured for this user and all companies are allowed.
// If the user has memberships, returns the distinct company names (blank company = all companies).
func (h *AuthHandler) getUserAllowedCompanies(userID string) ([]string, bool) {
	var query string
	if h.dbType == database.DBTypePostgres {
		query = `SELECT DISTINCT company FROM "User_Member" WHERE user_id = $1`
	} else {
		query = `SELECT DISTINCT company FROM "User_Member" WHERE user_id = ?`
	}

	rows, err := h.db.Query(query, userID)
	if err != nil {
		return nil, false
	}
	defer rows.Close()

	var companies []string
	hasBlank := false
	for rows.Next() {
		var company string
		if err := rows.Scan(&company); err != nil {
			continue
		}
		if company == "" {
			hasBlank = true
		} else {
			companies = append(companies, company)
		}
	}

	// No memberships at all — permission system not configured for this user
	if len(companies) == 0 && !hasBlank {
		return nil, false
	}

	// Blank company means access to all companies
	if hasBlank {
		return nil, true
	}

	return companies, true
}

// userHasCompanyAccess checks if a user can access a specific company.
// Returns (allowed, hasRoles). If hasRoles is false, all companies are allowed.
func (h *AuthHandler) userHasCompanyAccess(userID, company string) (bool, bool) {
	allowedCompanies, hasRoles := h.getUserAllowedCompanies(userID)
	if !hasRoles {
		return true, false // No roles = all access
	}
	if allowedCompanies == nil {
		return true, true // Blank company = all companies
	}
	for _, c := range allowedCompanies {
		if strings.EqualFold(c, company) {
			return true, true
		}
	}
	return false, true
}

// loadUserPermissions loads a user's merged permissions in a company (session.LoadPermissions):
// (hasRoles, isSuper, permissions)
func (h *AuthHandler) loadUserPermissions(userID, company string) (bool, bool, map[string]session.TablePermission) {
	return session.LoadPermissions(h.db, h.dbType, userID, company)
}
