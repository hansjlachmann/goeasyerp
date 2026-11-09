package handlers

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"time"

	"github.com/gofiber/fiber/v2"
	apitypes "github.com/hansjlachmann/goeasyerp/backend/api/types"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/themerepo"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/themes"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// ThemesHandler serves the color themes and the user's appearance (User_Personalization).
type ThemesHandler struct {
	db        *sql.DB
	dbType    database.DBType
	installer *themerepo.Installer
}

// NewThemesHandler creates a themes handler
func NewThemesHandler(db *sql.DB, dbType database.DBType) *ThemesHandler {
	return &ThemesHandler{db: db, dbType: dbType, installer: themerepo.NewInstaller(db, dbType)}
}

// Option values of the appearance as the API names them (index = option value)
var (
	appearanceModes       = []string{"dark", "light"}
	appearanceFonts       = []string{"default", "system", "inter", "ibm_plex_sans", "serif", "monospace"}
	appearanceDensities   = []string{"comfortable", "compact"}
	appearanceAmountFonts = []string{"text", "monospace"}
)

// appearanceSettings is the user's appearance as the API sends and receives it. On PUT
// every field is optional: only the fields sent change (the mode toggle sends only mode).
type appearanceSettings struct {
	LightTheme       *string `json:"light_theme,omitempty"`
	DarkTheme        *string `json:"dark_theme,omitempty"`
	Mode             *string `json:"mode,omitempty"`
	Wallpaper        *string `json:"wallpaper,omitempty"`
	WallpaperOpacity *int    `json:"wallpaper_opacity,omitempty"`
	WallpaperBlur    *int    `json:"wallpaper_blur,omitempty"`
	PanelOpacity     *int    `json:"panel_opacity,omitempty"`
	Font             *string `json:"font,omitempty"`
	FontSize         *int    `json:"font_size,omitempty"`
	Density          *string `json:"density,omitempty"`
	AmountFont       *string `json:"amount_font,omitempty"`
}

// appearanceResponse is the user's settings plus the tokens of both theme slots, so the
// frontend can switch the mode without another request.
type appearanceResponse struct {
	Saved    bool               `json:"saved"` // false: the user has no record yet (defaults)
	Settings appearanceSettings `json:"settings"`
	Light    themes.Theme       `json:"light"`
	Dark     themes.Theme       `json:"dark"`
}

// themeList is the user's theme gallery: the built-in themes and the installed themes the
// user added. Every user may install, update and remove their own themes (the flags stay
// for the frontend; deleting a theme for everybody is the Themes page, Delete permission).
type themeList struct {
	Themes     []themes.Theme `json:"themes"`
	CanInstall bool           `json:"can_install"`
	CanRefresh bool           `json:"can_refresh"`
	CanDelete  bool           `json:"can_delete"`
}

// ListThemes returns the themes the user can choose: built-in, then the user's installed ones
// GET /api/themes
func (h *ThemesHandler) ListThemes(c *fiber.Ctx) error {
	sess := getSession(c)
	if sess == nil || sess.GetUserID() == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}
	list := append(append([]themes.Theme{}, themes.Builtins()...), tables.UserThemes(h.db, h.dbType, sess.GetUserID())...)
	return c.JSON(apitypes.NewSuccessResponse(themeList{Themes: list, CanInstall: true, CanRefresh: true, CanDelete: true}))
}

// InstallTheme adds the theme of a GitHub repository to the user's themes (any user)
// POST /api/themes/install {"url": "https://github.com/owner/repo"}
func (h *ThemesHandler) InstallTheme(c *fiber.Ctx) error {
	sess, lang, err := h.themeUser(c)
	if err != nil {
		return err
	}
	var in struct {
		URL string `json:"url"`
	}
	if err := c.BodyParser(&in); err != nil || len(in.URL) > 300 {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(lang)))
	}
	res, ierr := h.installer.Install(context.Background(), in.URL, sess.GetUserID())
	if ierr != nil {
		return c.Status(themeErrorStatus(ierr)).JSON(apitypes.NewErrorResponse(themeErrorMessage(ierr, lang)))
	}
	return c.JSON(apitypes.NewSuccessResponse(res))
}

// RefreshTheme downloads one of the user's installed themes again; the files are shared, so every user who has the theme gets the update
// POST /api/themes/:code/refresh
func (h *ThemesHandler) RefreshTheme(c *fiber.Ctx) error {
	sess, lang, err := h.themeUser(c)
	if err != nil {
		return err
	}
	if !tables.UserHasTheme(h.db, h.dbType, sess.GetUserID(), c.Params("code")) {
		return c.Status(404).JSON(apitypes.NewErrorResponse(themeErrorMessage(themerepo.ErrNotInstalled, lang)))
	}
	res, rerr := h.installer.Refresh(context.Background(), c.Params("code"), sess.GetUserID())
	if rerr != nil {
		return c.Status(themeErrorStatus(rerr)).JSON(apitypes.NewErrorResponse(themeErrorMessage(rerr, lang)))
	}
	return c.JSON(apitypes.NewSuccessResponse(res))
}

