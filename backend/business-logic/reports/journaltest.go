package reports

import (
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// jnlTest is what the journals' test reports (2 General Journal, 702 Inventory Posting, 802 BOM
// Journal, 1102 Resource Journal, 1202 Project Journal) share: the warnings printed under a
// line (Error_Loop), the posting date's checks (closing date, date order of the batch's number
// series, the user's allowed posting dates), the gap in the batch's number series and the
// recurring journals' fields. Each report's texts have the keys used here.
type jnlTest struct {
	errors                           []string
	noSeries                         *gtables.NoSeries
	lastPostingDate                  types.Date
	lastDocNo                        types.Code
	allowPostingFrom, allowPostingTo types.Date
}

// addError adds a warning to the current line's (AddError)
func (t *jnlTest) addError(text string) {
	t.errors = append(t.errors, text)
}

// recordFilters is a journal line's filters (the recurring journal's own filters)
type recordFilters interface {
	GetFilter(field string) string
	SetRange(field string, values ...interface{})
	SetFilter(field, expression string)
	GetFieldCaption(field, language string) string
}

// startBatch (the lines' OnPreDataItem): a recurring journal's lines up to the work date, not
// filtered on the posting or expiration date (else a warning); the batch's number series; no
// line yet
func (t *jnlTest) startBatch(run *Run, line recordFilters, recurring bool, batchNoSeries types.Code) {
	language := fcodeunits.CurrentLanguage()
	t.errors = nil
	if recurring {
		if line.GetFilter("posting_date") != "" {
			t.addError(run.Text("CANNOT_BE_FILTERED", line.GetFieldCaption("posting_date", language)))
		}
		line.SetRange("posting_date", types.Date{}, system.WorkDate())
		if line.GetFilter("expiration_date") != "" {
			t.addError(run.Text("CANNOT_BE_FILTERED", line.GetFieldCaption("expiration_date", language)))
		}
		line.SetFilter("expiration_date", "''|"+system.WorkDate().String()+"..")
	}
	t.noSeries = nil
	if batchNoSeries != "" {
		t.noSeries = gtables.NewNoSeries(run)
		if !t.noSeries.Get(batchNoSeries) {
			t.noSeries.Init()
		}
	}
	t.lastPostingDate = types.Date{}
	t.lastDocNo = ""
}

// checkPostingDate: the posting date is specified, no closing date, in the date order of the
// batch's number series and in the user's allowed range
func (t *jnlTest) checkPostingDate(run *Run, postingDate types.Date, caption string) {
	if postingDate.IsZero() {
		t.addError(run.Text("MUST_BE_SPECIFIED", caption))
		return
	}
	if postingDate.IsClosing() {
		t.addError(run.Text("NOT_CLOSING_DATE", caption))
	}
	if t.noSeries != nil && t.noSeries.Date_order && postingDate.Before(t.lastPostingDate) {
		t.addError(run.Text("NOT_IN_DATE_ORDER"))
	}
	t.lastPostingDate = postingDate
	if t.dateNotAllowed(run, postingDate) {
		t.addError(run.Text("DATE_NOT_ALLOWED", formatDate(postingDate, fcodeunits.CurrentLanguage())))
	}
}

// checkDocumentDate: a document date is no closing date
func (t *jnlTest) checkDocumentDate(run *Run, documentDate types.Date, caption string) {
	if !documentDate.IsZero() && documentDate.IsClosing() {
		t.addError(run.Text("NOT_CLOSING_DATE", caption))
	}
}

// checkDocumentNo: with a number series on the batch, the next line's document no. is the same
// or the next number
func (t *jnlTest) checkDocumentNo(run *Run, batchNoSeries, documentNo types.Code) {
	if batchNoSeries == "" {
		return
	}
	if t.lastDocNo != "" && documentNo != t.lastDocNo && documentNo.String() != system.IncStr(t.lastDocNo.String()) {
		t.addError(run.Text("GAP_IN_NO_SERIES"))
	}
	t.lastDocNo = documentNo
}

// recurringLine is what CheckRecurringLine looks at on a journal line
type recurringLine struct {
	recurring      bool // the template is recurring
	methodBlank    bool
	methodVariable bool
	frequency      types.Code
	quantityZero   bool // the amount (quantity) a variable method needs
	quantityField  string
}

// checkRecurringLine (CheckRecurringLine): a recurring journal's line has a recurring method and
// frequency (a variable one an amount or quantity); another journal's line has neither
func (t *jnlTest) checkRecurringLine(run *Run, line recordFilters, r recurringLine) {
	language := fcodeunits.CurrentLanguage()
	caption := func(field string) string { return line.GetFieldCaption(field, language) }
	if r.recurring {
		if r.methodBlank {
			t.addError(run.Text("MUST_BE_SPECIFIED", caption("recurring_method")))
		}
		if r.frequency == "" {
			t.addError(run.Text("MUST_BE_SPECIFIED", caption("recurring_frequency")))
		}
		if r.methodVariable && r.quantityZero {
			t.addError(run.Text("MUST_BE_SPECIFIED", caption(r.quantityField)))
		}
		return
	}
	if !r.methodBlank {
		t.addError(run.Text("CANNOT_BE_SPECIFIED", caption("recurring_method")))
	}
	if r.frequency != "" {
		t.addError(run.Text("CANNOT_BE_SPECIFIED", caption("recurring_frequency")))
	}
}

// dateNotAllowed: the posting date is outside the user's allowed posting dates (User Setup,
// else General Ledger Setup; no end: no limit)
func (t *jnlTest) dateNotAllowed(run *Run, postingDate types.Date) bool {
	if t.allowPostingFrom.IsZero() && t.allowPostingTo.IsZero() {
		if system.UserId() != "" {
			userSetup := gtables.NewUserSetup(run)
			if userSetup.Get(types.Code(system.UserId())) {
				t.allowPostingFrom = userSetup.Allow_posting_from
				t.allowPostingTo = userSetup.Allow_posting_to
			}
		}
		if t.allowPostingFrom.IsZero() && t.allowPostingTo.IsZero() {
			glSetup := gtables.GetGeneralLedgerSetup(run)
			t.allowPostingFrom = glSetup.Allow_posting_from
			t.allowPostingTo = glSetup.Allow_posting_to
		}
		if t.allowPostingTo.IsZero() {
			t.allowPostingTo, _ = system.DMY2Date(31, 12, 9999)
		}
	}
	return postingDate.Before(t.allowPostingFrom) || t.allowPostingTo.Before(postingDate)
}

// checkGLAccount: the account exists and is not blocked
func (t *jnlTest) checkGLAccount(run *Run, no types.Code) {
	glAcc := gtables.NewGLAccount(run)
	if !glAcc.Get(no) {
		t.addError(run.Text("GL_ACCOUNT_NOT_EXIST", no))
	} else if glAcc.Blocked {
		language := fcodeunits.CurrentLanguage()
		t.addError(run.Text("MUST_BE_FOR_ACCOUNT", glAcc.GetFieldCaption("blocked", language), formatValue(false, "", language), no))
	}
}

// checkGenPostingSetup: the general posting setup of the groups exists (a product group must be
// specified)
func (t *jnlTest) checkGenPostingSetup(run *Run, busGroup, prodGroup types.Code, prodCaption string) {
	if prodGroup == "" {
		t.addError(run.Text("MUST_BE_SPECIFIED", prodCaption))
		return
	}
	genPostingSetup := gtables.NewGeneralPostingSetup(run)
	if !genPostingSetup.Get(map[string]interface{}{"gen_bus_posting_group": busGroup, "gen_prod_posting_group": prodGroup}) {
		t.addError(run.Text("SETUP_NOT_EXIST", genPostingSetup.TableCaption(fcodeunits.CurrentLanguage()), busGroup, prodGroup))
	}
}

// errorLoop is the Error_Loop data item: the line's warnings, one per row
func (t *jnlTest) errorLoop() *DataItem {
	return &DataItem{
		Name: "Error_Loop", Table: IntegerTable,
		OnPreDataItem: Trigger(func(rec *Integer, run *Run) error {
			rec.SetRange("number", 1, len(t.errors))
			return nil
		}),
		OnAfterGetRecord: Trigger(func(rec *Integer, run *Run) error {
			run.SetValue("error_text", t.errors[rec.Number-1])
			return nil
		}),
	}
}

// jnlTestBatch is the batch data item's OnAfterGetRecord check: a batch without lines prints
// nothing (PrintOnlyIfDetail); each batch starts on page 1
func jnlTestBatchHasLines(run *Run, isEmpty bool) error {
	if isEmpty {
		return Skip()
	}
	run.SetPageNo(1)
	return nil
}
