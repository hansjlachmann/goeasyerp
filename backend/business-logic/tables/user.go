package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/passwords"
	"time"

	"golang.org/x/crypto/bcrypt"

	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// userLogic is the business logic of table 2000000002 User
type userLogic struct{}

func init() { gtables.RegisterUserLogic(userLogic{}) }

// OnModify: another user is changed (password, active …) only by a SUPER user of every company
// where that user has roles (securityadmin.go); users change their own record
func (userLogic) OnModify(rec *gtables.User) error {
	xRec := rec.XRec()
	err := checkUserAdmin(rec, xRec.User_id.String())
	if err != nil {
		return err
	}
	revokeUserSessions(rec, xRec)
	return nil
}

// revokeUserSessions: a user who is deactivated, or whose password another user set, is logged
// out everywhere — the tokens issued so far are no longer accepted (Sessions Valid From)
func revokeUserSessions(rec, xRec *gtables.User) {
	deactivated := xRec.Active && !rec.Active
	passwordSet := rec.Password_hash != xRec.Password_hash && !isCurrentUser(xRec.User_id.String())
	if deactivated || passwordSet {
		rec.Sessions_valid_from = types.NewDateTimeFromTime(time.Now().Truncate(time.Second))
	}
	if rec.Active != xRec.Active || deactivated || passwordSet {
		session.SecurityChanged()
	}
}

// OnRename: as OnModify
func (userLogic) OnRename(rec *gtables.User) error {
	return checkUserAdmin(rec, rec.XRec().User_id.String())
}

// checkUserAdmin refuses changing another user without being SUPER wherever that user has
// roles
func checkUserAdmin(rec *gtables.User, userID string) error {
	if isCurrentUser(userID) {
		return nil
	}
	if !canAdministerUser(rec, userID) {
		return apperrors.UserSuperOnly(userID)
	}
	return nil
}

// OnDelete deletes the user's preferences, appearance and theme list so they don't
// remain orphaned. They are global tables (company ""), deleted regardless of the
// session's company. SetRange is an exact match, so a user ID containing filter
// characters (*, |, ..) cannot widen the delete. An installed theme nobody has any more
// stays until it is deleted on the Themes page.
func (userLogic) OnDelete(rec *gtables.User) error {
	err := checkUserAdmin(rec, rec.User_id.String())
	if err != nil {
		return err
	}
	session.SecurityChanged()
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
	// the password rules (S38): long enough, not a common password, not the user ID
	problem := passwords.Check(password, user.User_id.String())
	switch problem {
	case passwords.TooShort:
		return apperrors.FieldTooShort(gtables.UserTableName, "password", passwords.MinLength)
	case passwords.TooCommon:
		return apperrors.PasswordTooCommon()
	case passwords.HasUserID:
		return apperrors.PasswordHasUserID()
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