// DeleteTheme removes an installed theme from the user's themes; its files are deleted when no user has it any more. Deleting a theme
// for every user is done on the Themes page (Delete permission).
// DELETE /api/themes/:code
func (h *ThemesHandler) DeleteTheme(c *fiber.Ctx) error {
	sess, lang, err := h.themeUser(c)
	if err != nil {
		return err
	}
	code := c.Params("code")
	removed, rerr := h.installer.Remove(sess.GetUserID(), code)
	if rerr != nil {
		return c.Status(themeErrorStatus(rerr)).JSON(apitypes.NewErrorResponse(themeErrorMessage(rerr, lang)))
	}
	if !removed {
		return c.Status(404).JSON(apitypes.NewErrorResponse(themeErrorMessage(themerepo.ErrNotInstalled, lang)))
	}
	return c.JSON(apitypes.NewSuccessResponse(map[string]string{"code": code}))
}

var assetNameRe = regexp.MustCompile(`^(preview|thumb|bg-[1-5]|tb-[1-5])$`)

// GetThemeAsset serves an image of an installed theme. The images were re-encoded on
// install; they are sent with their exact type, no sniffing and a CSP that allows nothing,
// so even a crafted file could not run as a page. URLs carry the theme's commit (?v=), so
// the browser may keep them.
// GET /api/themes/:code/assets/:name
func (h *ThemesHandler) GetThemeAsset(c *fiber.Ctx) error {
	sess := getSession(c)
	if sess == nil || sess.GetUserID() == "" {
		return c.SendStatus(fiber.StatusUnauthorized)
	}
	name := c.Params("name")
	if !assetNameRe.MatchString(name) || !tables.UserHasTheme(h.db, h.dbType, sess.GetUserID(), c.Params("code")) {
		return c.SendStatus(fiber.StatusNotFound)
	}
	var rec gtables.ThemeAsset
	rec.InitWithDBType(h.db, "", h.dbType)
	if !rec.GetByPK(types.NewCode(c.Params("code")), types.NewCode(name)) {
		return c.SendStatus(fiber.StatusNotFound)
	}
	ct := rec.Content_type.String()
	if ct != "image/jpeg" && ct != "image/png" {
		return c.SendStatus(fiber.StatusNotFound)
	}
	etag := `"` + rec.Sha256.String() + `"`
	c.Set("Cache-Control", "private, max-age=31536000, immutable")
	c.Set("ETag", etag)
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	c.Set("Content-Disposition", "inline")
	if c.Get("If-None-Match") == etag {
		return c.SendStatus(fiber.StatusNotModified)
	}
	c.Set("Content-Type", ct)
	return c.Send(rec.Data)
}

// themeUser checks the session for installing, updating or removing the user's own themes —
// every logged-in user may (the themes are per user; downloads are limited by the
// installer and the rate limit). On failure the response is written and returned as err.
func (h *ThemesHandler) themeUser(c *fiber.Ctx) (*session.Session, string, error) {
	sess := getSession(c)
	if sess == nil || sess.GetUserID() == "" {
		return nil, "en-US", c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}
	return sess, getLanguageOrDefault(sess), nil
}

// themeErrors maps install/refresh errors to their messages (messages.yaml)
var themeErrors = []struct {
	err    error
	key    string
	status int
}{
	{themes.ErrRepoURL, "THEME_ERR_URL", 400},
	{themes.ErrRateLimited, "THEME_ERR_RATE_LIMIT", 429},
	{themes.ErrNotFound, "THEME_ERR_NOT_FOUND", 404},
	{themes.ErrTooLarge, "THEME_ERR_TOO_LARGE", 400},
	{themes.ErrTreeTooLarge, "THEME_ERR_TOO_LARGE", 400},
	{themes.ErrNoThemeColors, "THEME_ERR_NO_COLORS", 400},
	{themes.ErrNoColors, "THEME_ERR_NO_COLORS", 400},
	{themerepo.ErrBusy, "THEME_ERR_BUSY", 409},
	{themerepo.ErrTooMany, "THEME_ERR_TOO_MANY", 400},
	{themerepo.ErrOwnerBlocked, "THEME_ERR_OWNER", 403},
	{themerepo.ErrNotInstalled, "THEME_ERR_NOT_INSTALLED", 404},
	{themerepo.ErrBuiltin, "THEME_ERR_BUILTIN", 400},
	{themerepo.ErrCodeTaken, "THEME_ERR_CODE_TAKEN", 409},
}

func themeErrorStatus(err error) int {
	for _, e := range themeErrors {
		if errors.Is(err, e.err) {
			return e.status
		}
	}
	return 502 // GitHub failed or sent something unusable
}

func themeErrorMessage(err error, lang string) string {
	for _, e := range themeErrors {
		if errors.Is(err, e.err) {
			return i18n.GetInstance().Message(e.key, lang)
		}
	}
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return appErr.Message(lang)
	}
	return i18n.GetInstance().MessageWithParams("THEME_ERR_FAILED", lang, err.Error())
}

