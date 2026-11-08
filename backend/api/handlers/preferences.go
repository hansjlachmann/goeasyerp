package handlers

import (
	"database/sql"
	"encoding/json"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	apitypes "github.com/hansjlachmann/goeasyerp/backend/api/types"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// maxPreferenceData: the most a preference may hold (JSON bytes)
const maxPreferenceData = 64 * 1024

// PreferencesHandler handles user preferences API requests
type PreferencesHandler struct {
	db     *sql.DB
	dbType database.DBType
}

// NewPreferencesHandler creates a new preferences handler (defaults to SQLite)
func NewPreferencesHandler(db *sql.DB) *PreferencesHandler {
	return NewPreferencesHandlerWithDBType(db, database.DBTypeSQLite)
}

// NewPreferencesHandlerWithDBType creates a new preferences handler with explicit database type
func NewPreferencesHandlerWithDBType(db *sql.DB, dbType database.DBType) *PreferencesHandler {
	return &PreferencesHandler{db: db, dbType: dbType}
}

// GetPreferences returns user preferences for a specific page and type
// GET /api/preferences/:page_id/:type
func (h *PreferencesHandler) GetPreferences(c *fiber.Ctx) error {
	pageID := c.Params("page_id")
	preferenceType := c.Params("type")
	sess := getSession(c)

	if sess == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}

	language := getLanguageOrDefault(sess)
	userID := sess.GetUserID()
	if userID == "" {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.NotLoggedIn().Message(language)))
	}

	// Query preferences for this user, page, and type
	var prefs gtables.UserPreferences
	prefs.InitWithDBType(h.db, "", h.dbType) // No company context

	// Set filters
	prefs.SetFilter("user_id", userID)
	prefs.SetFilter("page_id", pageID) // SetFilter handles type conversion
	prefs.SetFilter("preference_type", preferenceType)

	var results []map[string]interface{}

	if prefs.FindSet() {
		for {
			results = append(results, map[string]interface{}{
				"preference_name": prefs.Preference_name.String(),
				"preference_data": prefs.Preference_data.String(),
				"created_at":      prefs.Created_at.Time,
				"updated_at":      prefs.Updated_at.Time,
			})
			if !prefs.Next() {
				break
			}
		}
	}

	response := apitypes.NewSuccessResponse(results)
	return c.JSON(response)
}

// SavePreference saves or updates a user preference
// POST /api/preferences/:page_id/:type
func (h *PreferencesHandler) SavePreference(c *fiber.Ctx) error {
	pageID := c.Params("page_id")
	preferenceType := c.Params("type")
	sess := getSession(c)

	if sess == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}

	language := getLanguageOrDefault(sess)
	userID := sess.GetUserID()
	if userID == "" {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.NotLoggedIn().Message(language)))
	}

	// Parse request body
	var requestBody struct {
		PreferenceName string                 `json:"preference_name"`
		PreferenceData map[string]interface{} `json:"preference_data"`
	}

	if err := c.BodyParser(&requestBody); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
	}

	// Convert preference data to JSON string
	dataJSON, err := json.Marshal(requestBody.PreferenceData)
	if err != nil {
		return c.Status(500).JSON(apitypes.NewErrorResponse(apperrors.PreferenceSerialize().Message(language)))
	}
	// A page's layout or a filter preset is small; a user cannot fill the global table
	if len(dataJSON) > maxPreferenceData {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
	}

	// Convert page_id string to int
	pageIDInt, err := strconv.Atoi(pageID)
	if err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidPageID().Message(language)))
	}

	// Check if preference already exists
	var pref gtables.UserPreferences
	pref.InitWithDBType(h.db, "", h.dbType)

	exists := pref.GetByPK(
		types.NewCode(userID),
		pageIDInt,
		types.NewCode(preferenceType),
		types.NewCode(requestBody.PreferenceName),
	)
	now := types.NewDateTimeFromTime(time.Now())

	if exists {
		// Update existing preference
		pref.Preference_data = types.NewText(string(dataJSON))
		pref.Updated_at = now

		if !pref.Modify(true) {
			return c.Status(500).JSON(apitypes.NewErrorResponse(apperrors.PreferenceUpdateFailed().Message(language)))
		}
	} else {
		// Insert new preference (GetByPK found nothing, so the key fields are set here)
		pref.InitRecord()
		pref.User_id = types.NewCode(userID)
		pref.Page_id = pageIDInt
		pref.Preference_type = types.NewCode(preferenceType)
		pref.Preference_name = types.NewCode(requestBody.PreferenceName)
		pref.Preference_data = types.NewText(string(dataJSON))
		pref.Created_at = now
		pref.Updated_at = now

		if !pref.Insert(true) {
			return c.Status(500).JSON(apitypes.NewErrorResponse(apperrors.PreferenceSaveFailed().Message(language)))
		}
	}

	ts := i18n.GetInstance()
	response := apitypes.NewSuccessResponse(map[string]interface{}{
		"message": ts.Message("MSG_PREFERENCE_SAVED", language),
	})
	return c.JSON(response)
}

// DeletePreference deletes a user preference
// DELETE /api/preferences/:page_id/:type/:name
func (h *PreferencesHandler) DeletePreference(c *fiber.Ctx) error {
	pageID := c.Params("page_id")
	preferenceType := c.Params("type")
	preferenceName := c.Params("name")
	sess := getSession(c)

	if sess == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}

	language := getLanguageOrDefault(sess)
	userID := sess.GetUserID()
	if userID == "" {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.NotLoggedIn().Message(language)))
	}

	// Convert page_id string to int
	pageIDInt, err := strconv.Atoi(pageID)
	if err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidPageID().Message(language)))
	}

	// Find and delete the preference
	var pref gtables.UserPreferences
	pref.InitWithDBType(h.db, "", h.dbType)

	if !pref.GetByPK(
		types.NewCode(userID),
		pageIDInt,
		types.NewCode(preferenceType),
		types.NewCode(preferenceName),
	) {
		return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.PreferenceNotFound().Message(language)))
	}

	if !pref.Delete(true) {
		return c.Status(500).JSON(apitypes.NewErrorResponse(apperrors.PreferenceDeleteFailed().Message(language)))
	}

	ts := i18n.GetInstance()
	response := apitypes.NewSuccessResponse(map[string]interface{}{
		"message": ts.Message("MSG_PREFERENCE_DELETED", language),
	})
	return c.JSON(response)
}
