package handlers

import (
	"database/sql"
	"strings"

	"github.com/gofiber/fiber/v2"

	apitypes "github.com/hansjlachmann/goeasyerp/backend/api/types"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Main Menu Text: the company's own text at the top of the main menu (table 907, entered on
// page 914 from Company Information). Every user of the company sees it, so it is read here for
// the main menu without the table's read permission.

// mainMenuTextLine is one line as the main menu shows it (plain text)
type mainMenuTextLine struct {
	Text   string `json:"text"`
	Style  string `json:"style"` // normal, heading, large, small
	Bold   bool   `json:"bold"`
	Italic bool   `json:"italic"`
}

// mainMenuStyles: the API's names of the Style options (index = option number)
var mainMenuStyles = []string{"normal", "heading", "large", "small"}

// MainMenuTextHandler serves the main menu's text
type MainMenuTextHandler struct {
	db     *sql.DB
	dbType database.DBType
}

// NewMainMenuTextHandler creates the handler
func NewMainMenuTextHandler(db *sql.DB, dbType database.DBType) *MainMenuTextHandler {
	return &MainMenuTextHandler{db: db, dbType: dbType}
}

// GetMainMenuText returns the lines of the user's language, else those of all languages (a
// blank Language Code); none: the main menu shows its standard welcome text
// GET /api/main-menu-text
func (h *MainMenuTextHandler) GetMainMenuText(c *fiber.Ctx) error {
	sess := getSession(c)
	if sess == nil || sess.GetUserID() == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}
	lines := h.lines(sess.GetCompany(), sess.GetLanguage())
	return c.JSON(apitypes.NewSuccessResponse(map[string]interface{}{"lines": lines}))
}

// lines: the company's lines for the language (a locale such as nb-NO)
func (h *MainMenuTextHandler) lines(company, language string) []mainMenuTextLine {
	byCode := map[string][]mainMenuTextLine{}
	rec := &gtables.MainMenuText{}
	rec.InitWithDBType(h.db, company, h.dbType)
	for found := rec.FindSet(); found; found = rec.Next() {
		style := "normal"
		if i := int(rec.Style); i >= 0 && i < len(mainMenuStyles) {
			style = mainMenuStyles[i]
		}
		code := rec.Language_code.String()
		byCode[code] = append(byCode[code], mainMenuTextLine{Text: rec.Text.String(), Style: style, Bold: rec.Bold, Italic: rec.Italic})
	}
	for code, lines := range byCode {
		if code != "" && h.isLanguage(company, code, language) {
			return lines
		}
	}
	lines := byCode[""]
	if lines == nil {
		lines = []mainMenuTextLine{}
	}
	return lines
}

// isLanguage: the Language Code stands for the locale — its Translation Key, or the code
// itself (EN-US)
func (h *MainMenuTextHandler) isLanguage(company, code, locale string) bool {
	if strings.EqualFold(code, locale) {
		return true
	}
	lang := &gtables.Language{}
	lang.InitWithDBType(h.db, company, h.dbType)
	return lang.Get(types.NewCode(code)) && strings.EqualFold(lang.Translation_key.String(), locale)
}