// GetAppearance returns the user's appearance
// GET /api/appearance
func (h *ThemesHandler) GetAppearance(c *fiber.Ctx) error {
	sess := getSession(c)
	if sess == nil || sess.GetUserID() == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}
	rec, exists := h.load(sess.GetUserID())
	return c.JSON(apitypes.NewSuccessResponse(h.appearanceOf(rec, exists)))
}

// SaveAppearance changes the user's appearance; the table's triggers check the values
// PUT /api/appearance
func (h *ThemesHandler) SaveAppearance(c *fiber.Ctx) error {
	sess := getSession(c)
	if sess == nil || sess.GetUserID() == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}
	language := getLanguageOrDefault(sess)
	var in appearanceSettings
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
	}

	rec, exists := h.load(sess.GetUserID())
	table := gtables.UserPersonalizationTableName
	if in.LightTheme != nil {
		rec.Light_theme = types.NewCode(*in.LightTheme)
	}
	if in.DarkTheme != nil {
		rec.Dark_theme = types.NewCode(*in.DarkTheme)
	}
	if in.Mode != nil {
		i := optionIndex(appearanceModes, *in.Mode)
		if i < 0 {
			return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.FieldOutOfRange(table, "mode", "dark", "light").Message(language)))
		}
		rec.Mode = gtables.UserPersonalizationMode(i)
	}
	if in.Wallpaper != nil {
		rec.Wallpaper = types.NewText(*in.Wallpaper)
	}
	if in.WallpaperOpacity != nil {
		rec.Wallpaper_opacity = *in.WallpaperOpacity
	}
	if in.WallpaperBlur != nil {
		rec.Wallpaper_blur = *in.WallpaperBlur
	}
	if in.PanelOpacity != nil {
		rec.Panel_opacity = *in.PanelOpacity
	}
	if in.Font != nil {
		i := optionIndex(appearanceFonts, *in.Font)
		if i < 0 {
			return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidValue("font", *in.Font).Message(language)))
		}
		rec.Font = gtables.UserPersonalizationFont(i)
	}
	if in.FontSize != nil {
		rec.Font_size = *in.FontSize
	}
	if in.Density != nil {
		i := optionIndex(appearanceDensities, *in.Density)
		if i < 0 {
			return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidValue("density", *in.Density).Message(language)))
		}
		rec.Density = gtables.UserPersonalizationDensity(i)
	}
	if in.AmountFont != nil {
		i := optionIndex(appearanceAmountFonts, *in.AmountFont)
		if i < 0 {
			return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidValue("amount_font", *in.AmountFont).Message(language)))
		}
		rec.Amount_font = gtables.UserPersonalizationAmount_font(i)
	}
	rec.Updated_at = types.NewDateTimeFromTime(time.Now())

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
		return c.Status(500).JSON(apitypes.NewErrorResponse(apperrors.ModifyFailed(table, sess.GetUserID()).Message(language)))
	}
	return c.JSON(apitypes.NewSuccessResponse(h.appearanceOf(rec, true)))
}

// load reads the user's record; a user without one gets the defaults (exists = false).
func (h *ThemesHandler) load(userID string) (*gtables.UserPersonalization, bool) {
	rec := &gtables.UserPersonalization{}
	rec.InitWithDBType(h.db, "", h.dbType)
	if rec.GetByPK(types.NewCode(userID)) {
		return rec, true
	}
	def := tables.DefaultUserPersonalization(userID)
	def.InitWithDBType(h.db, "", h.dbType)
	return def, false
}

// appearanceOf builds the response; a slot whose theme no longer exists falls back to the
// default theme of its mode.
func (h *ThemesHandler) appearanceOf(rec *gtables.UserPersonalization, saved bool) appearanceResponse {
	slot := func(code, fallback string) themes.Theme {
		if t, ok := tables.UserThemeByCode(h.db, h.dbType, rec.User_id.String(), code); ok {
			return t
		}
		t, _ := themes.Builtin(fallback)
		return t
	}
	light := slot(rec.Light_theme.String(), themes.DefaultLight)
	dark := slot(rec.Dark_theme.String(), themes.DefaultDark)
	str := func(s string) *string { return &s }
	num := func(n int) *int { return &n }
	return appearanceResponse{
		Saved: saved,
		Settings: appearanceSettings{
			LightTheme:       str(light.Code),
			DarkTheme:        str(dark.Code),
			Mode:             str(optionName(appearanceModes, int(rec.Mode))),
			Wallpaper:        str(rec.Wallpaper.String()),
			WallpaperOpacity: num(rec.Wallpaper_opacity),
			WallpaperBlur:    num(rec.Wallpaper_blur),
			PanelOpacity:     num(rec.Panel_opacity),
			Font:             str(optionName(appearanceFonts, int(rec.Font))),
			FontSize:         num(rec.Font_size),
			Density:          str(optionName(appearanceDensities, int(rec.Density))),
			AmountFont:       str(optionName(appearanceAmountFonts, int(rec.Amount_font))),
		},
		Light: light,
		Dark:  dark,
	}
}

func optionIndex(names []string, name string) int {
	for i, n := range names {
		if n == name {
			return i
		}
	}
	return -1
}

func optionName(names []string, i int) string {
	if i >= 0 && i < len(names) {
		return names[i]
	}
	return names[0]
}
