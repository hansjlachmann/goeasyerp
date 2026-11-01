package fajnl_test

import (
	"math"
	"testing"

	"github.com/hansjlachmann/openerp/backend/business-logic/fadepr"
	"github.com/hansjlachmann/openerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/reports"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// taxAsset: FA0090 in the TAX book (no G/L integration), depreciation from 2026-01-01 with a
// method set by setMethod, acquired for 36,000 on 2026-01-01 through the FA journal
func taxAsset(t *testing.T, src ftables.Source, setMethod func(b *gtables.FADepreciationBook)) {
	t.Helper()
	fa := gtables.NewFixedAsset(src)
	fa.No = "FA0090"
	fa.Description = "Packing machine"
	must(t, fa.InsertOrError(true))
	b := gtables.NewFADepreciationBook(src)
	b.Fa_no = "FA0090"
	b.Depreciation_book_code = "TAX"
	must(t, b.InsertOrError(true))
	must(t, b.Validate("fa_posting_group", types.Code("EQUIPMENT")))
	must(t, b.Validate("depreciation_starting_date", date("2026-01-01")))
	setMethod(b)
	must(t, b.ModifyOrError(true))
	_, batch := journal(t, src)
	line := gtables.NewFAJournalLine(src)
	line.Journal_template_name = batch.Journal_template_name
	line.Journal_batch_name = batch.Name
	line.Line_no = 10000
	line.Depreciation_book_code = "TAX"
	must(t, line.Validate("fa_no", types.Code("FA0090")))
	line.Fa_posting_date = date("2026-01-01")
	line.Document_no = "ACQ"
	must(t, line.Validate("amount", dec("36000")))
	must(t, line.InsertOrError(true))
	_, err := postBatch(t, src, batch)
	must(t, err)
}

func calcTax(t *testing.T, src ftables.Source, until string) fadepr.Result {
	t.Helper()
	result, err := fadepr.Calculate(src, "FA0090", "TAX", date(until), fadepr.EntryAmounts{}, types.Date{}, 0)
	must(t, err)
	return result
}

// The depreciation methods over the first quarter (90 depreciation days of a 360-day year) of an
// asset of 36,000: straight-line over 3 years and by 20 %, declining-balance 1 and 2 at 30 %,
// a user-defined table of months (10 %, 20 %, 30 %)
func TestDepreciationMethods(t *testing.T) {
	db := demoDB(t)
	cases := []struct {
		name   string
		method func(b *gtables.FADepreciationBook)
		want   string
	}{
		{"straight-line 3 years", func(b *gtables.FADepreciationBook) {
			must(t, b.Validate("no_of_depreciation_years", dec("3")))
		}, "-3000"},
		{"straight-line 20 %", func(b *gtables.FADepreciationBook) {
			must(t, b.Validate("straight_line_percent", dec("20")))
		}, "-1800"},
		{"declining-balance 1", func(b *gtables.FADepreciationBook) {
			must(t, b.Validate("depreciation_method", gtables.FADepreciationBook_Depreciation_method.DecliningBalance1))
			must(t, b.Validate("declining_balance_percent", dec("30")))
		}, "-2700"},
		{"declining-balance 2", func(b *gtables.FADepreciationBook) {
			must(t, b.Validate("depreciation_method", gtables.FADepreciationBook_Depreciation_method.DecliningBalance2))
			must(t, b.Validate("declining_balance_percent", dec("30")))
		}, types.NewDecimal(-36000 * (1 - math.Pow(0.7, 0.25))).Round(2).String()},
		{"user-defined", func(b *gtables.FADepreciationBook) {
			header := gtables.NewDepreciationTableHeader(b)
			header.Code = "MONTHS"
			header.Period_length = gtables.DepreciationTableHeader_Period_length.Month
			must(t, header.InsertOrError(true))
			for i, pct := range []string{"10", "20", "30"} {
				line := gtables.NewDepreciationTableLine(b)
				line.Depreciation_table_code = "MONTHS"
				line.Period_no = i + 1
				must(t, line.Validate("period_depreciation_percent", dec(pct)))
				must(t, line.InsertOrError(true))
			}
			must(t, b.Validate("depreciation_method", gtables.FADepreciationBook_Depreciation_method.UserDefined))
			must(t, b.Validate("depreciation_table_code", types.Code("MONTHS")))
			must(t, b.Validate("first_user_defined_depr_date", date("2026-01-01")))
		}, "-21600"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src, rollback := inTransaction(t, db)
			defer rollback()
			taxAsset(t, src, c.method)
			result := calcTax(t, src, "2026-03-31")
			if !result.DeprAmount.Equal(dec(c.want)) || result.NumberOfDays != 90 {
				t.Errorf("depreciation %s over %d days, want %s over 90", result.DeprAmount, result.NumberOfDays, c.want)
			}
		})
	}
}

