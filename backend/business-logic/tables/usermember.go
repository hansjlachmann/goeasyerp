package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// userMemberLogic is the business logic of table 2000000003 User_Member: a user's role in a
// company (blank: all companies). Only a user who is SUPER in that company gives or removes
// one (securityadmin.go).
type userMemberLogic struct{}

func init() { gtables.RegisterUserMemberLogic(userMemberLogic{}) }

// OnInsert: the role is given by a SUPER user of its company
func (userMemberLogic) OnInsert(rec *gtables.UserMember) error {
	return checkMemberAdmin(rec, rec.Company.String())
}

// OnModify: the role before and after is given by a SUPER user of its company
func (userMemberLogic) OnModify(rec *gtables.UserMember) error {
	err := checkMemberAdmin(rec, rec.XRec().Company.String())
	if err != nil {
		return err
	}
	return checkMemberAdmin(rec, rec.Company.String())
}

// OnDelete: the role is removed by a SUPER user of its company
func (userMemberLogic) OnDelete(rec *gtables.UserMember) error {
	return checkMemberAdmin(rec, rec.Company.String())
}

// OnRename: the role before and after is given by a SUPER user of its company
func (userMemberLogic) OnRename(rec *gtables.UserMember) error {
	err := checkMemberAdmin(rec, rec.XRec().Company.String())
	if err != nil {
		return err
	}
	return checkMemberAdmin(rec, rec.Company.String())
}

// checkMemberAdmin refuses a user who is not SUPER in company
func checkMemberAdmin(rec *gtables.UserMember, company string) error {
	if !superIn(rec, company) {
		return apperrors.MemberSuperOnly(company)
	}
	session.SecurityChanged()
	return nil
}
