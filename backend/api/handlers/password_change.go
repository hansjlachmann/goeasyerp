package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/goeasyerp/backend/api/middleware"
	apitypes "github.com/hansjlachmann/goeasyerp/backend/api/types"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/securitylog"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// ChangePassword lets the logged-in user change their own password (S38): the current password
// is asked (wrong ones count as failed logins of the user from this address), the new one must
// meet the password rules, and every other session of the user ends (Sessions Valid From) —
// this browser gets a new token and stays logged in.
// POST /api/auth/password {current_password, new_password}
func (h *AuthHandler) ChangePassword(c *fiber.Ctx) error {
	sess := getSession(c)
	if sess == nil || sess.GetUserID() == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}
	language := getLanguageOrDefault(sess)
	var requestBody struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := c.BodyParser(&requestBody); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
	}

	userID := sess.GetUserID()
	address := middleware.ClientKey(c) // as the login guard counts it (an IPv6 /64)
	if logins.locked(userID, address) {
		recordSecurity(c, securitylog.PasswordChangeFailed, userID, sess.GetCompany(), "address locked")
		return c.Status(fiber.StatusTooManyRequests).JSON(apitypes.NewErrorResponse(apperrors.TooManyRequests().Message(language)))
	}
	var user gtables.User
	user.InitWithDBType(h.db, "", h.dbType) // User is a global table
	if !user.Get(types.NewCode(userID)) {
		return c.Status(fiber.StatusNotFound).JSON(apitypes.NewErrorResponse(apperrors.UserNotFound().Message(language)))
	}
	if !tables.CheckPassword(&user, requestBody.CurrentPassword) {
		failedLogin(c, securitylog.PasswordChangeFailed, userID, sess.GetCompany(), "wrong current password")
		return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(apperrors.PasswordCurrentInvalid().Message(language)))
	}
	if err := tables.SetPassword(&user, requestBody.NewPassword); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
	}
	// every token issued so far ends (other browsers, a copied cookie); this one is issued anew
	// below, in this second or later, so it stays good
	user.Sessions_valid_from = types.NewDateTimeFromTime(time.Now().Truncate(time.Second))
	if !user.Modify(false) {
		return c.Status(fiber.StatusInternalServerError).JSON(apitypes.NewErrorResponse(apperrors.TableModifyFailed(gtables.UserTableName, userID).Message(language)))
	}
	logins.succeeded(userID, address)
	recordSecurity(c, securitylog.PasswordChanged, userID, sess.GetCompany(), "other sessions ended")
	session.SecurityChanged()
	h.sessionCache.RemoveByUser(userID)

	company := sess.GetCompany()
	newSess := session.NewSession(database.WrapConnection(h.db, h.dbType), company, nil)
	newSess.SetUser(userID, sess.GetUserName(), sess.GetLanguage(), sess.GetMenu())
	newSess.SetPermissions(sess.HasRoles, sess.IsSuper, sess.Permissions)
	newSess.SetLoginTime(sess.GetLoginTime())
	token, err := middleware.GenerateToken(h.jwtConfig, userID, sess.GetUserName(), company, sess.GetLanguage(), sess.GetMenu(), sess.GetLoginTime())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(apitypes.NewErrorResponse("Failed to create session token"))
	}
	middleware.SetAuthCookie(c, h.jwtConfig, token)
	h.sessionCache.Set(userID+":"+company, newSess, h.jwtConfig.TokenExpiry)
	return c.JSON(apitypes.NewSuccessResponse(map[string]interface{}{"user_id": userID}))
}
