package tables

import (
	"regexp"

	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/themes"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// userPersonalizationLogic is the business logic of table 2000000073 User_Personalization (User
// Personalization): a user's appearance — the theme for light mode and the one for dark
// mode, the current mode, wallpaper, font and density. One record per user, global (the
// same in every company); a user without a record gets DefaultUserPersonalization.
type userPersonalizationLogic struct{}

func init() { gtables.RegisterUserPersonalizationLogic(userPersonalizationLogic{}) }

var wallpaperName = regexp.MustCompile(`^bg-[1-5]$`)

// Limits of the appearance settings
const (
	WallpaperOpacityMax = 100
	WallpaperBlurMax    = 20
	PanelOpacityMin     = 75 // panels stay opaque enough to read lists on any wallpaper
	FontSizeMin         = 80 // percent of the default; 0 = default
	FontSizeMax         = 150
)

// DefaultUserPersonalization returns the appearance of a user without a record: Classic
// Light/Dark, dark mode (as before themes existed), no wallpaper.
func DefaultUserPersonalization(userID string) *gtables.UserPersonalization {
	p := &gtables.UserPersonalization{}
	p.User_id = types.NewCode(userID)
	p.Light_theme = types.NewCode(themes.DefaultLight)
	p.Dark_theme = types.NewCode(themes.DefaultDark)
	p.Mode = gtables.UserPersonalization_Mode.Dark
	p.Wallpaper_opacity = 25
	p.Panel_opacity = 92
	return p
}

func (userPersonalizationLogic) OnInsert(rec *gtables.UserPersonalization) error {
	return checkAppearance(rec)
}

func (userPersonalizationLogic) OnModify(rec *gtables.UserPersonalization) error {
	return checkAppearance(rec)
}

// OnValidate_Light_theme checks that the light theme exists and is made for light mode
func (userPersonalizationLogic) OnValidate_Light_theme(rec *gtables.UserPersonalization) error {
	return checkThemeSlot(rec, "light_theme", rec.Light_theme.String(), "light")
}

// OnValidate_Dark_theme checks that the dark theme exists and is made for dark mode
func (userPersonalizationLogic) OnValidate_Dark_theme(rec *gtables.UserPersonalization) error {
	return checkThemeSlot(rec, "dark_theme", rec.Dark_theme.String(), "dark")
}

// checkAppearance checks the whole record: both themes and the limits of the settings
func checkAppearance(rec *gtables.UserPersonalization) error {
	err := checkThemeSlot(rec, "light_theme", rec.Light_theme.String(), "light")
	if err != nil {
		return err
	}
	err = checkThemeSlot(rec, "dark_theme", rec.Dark_theme.String(), "dark")
	if err != nil {
		return err
	}
	table := gtables.UserPersonalizationTableName
	err = ftables.CheckRange(table, "wallpaper_opacity", rec.Wallpaper_opacity, 0, WallpaperOpacityMax)
	if err != nil {
		return err
	}
	err = ftables.CheckRange(table, "wallpaper_blur", rec.Wallpaper_blur, 0, WallpaperBlurMax)
	if err != nil {
		return err
	}
	err = ftables.CheckRange(table, "panel_opacity", rec.Panel_opacity, PanelOpacityMin, 100)
	if err != nil {
		return err
	}
	if rec.Font_size != 0 {
		err := ftables.CheckRange(table, "font_size", rec.Font_size, FontSizeMin, FontSizeMax)
		if err != nil {
			return err
		}
	}
	// Wallpaper: blank (none) or one of the active theme's wallpapers bg-1 … bg-5
	w := rec.Wallpaper.String()
	if w != "" && !wallpaperName.MatchString(w) {
		return apperrors.FieldFormat(table, "wallpaper", "bg-1 … bg-5")
	}
	if !rec.Mode.IsValid() {
		return apperrors.FieldOutOfRange(table, "mode", "0", "1")
	}
	if !rec.Font.IsValid() {
		return apperrors.FieldOutOfRange(table, "font", "0", "5")
	}
	if !rec.Density.IsValid() {
		return apperrors.FieldOutOfRange(table, "density", "0", "1")
	}
	return nil
}

func checkThemeSlot(rec *gtables.UserPersonalization, field, code, mode string) error {
	table := gtables.UserPersonalizationTableName
	theme, ok := UserThemeByCode(rec.GetDB(), rec.GetDBType(), rec.User_id.String(), code)
	if !ok {
		return apperrors.RelatedNotFound(table, field, code)
	}
	if theme.Mode != mode {
		return apperrors.ThemeWrongMode(table, field, theme.Name)
	}
	return nil
}
