package tables

import (
	"strings"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
)

// ownUserRowOnly: a user's own rows of a per-user table (User Preferences, User
// Personalization) are changed by that user only — or a SUPER user (to reset another user's
// layout); code without a user is allowed
func ownUserRowOnly(tableName, userID string) error {
	access := fcodeunits.CurrentAccess()
	if access == nil || access.IsSuper || strings.EqualFold(access.GetUserID(), userID) {
		return nil
	}
	return apperrors.OwnUserRowsOnly(tableName, userID)
}
