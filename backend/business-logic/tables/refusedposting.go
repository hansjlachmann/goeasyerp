package tables

import (
	"encoding/json"

	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/functions"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// refusedPostingLogic is the business logic of table 904 Refused Posting (written by
// refusedposting.Log when the balance check refuses a posting).
type refusedPostingLogic struct{}

func init() {
	gtables.RegisterRefusedPostingLogic(refusedPostingLogic{})
	functions.Register("table", gtables.RefusedPostingTableName, "ShowError", functions.For(RefusedPostingShowError), functions.WithDialog())
}

// OnInsert runs before a new record is inserted
func (refusedPostingLogic) OnInsert(rec *gtables.RefusedPosting) error {
	return nil
}

// OnModify runs before a record is modified
func (refusedPostingLogic) OnModify(rec *gtables.RefusedPosting) error {
	return nil
}

// OnDelete runs before a record is deleted
func (refusedPostingLogic) OnDelete(rec *gtables.RefusedPosting) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (refusedPostingLogic) OnRename(rec *gtables.RefusedPosting) error {
	return nil
}

// ShowError (Show Error on page 904): the refusal again as the error it was — a G/L refusal
// with its document, date, difference and details (Show Entries)
func RefusedPostingShowError(rec *gtables.RefusedPosting) (fcodeunits.Result, error) {
	refusedPosting := gtables.NewRefusedPosting(rec)
	err := refusedPosting.GetOrError(rec.Entry_no)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	var details *apperrors.Details
	if len(refusedPosting.Details) > 0 {
		details = &apperrors.Details{}
		if json.Unmarshal(refusedPosting.Details, details) != nil {
			details = nil
		}
	}
	if refusedPosting.Error_code == types.Code(apperrors.ErrGLNotBalanced) {
		return fcodeunits.Result{}, apperrors.GLNotBalanced(refusedPosting.Document_no.String(), refusedPosting.Posting_date.String(), refusedPosting.Difference.String()).WithDetails(details)
	}
	return fcodeunits.Result{}, system.Error(refusedPosting.Message.String())
}
