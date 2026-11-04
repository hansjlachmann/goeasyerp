package refusedposting

import (
	"database/sql"
	"errors"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// A commit the balance check refuses in a request of a company is logged there — the user,
// the error, the document, date and difference and the details — and Show Error gives the
// error back with its details; without a company (no request) nothing is logged
func TestRefusedCommitIsLogged(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := (&gtables.RefusedPosting{}).CreateTableWithDBType(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	Register(db, database.DBTypeSQLite)

	refuse := func() error {
		tx, err := ftables.BeginTransaction(db)
		if err != nil {
			t.Fatal(err)
		}
		details := &apperrors.Details{Title: "G/L entries", Sections: []apperrors.DetailsSection{{Caption: "Entries", Columns: []apperrors.DetailsColumn{{Caption: "Amount", Numeric: true}}, Rows: [][]string{{"100.00"}}}}}
		ftables.OnInconsistent(tx, "G_L_Entry", func() error {
			return apperrors.GLNotBalanced("SI000123", "11/03/2026", "100.00").WithDetails(details)
		})
		ftables.SetConsistent(tx, "G_L_Entry", false)
		return ftables.Commit(tx)
	}

	if err := refuse(); err == nil {
		t.Fatal("the commit was not refused")
	}
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	if n := gtables.NewRefusedPosting(src).Count(); n != 0 {
		t.Fatalf("%d refusals logged without a request", n)
	}

	fcodeunits.SetCurrentContext("ANNA", "Anna Berg", "c1", "en-US")
	t.Cleanup(fcodeunits.ClearCurrentContext)
	if err := refuse(); err == nil {
		t.Fatal("the commit was not refused")
	}
	refusedPosting := gtables.NewRefusedPosting(src)
	if !refusedPosting.FindLast() || refusedPosting.Entry_no != 1 || refusedPosting.User_id != "ANNA" ||
		refusedPosting.Document_no != "SI000123" || refusedPosting.Difference != "100.00" ||
		refusedPosting.Error_code != "ERR_GL_NOT_BALANCED" || refusedPosting.Message == "" || len(refusedPosting.Details) == 0 {
		t.Fatalf("logged: %+v", refusedPosting.ToMap())
	}

	_, err = tables.RefusedPostingShowError(refusedPosting)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.ErrGLNotBalanced || appErr.Params[0] != "SI000123" ||
		appErr.Details == nil || appErr.Details.Sections[0].Rows[0][0] != "100.00" {
		t.Errorf("Show Error: %v %+v", err, appErr)
	}
}
