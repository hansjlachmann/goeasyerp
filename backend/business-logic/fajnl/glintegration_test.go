package fajnl_test

import (
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/fajnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/genjnl"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// newAsset: a fixed asset FA0090 in the COMPANY book (G/L integration on) of posting group
// EQUIPMENT
func newAsset(t *testing.T, src ftables.Source) {
	t.Helper()
	fa := gtables.NewFixedAsset(src)
	fa.No = "FA0090"
	fa.Description = "Packing machine"
	must(t, fa.InsertOrError(true))
	faDeprBook := gtables.NewFADepreciationBook(src)
	faDeprBook.Fa_no = "FA0090"
	faDeprBook.Depreciation_book_code = "COMPANY"
	must(t, faDeprBook.InsertOrError(true))
	must(t, faDeprBook.Validate("fa_posting_group", types.Code("EQUIPMENT")))
	must(t, faDeprBook.ModifyOrError(true))
}

// faLine: a general journal line for FA0090 with a posting type and amount, balanced by a G/L
// account (blank: no balancing account)
func faLine(t *testing.T, src ftables.Source, lineNo int, postingType gtables.GenJournalLineFa_posting_type, amount, balAccount string) *gtables.GenJournalLine {
	t.Helper()
	line := gtables.NewGenJournalLine(src)
	line.Line_no = lineNo
	line.Posting_date = date("2026-07-01")
	line.Document_no = "FAGL-1"
	must(t, line.Validate("account_type", gtables.GenJournalLine_Account_type.FixedAsset))
	must(t, line.Validate("account_no", types.Code("FA0090")))
	must(t, line.Validate("fa_posting_type", postingType))
	must(t, line.Validate("amount", dec(amount)))
	if balAccount != "" {
		must(t, line.Validate("bal_account_no", types.Code(balAccount)))
	}
	return line
}

// glEntries: the G/L entries of the document by account
func glEntries(t *testing.T, src ftables.Source) map[types.Code]*gtables.GLEntry {
	t.Helper()
	glEntry := gtables.NewGLEntry(src)
	glEntry.SetRange("document_no", types.Code("FAGL-1"))
	byAccount := map[types.Code]*gtables.GLEntry{}
	total := types.Decimal{}
	for found := glEntry.FindSet(); found; found = glEntry.Next() {
		e := gtables.NewGLEntry(src)
		e.Copy(glEntry)
		if prev, ok := byAccount[e.Gl_account_no]; ok {
			e.Amount = e.Amount.Add(prev.Amount)
		}
		byAccount[e.Gl_account_no] = e
		total = total.Add(glEntry.Amount)
	}
	if !total.IsZero() {
		t.Errorf("the G/L entries do not balance: %s", total)
	}
	return byAccount
}

// An acquisition in the general journal: the FA ledger entry and the G/L entries on the posting
// group's acquisition cost account (pointing at the FA entry) and the balancing account; the FA
// register in the G/L register
func TestGenJnlFAAcquisition(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	newAsset(t, src)
	line := faLine(t, src, 10000, gtables.GenJournalLine_Fa_posting_type.AcquisitionCost, "50000", "1520")
	if line.Depreciation_book_code != "COMPANY" || line.Posting_group != "EQUIPMENT" || line.Description != "Packing machine" {
		t.Fatalf("line from the asset: book %q group %q %q", line.Depreciation_book_code, line.Posting_group, line.Description)
	}
	postLine := genjnl.NewGenJnlPostLine(src)
	must(t, postLine.Run(line))

	entries := glEntries(t, src)
	acq := entries["1230"]
	if acq == nil || !acq.Amount.Equal(dec("50000")) || acq.Fa_entry_type != gtables.GLEntry_Fa_entry_type.FixedAsset || acq.Fa_entry_no == 0 {
		t.Fatalf("acquisition cost account: %+v", acq)
	}
	if bank := entries["1520"]; bank == nil || !bank.Amount.Equal(dec("-50000")) {
		t.Errorf("balancing account: %+v", bank)
	}
	faLedgEntry := gtables.NewFALedgerEntry(src)
	must(t, faLedgEntry.GetOrError(acq.Fa_entry_no))
	if faLedgEntry.G_l_entry_no != acq.Entry_no || faLedgEntry.Fa_no != "FA0090" || !faLedgEntry.Amount.Equal(dec("50000")) {
		t.Errorf("FA entry: G/L entry %d, FA %s, amount %s", faLedgEntry.G_l_entry_no, faLedgEntry.Fa_no, faLedgEntry.Amount)
	}
	faReg := gtables.NewFARegister(src)
	if !faReg.FindLast() || faReg.Journal_type != gtables.FARegister_Journal_type.GL || faReg.G_l_register_no != postLine.GetGLReg().No {
		t.Errorf("FA register: type %v, G/L register %d (want %d)", faReg.Journal_type, faReg.G_l_register_no, postLine.GetGLReg().No)
	}
}

// Depreciation with Insert FA Bal. Account: the posting group's depreciation expense account
// split by an FA allocation (60 % to a dimension 2), the rest on the group's account; posted, the
// accumulated depreciation account takes the amount
func TestGenJnlFADepreciationWithAllocation(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	newAsset(t, src)
	acq := faLine(t, src, 10000, gtables.GenJournalLine_Fa_posting_type.AcquisitionCost, "50000", "1520")
	must(t, genjnl.NewGenJnlPostLine(src).Run(acq))

	alloc := gtables.NewFAAllocation(src)
	alloc.Code = "EQUIPMENT"
	alloc.Allocation_type = gtables.FAAllocation_Allocation_type.Depreciation
	alloc.Line_no = 10000
	must(t, alloc.Validate("account_no", types.Code("5620")))
	alloc.Allocation_percent = dec("60")
	alloc.Dimension_2_code = "PROJ-A"
	must(t, alloc.InsertOrError(true))

	// the journal (an Assets template) with the depreciation line
	tpl := gtables.NewGenJournalTemplate(src)
	tpl.Name = "ASSETS"
	must(t, tpl.Validate("type", gtables.GenJournalTemplate_Type.Assets))
	must(t, tpl.InsertOrError(true))
	if tpl.Page_id != 5628 {
		t.Errorf("Assets template page %d", tpl.Page_id)
	}
	batch := gtables.NewGenJournalBatch(src)
	batch.Journal_template_name = "ASSETS"
	batch.Name = "DEFAULT"
	must(t, batch.InsertOrError(true))
	depr := faLine(t, src, 10000, gtables.GenJournalLine_Fa_posting_type.Depreciation, "-1000", "")
	depr.Journal_template_name = "ASSETS"
	depr.Journal_batch_name = "DEFAULT"
	must(t, depr.InsertOrError(true))
	_, err := fajnl.InsertFABalAccount(depr)
	must(t, err)
	lines := gtables.NewGenJournalLine(src)
	lines.SetRange("journal_template_name", types.Code("ASSETS"))
	lines.SetRange("journal_batch_name", types.Code("DEFAULT"))
	var balLines []string
	for found := lines.FindSet(); found; found = lines.Next() {
		if lines.Account_type == gtables.GenJournalLine_Account_type.GLAccount {
			balLines = append(balLines, lines.Account_no.String()+" "+lines.Amount.String()+" "+lines.Dimension_2_code.String())
		}
	}
	if strings.Join(balLines, "|") != "5620 600 PROJ-A|5620 400 " {
		t.Errorf("balancing lines: %v", balLines)
	}

	result, err := genjnl.NewGenJnlPostBatch(src).Run(depr)
	must(t, err)
	if result.RegisterNo == 0 {
		t.Fatal("nothing posted")
	}
	b := gtables.NewFADepreciationBook(src)
	must(t, b.GetOrError(map[string]interface{}{"fa_no": "FA0090", "depreciation_book_code": "COMPANY"}))
	b.CalcFields("acquisition_cost", "depreciation", "book_value")
	wantDec(t, "depreciation", b.Depreciation, "-1000")
	wantDec(t, "book value", b.Book_value, "49000")
	accum := gtables.NewGLEntry(src)
	accum.SetRange("gl_account_no", types.Code("1240"))
	accum.SetRange("fa_entry_type", gtables.GLEntry_Fa_entry_type.FixedAsset)
	must(t, accum.CalcSums("amount"))
	if !accum.Amount.Round(2).Equal(dec("-1000")) {
		t.Errorf("accumulated depreciation account: %s", accum.Amount)
	}
}

// A maintenance in the general journal: the maintenance entry and the G/L entry on the
// maintenance expense account; the checks: a posting type the book does not post to the G/L, a
// G/L line with a depreciation book
func TestGenJnlFAMaintenanceAndChecks(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	newAsset(t, src)
	must(t, genjnl.NewGenJnlPostLine(src).Run(faLine(t, src, 10000, gtables.GenJournalLine_Fa_posting_type.AcquisitionCost, "50000", "1520")))
	maint := faLine(t, src, 20000, gtables.GenJournalLine_Fa_posting_type.Maintenance, "1200", "1520")
	must(t, maint.Validate("maintenance_code", types.Code("SERVICE")))
	must(t, genjnl.NewGenJnlPostLine(src).Run(maint))
	maintEntry := gtables.NewMaintenanceLedgerEntry(src)
	maintEntry.SetRange("fa_no", types.Code("FA0090"))
	if !maintEntry.FindFirst() || maintEntry.G_l_entry_no == 0 || !maintEntry.Amount.Equal(dec("1200")) {
		t.Fatalf("maintenance entry: G/L entry %d amount %s", maintEntry.G_l_entry_no, maintEntry.Amount)
	}
	glEntry := gtables.NewGLEntry(src)
	must(t, glEntry.GetOrError(maintEntry.G_l_entry_no))
	if glEntry.Gl_account_no != "5320" || glEntry.Fa_entry_type != gtables.GLEntry_Fa_entry_type.Maintenance {
		t.Errorf("maintenance G/L entry: %s %v", glEntry.Gl_account_no, glEntry.Fa_entry_type)
	}

	// the TAX book does not post to the G/L: its posting types belong in the FA journal
	faDeprBook := gtables.NewFADepreciationBook(src)
	faDeprBook.Fa_no = "FA0090"
	faDeprBook.Depreciation_book_code = "TAX"
	must(t, faDeprBook.InsertOrError(true))
	taxLine := faLine(t, src, 30000, gtables.GenJournalLine_Fa_posting_type.AcquisitionCost, "100", "1520")
	must(t, taxLine.Validate("depreciation_book_code", types.Code("TAX")))
	err := genjnl.NewGenJnlPostLine(src).Run(taxLine)
	if err == nil || !strings.Contains(err.Error(), "must be posted in the FA journal") {
		t.Errorf("TAX book in the general journal: %v", err)
	}
	// a G/L line must not have a depreciation book
	glLine := gtables.NewGenJournalLine(src)
	glLine.Posting_date = date("2026-07-01")
	glLine.Document_no = "FAGL-2"
	must(t, glLine.Validate("account_no", types.Code("5320")))
	must(t, glLine.Validate("amount", dec("10")))
	must(t, glLine.Validate("bal_account_no", types.Code("1520")))
	glLine.Depreciation_book_code = "COMPANY"
	err = genjnl.NewGenJnlPostLine(src).Run(glLine)
	if err == nil || !strings.Contains(err.Error(), "Depreciation Book Code") {
		t.Errorf("G/L line with a depreciation book: %v", err)
	}
}

// With euro as the additional reporting currency and a book posting acquisitions at the asset's
// own rate: the line takes the FA depreciation book's FA Add.-Currency Factor and codeunit 12
// exchanges the line's G/L entries with it (0.1: 50000 → 5000 euro), not with the day's rate
func TestGenJnlFAAddCurrencyFactor(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	glSetup := gtables.GetGeneralLedgerSetup(src)
	glSetup.Additional_reporting_currency = "EUR"
	must(t, glSetup.ModifyOrError(false))
	deprBook := gtables.NewDepreciationBook(src)
	must(t, deprBook.GetOrError(types.Code("COMPANY")))
	deprBook.Add_curr_exch_rate_acq_cost = true
	must(t, deprBook.ModifyOrError(false))
	newAsset(t, src)
	faDeprBook := gtables.NewFADepreciationBook(src)
	must(t, faDeprBook.GetOrError(map[string]interface{}{"fa_no": "FA0090", "depreciation_book_code": "COMPANY"}))
	faDeprBook.Fa_add_currency_factor = dec("0.1")
	must(t, faDeprBook.ModifyOrError(false))

	line := faLine(t, src, 10000, gtables.GenJournalLine_Fa_posting_type.AcquisitionCost, "50000", "1520")
	if !line.Fa_add_currency_factor.Equal(dec("0.1")) {
		t.Fatalf("the line's FA Add.-Currency Factor %s, want 0.1", line.Fa_add_currency_factor)
	}
	must(t, genjnl.NewGenJnlPostLine(src).Run(line))
	entries := glEntries(t, src)
	if acq := entries["1230"]; acq == nil || !acq.Additional_currency_amount.Equal(dec("5000")) {
		t.Errorf("acquisition cost account in euro: %+v", acq)
	}
	if bank := entries["1520"]; bank == nil || !bank.Additional_currency_amount.Equal(dec("-5000")) {
		t.Errorf("balancing account in euro: %+v", bank)
	}

	// a depreciation the book posts at the day's rate: no factor
	depr := faLine(t, src, 20000, gtables.GenJournalLine_Fa_posting_type.Depreciation, "-1000", "1520")
	if !depr.Fa_add_currency_factor.IsZero() {
		t.Errorf("depreciation line's factor %s, want 0", depr.Fa_add_currency_factor)
	}
}
