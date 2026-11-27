package tables

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// userPreferencesLogic is the business logic of table 2000000075 User_Preferences.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/userpreferences_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type userPreferencesLogic struct{}

func init() { gtables.RegisterUserPreferencesLogic(userPreferencesLogic{}) }

// OnInsert: a user's own preferences only (or a SUPER user)
func (userPreferencesLogic) OnInsert(rec *gtables.UserPreferences) error {
	return ownUserRowOnly(rec.TableCaption(fcodeunits.CurrentLanguage()), rec.User_id.String())
}

// OnModify: a user's own preferences only (or a SUPER user)
func (userPreferencesLogic) OnModify(rec *gtables.UserPreferences) error {
	return ownUserRowOnly(rec.TableCaption(fcodeunits.CurrentLanguage()), rec.User_id.String())
}

// OnDelete: a user's own preferences only (or a SUPER user)
func (userPreferencesLogic) OnDelete(rec *gtables.UserPreferences) error {
	return ownUserRowOnly(rec.TableCaption(fcodeunits.CurrentLanguage()), rec.User_id.String())
}

// OnRename: neither the old nor the new user may be another user (or a SUPER user)
func (userPreferencesLogic) OnRename(rec *gtables.UserPreferences) error {
	err := ownUserRowOnly(rec.TableCaption(fcodeunits.CurrentLanguage()), rec.XRec().User_id.String())
	if err != nil {
		return err
	}
	return ownUserRowOnly(rec.TableCaption(fcodeunits.CurrentLanguage()), rec.User_id.String())
}
