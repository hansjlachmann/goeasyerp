// Package refusedposting logs the postings the balance check refused (table 904 Refused
// Posting): Commit refused a transaction whose G/L entries would not have balanced — a bug in a
// posting, which an administrator should see even when the user only closed the error. Show
// Error on the log's page (tables.RefusedPostingShowError) shows a refusal again, with its
// details (Show Entries).
package refusedposting

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"

	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Register logs every refused commit of the server's database into the request's company
// (its goroutine context: the user and company; none, e.g. demo data being created: no log)
func Register(db *sql.DB, dbType database.DBType) {
	ftables.OnRefusedCommit(func(err error) {
		company := fcodeunits.CurrentCompany()
		if company == "" {
			return
		}
		logErr := Log(db, dbType, company, fcodeunits.CurrentUserID(), fcodeunits.CurrentLanguage(), err)
		if logErr != nil {
			log.Printf("refused posting of %s not logged: %v (the refusal: %v)", company, logErr, err)
		}
	})
}

// Log writes a refused posting — the error in the user's language, a G/L refusal's document,
// posting date and difference, and the error's details — in a transaction of its own
func Log(db *sql.DB, dbType database.DBType, company, userID, language string, refusal error) error {
	tx, err := ftables.BeginTransaction(db)
	if err != nil {
		return err
	}
	refusedPosting := gtables.NewRefusedPosting(ftables.DBSource(tx, company, dbType))
	ftables.LockTable(tx, dbType, company, gtables.RefusedPostingTableName)
	entryNo := 1
	if refusedPosting.FindLast() {
		entryNo = refusedPosting.Entry_no + 1
	}
	refusedPosting.Init()
	refusedPosting.Entry_no = entryNo
	refusedPosting.Refused_at = types.Now()
	refusedPosting.User_id = types.Code(system.CopyStr(userID, 1, 50))
	refusedPosting.Message = types.Text(system.CopyStr(refusal.Error(), 1, 250))
	var appErr *apperrors.AppError
	if errors.As(refusal, &appErr) {
		refusedPosting.Error_code = types.Code(appErr.Code)
		refusedPosting.Message = types.Text(system.CopyStr(appErr.Message(language), 1, 250))
		if appErr.Code == apperrors.ErrGLNotBalanced && len(appErr.Params) == 3 {
			refusedPosting.Document_no = types.Code(system.CopyStr(appErr.Params[0], 1, 20))
			refusedPosting.Posting_date = types.Text(system.CopyStr(appErr.Params[1], 1, 30))
			refusedPosting.Difference = types.Text(system.CopyStr(appErr.Params[2], 1, 30))
		}
		if appErr.Details != nil {
			refusedPosting.Details, err = json.Marshal(appErr.Details)
			if err != nil {
				_ = ftables.Rollback(tx)
				return err
			}
		}
	}
	err = refusedPosting.InsertOrError(false)
	if err != nil {
		_ = ftables.Rollback(tx)
		return err
	}
	return ftables.Commit(tx)
}
