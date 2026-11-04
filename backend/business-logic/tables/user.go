package tables

import (
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// userLogic is the business logic of table 2000000002 User
type userLogic struct{}

func init() { gtables.RegisterUserLogic(userLogic{}) }

// OnDelete deletes the user's preferences, appearance and theme list so they don't
// remain orphaned. They are global tables (company ""), deleted regardless of the
// session's company. SetRange is an exact match, so a user ID containing filter
// characters (*, |, ..) cannot widen the delete. An installed theme nobody has any more
// stays until it is deleted on the Themes page.
func (userLogic) OnDelete(rec *gtables.User) error {
	db, dbType, userID := rec.GetDB(), rec.GetDBType(), rec.User_id.String()

	var prefs gtables.UserPreferences
	prefs.InitWithDBType(db, "", dbType)
	prefs.SetRange("user_id", userID)
	prefs.DeleteAll()

	var pers gtables.UserPersonalization
	pers.InitWithDBType(db, "", dbType)
	pers.SetRange("user_id", userID)
	pers.DeleteAll()

	var links gtables.UserTheme
	links.InitWithDBType(db, "", dbType)
	links.SetRange("user_id", userID)
	links.DeleteAll()
	return nil
}

// SetPassword hashes and stores the user's password
func SetPassword(user *gtables.User, password string) error {
	if password == "" {
		return apperrors.FieldRequired(gtables.UserTableName, "password")
	}
	if utf8.RuneCountInString(password) < 6 {
		return apperrors.FieldTooShort(gtables.UserTableName, "password", 6)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.Password_hash = types.NewText(string(hashedPassword))
	return nil
}

// CheckPassword verifies if the provided password matches the user's stored hash
func CheckPassword(user *gtables.User, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(user.Password_hash.String()), []byte(password))
	return err == nil
}

// UpdateLastLogin sets the user's last login timestamp to now
func UpdateLastLogin(user *gtables.User) {
	user.Last_login = types.NewDateTimeFromTime(time.Now())
}
