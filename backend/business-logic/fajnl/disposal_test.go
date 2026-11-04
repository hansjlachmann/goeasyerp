package fajnl_test

import (
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/genjnl"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// acquiredAndDepreciated: FA0090 in the COMPANY book acquired for 50,000 and depreciated by
// 10,000 (book value 40,000)
func acquiredAndDepreciated(t *testing.T, src ftables.Source) {
	t.Helper()
	newAsset(t, src)
	must(t, genjnl.NewGenJnlPostLine(src).Run(faLine(t, src, 10000, gtables.GenJournalLine_Fa_posting_type.AcquisitionCost, "50000", "1520")))
	depr := faLine(t, src, 20000, gtables.GenJournalLine_Fa_posting_type.Depreciation, "-10000", "5620")
	must(t, genjnl.NewGenJnlPostLine(src).Run(depr))
}

// faBook: FA0090's COMPANY book with its amounts
func faBook(t *testing.T, src ftables.Source) *gtables.FADepreciationBook {
	t.Helper()
	b := gtables.NewFADepreciationBook(src)
	must(t, b.GetOrError(map[string]interface{}{"fa_no": "FA0090", "depreciation_book_code": "COMPANY"}))
	b.CalcFields("book_value", "proceeds_on_disposal", "gain_loss", "book_value_on_disposal", "acquisition_cost", "depreciation")
	return b
}

// A disposal (method Net) for 45,000: the acquisition cost and depreciation taken out (category
// Disposal), the gain of 5,000 on the gains account, the G/L balanced; a second disposal adds to
// the gain; the disposal date stops further postings
func TestDisposalNet(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	acquiredAndDepreciated(t, src)
	sale := faLine(t, src, 30000, gtables.GenJournalLine_Fa_posting_type.Disposal, "-45000", "1520")
	sale.Posting_date = date("2026-08-01")
	sale.Document_no = "FAGL-1"
	must(t, genjnl.NewGenJnlPostLine(src).Run(sale))
	b := faBook(t, src)
	wantDec(t, "proceeds", b.Proceeds_on_disposal, "-45000")
	wantDec(t, "gain/loss", b.Gain_loss, "-5000")
	// the disposal's entries (category Disposal) are not part of the book value: it stays as it
	// was at the disposal, as in the classic FlowField
	wantDec(t, "book value", b.Book_value, "40000")
	if !b.Disposal_date.Equal(date("2026-08-01")) {
		t.Errorf("disposal date %s", b.Disposal_date)
	}
	entries := glEntries(t, src)
	if g := entries["6180"]; g == nil || !g.Amount.Equal(dec("-5000")) {
		t.Errorf("gains account: %+v", g)
	}
	if a := entries["1230"]; a == nil || !a.Amount.IsZero() {
		t.Errorf("acquisition cost account after the disposal: %+v", a)
	}
	if d := entries["1240"]; d == nil || !d.Amount.IsZero() {
		t.Errorf("accumulated depreciation account after the disposal: %+v", d)
	}

	// a second disposal: more proceeds, more gain
	second := faLine(t, src, 40000, gtables.GenJournalLine_Fa_posting_type.Disposal, "-1000", "1520")
	second.Posting_date = date("2026-08-01")
	must(t, genjnl.NewGenJnlPostLine(src).Run(second))
	wantDec(t, "gain after the second disposal", faBook(t, src).Gain_loss, "-6000")

	// a disposed asset takes no more depreciation
	depr := faLine(t, src, 50000, gtables.GenJournalLine_Fa_posting_type.Depreciation, "-100", "5620")
	depr.Posting_date = date("2026-09-01")
	err := genjnl.NewGenJnlPostLine(src).Run(depr)
	if err == nil || !strings.Contains(err.Error(), "is disposed") {
		t.Errorf("depreciation after the disposal: %v", err)
	}
}

// A disposal (method Gross) at a loss: the proceeds on the sales account, the book value on
// disposal on its account, the loss of 15,000 in the FA book (not on the G/L)
func TestDisposalGross(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	deprBook := gtables.NewDepreciationBook(src)
	must(t, deprBook.GetOrError(types.Code("COMPANY")))
	deprBook.Disposal_calculation_method = gtables.DepreciationBook_Disposal_calculation_method.Gross
	must(t, deprBook.ModifyOrError(true))
	acquiredAndDepreciated(t, src)
	sale := faLine(t, src, 30000, gtables.GenJournalLine_Fa_posting_type.Disposal, "-25000", "1520")
	must(t, genjnl.NewGenJnlPostLine(src).Run(sale))
	b := faBook(t, src)
	wantDec(t, "gain/loss", b.Gain_loss, "15000")
	wantDec(t, "book value on disposal", b.Book_value_on_disposal, "40000")
	entries := glEntries(t, src)
	if s := entries["6180"]; s == nil || !s.Amount.Equal(dec("-25000")) {
		t.Errorf("sales account on disposal: %+v", s)
	}
	if bv := entries["6190"]; bv == nil || !bv.Amount.Equal(dec("40000")) {
		t.Errorf("book value account on disposal: %+v", bv)
	}
}