// Depr. until FA Posting Date: a depreciation line without amount posts the depreciation up to
// its date; the next calculation starts the day after; at the end of the life the book value
// goes to zero
func TestDeprUntilFAPostingDate(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	taxAsset(t, src, func(b *gtables.FADepreciationBook) {
		must(t, b.Validate("no_of_depreciation_years", dec("3")))
	})
	_, batch := journal(t, src)
	line := addLine(t, batch, 20000, gtables.FAJournalLine_Fa_posting_type.Depreciation, "2026-03-31", "0")
	line.Fa_no = "FA0090"
	line.Depr_until_fa_posting_date = true
	must(t, line.ModifyOrError(true))
	_, err := postBatch(t, src, batch)
	must(t, err)
	b := gtables.NewFADepreciationBook(src)
	must(t, b.GetOrError(map[string]interface{}{"fa_no": "FA0090", "depreciation_book_code": "TAX"}))
	b.CalcFields("depreciation", "book_value")
	wantDec(t, "depreciation until 31 March", b.Depreciation, "-3000")
	if !b.Last_depreciation_date.Equal(date("2026-03-31")) {
		t.Errorf("last depreciation date %s", b.Last_depreciation_date)
	}
	next := calcTax(t, src, "2026-06-30")
	wantDec(t, "the next quarter", next.DeprAmount, "-3000")
	end := calcTax(t, src, "2030-12-31")
	wantDec(t, "to the end of the life", end.DeprAmount, "-33000")
}

// Calculate Depreciation (report 5692) on the COMPANY book, which posts depreciation to the G/L:
// lines in the general journal of the FA Journal Setup with their balancing accounts; posted, the
// asset is depreciated; the TAX book's lines go to its FA journal
func TestCalculateDepreciationReport(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	newAsset(t, src)
	b := gtables.NewFADepreciationBook(src)
	must(t, b.GetOrError(map[string]interface{}{"fa_no": "FA0090", "depreciation_book_code": "COMPANY"}))
	must(t, b.Validate("depreciation_starting_date", date("2026-07-01")))
	must(t, b.Validate("no_of_depreciation_years", dec("5")))
	must(t, b.ModifyOrError(true))
	must(t, genjnl.NewGenJnlPostLine(src).Run(faLine(t, src, 10000, gtables.GenJournalLine_Fa_posting_type.AcquisitionCost, "60000", "1520")))

	tpl := gtables.NewGenJournalTemplate(src)
	tpl.Name = "ASSETS"
	must(t, tpl.Validate("type", gtables.GenJournalTemplate_Type.Assets))
	must(t, tpl.InsertOrError(true))
	batch := gtables.NewGenJournalBatch(src)
	batch.Journal_template_name = "ASSETS"
	batch.Name = "DEFAULT"
	must(t, batch.InsertOrError(true))
	setup := gtables.NewFAJournalSetup(src)
	setup.Depreciation_book_code = "COMPANY"
	setup.Gen_jnl_template_name = "ASSETS"
	setup.Gen_jnl_batch_name = "DEFAULT"
	must(t, setup.InsertOrError(true))

	fa := gtables.NewFixedAsset(src)
	fa.SetRange("no", types.Code("FA0090"))
	_, err := reports.Execute(reports.CalculateDepreciationID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Record: fa,
		Options: map[string]string{"depreciation_book": "COMPANY", "fa_posting_date": "2026-09-30", "document_no": "DEPR-Q3",
			"posting_description": "Depreciation Q3", "insert_bal_account": "true"},
	})
	must(t, err)
	lines := gtables.NewGenJournalLine(src)
	lines.SetRange("journal_template_name", types.Code("ASSETS"))
	lines.SetRange("journal_batch_name", types.Code("DEFAULT"))
	var got []string
	for found := lines.FindSet(); found; found = lines.Next() {
		got = append(got, lines.Account_type.String()+" "+lines.Account_no.String()+" "+lines.Amount.String()+" "+lines.Description.String())
	}
	want := []string{"Fixed Asset FA0090 -3000 Depreciation Q3", "G/L Account 5620 3000 Depreciation Q3"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("journal lines %v, want %v", got, want)
	}
	lines.FindFirst()
	_, err = genjnl.NewGenJnlPostBatch(src).Run(lines)
	must(t, err)
	must(t, b.GetOrError(map[string]interface{}{"fa_no": "FA0090", "depreciation_book_code": "COMPANY"}))
	b.CalcFields("depreciation")
	wantDec(t, "depreciation posted", b.Depreciation, "-3000")
}
