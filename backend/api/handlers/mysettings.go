package handlers

import (
	"database/sql"
	"strings"

	"github.com/gofiber/fiber/v2"

	apitypes "github.com/hansjlachmann/openerp/backend/api/types"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// My Settings: a user's regional settings — date format, number format, work date — kept in
// User_Personalization (the options by index, the API by their format text)

// dateFormats and numberFormats: the API's names of the options (index = option number;
// "" = the language's)
var (
	dateFormats   = []string{"", "dd.mm.yy", "dd.mm.yyyy", "dd/mm/yyyy", "mm/dd/yyyy", "yyyy-mm-dd"}
	numberFormats = []string{"", "1,234.56", "1.234,56", "1 234,56", "1234.56"}
)

// mySettings is the body of GET/PUT /api/my-settings (PUT changes the fields sent)
type mySettings struct {
	DateFormat   *string `json:"date_format,omitempty"`
	NumberFormat *string `json:"number_format,omitempty"`
	WorkDate     *string `json:"work_date,omitempty"` // ISO; "" = today
	Today        string  `json:"today,omitempty"`     // the server's date (GET)
}

// MySettingsHandler serves My Settings
type MySettingsHandler struct {
	db     *sql.DB
	dbType database.DBType
}

// NewMySettingsHandler creates the handler
func NewMySettingsHandler(db *sql.DB, dbType database.DBType) *MySettingsHandler {
	return &MySettingsHandler{db: db, dbType: dbType}
}

// RegionalLoader reads a user's regional settings for the code that runs for the user
// (fcodeunits.RegionalLoader: the work date, the formats of printouts)
func RegionalLoader(db *sql.DB, dbType database.DBType) func(userID string) fcodeunits.Regional {
	return func(userID string) fcodeunits.Regional {
		rec := &gtables.UserPersonalization{}
		rec.InitWithDBType(db, "", dbType)
		if !rec.GetByPK(types.NewCode(userID)) {
			return fcodeunits.Regional{}
		}
		return fcodeunits.Regional{
			WorkDate:     rec.Work_date,
			DateFormat:   optionName(dateFormats, int(rec.Date_format)),
			NumberFormat: optionName(numberFormats, int(rec.Number_format)),
		}
	}
}

// load reads the user's record, a new one with the defaults when there is none
func (h *MySettingsHandler) load(userID string) (*gtables.UserPersonalization, bool) {
	rec := &gtables.UserPersonalization{}
	rec.InitWithDBType(h.db, "", h.dbType)
	if rec.GetByPK(types.NewCode(userID)) {
		return rec, true
	}
	def := tables.DefaultUserPersonalization(userID)
	def.InitWithDBType(h.db, "", h.dbType)
	return def, false
}

func settingsOf(rec *gtables.UserPersonalization) mySettings {
	str := func(s string) *string { return &s }
	return mySettings{
		DateFormat:   str(optionName(dateFormats, int(rec.Date_format))),
		NumberFormat: str(optionName(numberFormats, int(rec.Number_format))),
		WorkDate:     str(rec.Work_date.String()),
		Today:        types.Today().String(),
	}
}

// GetMySettings returns the user's regional settings
// GET /api/my-settings
func (h *MySettingsHandler) GetMySettings(c *fiber.Ctx) error {
	sess := getSession(c)
	if sess == nil || sess.GetUserID() == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}
	rec, _ := h.load(sess.GetUserID())
	return c.JSON(apitypes.NewSuccessResponse(settingsOf(rec)))
}

// SaveMySettings changes the user's regional settings (the fields sent)
// PUT /api/my-settings
func (h *MySettingsHandler) SaveMySettings(c *fiber.Ctx) error {
	sess := getSession(c)
	if sess == nil || sess.GetUserID() == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}
	language := getLanguageOrDefault(sess)
	var in mySettings
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
	}
	rec, exists := h.load(sess.GetUserID())
	if in.DateFormat != nil {
		i := optionIndex(dateFormats, strings.ToLower(*in.DateFormat))
		if i < 0 {
			return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidValue("date_format", *in.DateFormat).Message(language)))
		}
		rec.Date_format = gtables.UserPersonalizationDate_format(i)
	}
	if in.NumberFormat != nil {
		i := optionIndex(numberFormats, *in.NumberFormat)
		if i < 0 {
			return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidValue("number_format", *in.NumberFormat).Message(language)))
		}
		rec.Number_format = gtables.UserPersonalizationNumber_format(i)
	}
	if in.WorkDate != nil {
		if *in.WorkDate == "" {
			rec.Work_date = types.Date{}
		} else {
			d, err := types.NewDateFromString(*in.WorkDate)
			if err != nil || d.IsClosing() {
				return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidValue("work_date", *in.WorkDate).Message(language)))
			}
			rec.Work_date = d
		}
	}
	var ok bool
	if exists {
		ok = rec.Modify(true)
	} else {
		ok = rec.Insert(true)
	}
	if !ok {
		if err := rec.TriggerError(); err != nil {
			return c.Status(400).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
		}
		return c.Status(500).JSON(apitypes.NewErrorResponse(apperrors.ModifyFailed(gtables.UserPersonalizationTableName, sess.GetUserID()).Message(language)))
	}
	return c.JSON(apitypes.NewSuccessResponse(settingsOf(rec)))
}

// ResetWorkDate: the work date is back to today at every login (classic: a session's work date)
func ResetWorkDate(db *sql.DB, dbType database.DBType, userID string) {
	rec := &gtables.UserPersonalization{}
	rec.InitWithDBType(db, "", dbType)
	if rec.GetByPK(types.NewCode(userID)) && !rec.Work_date.IsZero() {
		rec.Work_date = types.Date{}
		rec.Modify(false)
	}
}

// DimensionLabelsLoader reads a company's names of its two dimensions (General Ledger Setup
// Dimension-1 / Dimension-2) for the captions (codeunits.DimensionLabelsLoader)
func DimensionLabelsLoader(db *sql.DB, dbType database.DBType) func(company string) [2]string {
	return func(company string) [2]string {
		glSetup := gtables.GetGeneralLedgerSetup(ftables.DBSource(db, company, dbType))
		return [2]string{glSetup.Dimension_1.String(), glSetup.Dimension_2.String()}
	}
}
