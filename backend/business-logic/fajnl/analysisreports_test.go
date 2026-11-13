package fajnl_test

import (
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// printFAErr runs a fixed asset report and returns its error
func printFAErr(src ftables.Source, id int, record ftables.Table, options map[string]string) error {
	_, err := reports.Execute(id, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Output: reports.OutputPDF, Record: record, Options: options})
	return err
}

// wantWrapped: texts printed, also when wrapped over lines (a heading)
func wantWrapped(t *testing.T, texts string, want ...string) {
	t.Helper()
	joined := strings.ReplaceAll(texts, "|", " ")
	for _, w := range want {
		if !strings.Contains(joined, " "+w+" ") {
			t.Errorf("%q not printed in %s", w, texts)
		}
	}
}

// faEntrySum: an asset's FA ledger entries of a posting type in a book (category blank) posted
// (Posting Date) between the dates
func faEntrySum(t *testing.T, src ftables.Source, faNo, book string, postingType gtables.FALedgerEntryFa_posting_type, from, to string) types.Decimal {
	t.Helper()
	e := gtables.NewFALedgerEntry(src)
	e.SetRange("fa_no", types.Code(faNo))
	e.SetRange("depreciation_book_code", types.Code(book))
	e.SetRange("fa_posting_category", gtables.FALedgerEntry_Fa_posting_category.Blank)
	e.SetRange("fa_posting_type", postingType)
	e.SetRange("posting_date", date(from), date(to))
	must(t, e.CalcSums("amount"))
	return e.Amount
}

// Report 5600 Fixed Asset - Analysis: the budgeted asset's acquisition cost at the ending date,
// its depreciation in the period and book value — with Budget Report the depreciation to come
// (60,000 over 5 years: 6,000 in the first half year); the columns' heads name the types and
// periods; Proceeds on Disposal only at the ending date
func TestFixedAssetAnalysisReport(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	_, batch := journal(t, src)
	budgetedAsset(t, src, batch)
	fa := gtables.NewFixedAsset(src)
	fa.SetRange("no", types.Code("FAB01"))
	options := map[string]string{"depreciation_book": "TAX", "starting_date": "2026-01-01", "ending_date": "2026-06-30",
		"date_field_1":   "2",                  // Last Acquisition Cost Date
		"amount_field_1": "2", "period_1": "3", // Acquisition Cost at Ending Date
		"amount_field_2": "3", "period_2": "2", // Depreciation Net Change
		"amount_field_3": "10", "period_3": "3", // Book Value at Ending Date
		"print_per_fixed_asset": "true"}
	texts := printFA(t, src, reports.FixedAssetAnalysisID, fa, options)
	wantTexts(t, texts, "Fixed Asset - Analysis", "FAB01", "60,000.00")
	wantWrapped(t, texts, "Last Acquisition Cost Date", "Acquisition Cost at Ending Date", "Depreciation Net Change", "Book Value at Ending Date")
	if strings.Contains(texts, "|-6,000.00|") {
		t.Errorf("depreciation to come without Budget Report: %s", texts)
	}
	options["budget_report"] = "true"
	texts = printFA(t, src, reports.FixedAssetAnalysisID, fa, options)
	wantTexts(t, texts, "Fixed Asset - Analysis (Budget Report)", "60,000.00", "-6,000.00", "54,000.00")

	options["amount_field_1"] = "8" // Proceeds on Disposal
	options["period_1"] = "2"
	err := printFAErr(src, reports.FixedAssetAnalysisID, fa, options)
	if err == nil || !strings.Contains(err.Error(), "must be specified only together with the option at Ending Date") {
		t.Errorf("Proceeds on Disposal Net Change: %v", err)
	}
}

// Report 5610 Fixed Asset - G/L Analysis: what the demo's delivery van posted to the G/L in the
// COMPANY book — its acquisition cost and depreciation; Bal. Disposal only for write-down,
// appreciation and the custom types
func TestFixedAssetGLAnalysisReport(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	fa := gtables.NewFixedAsset(src)
	fa.SetRange("no", types.Code("FA0010"))
	options := map[string]string{"depreciation_book": "COMPANY", "starting_date": "2020-01-01", "ending_date": "2026-12-31",
		"date_field_1":   "2",                  // Last Acquisition Cost Date
		"amount_field_1": "2", "period_1": "1", // Acquisition Cost
		"amount_field_2": "3", "period_2": "1", // Depreciation
		"print_per_fixed_asset": "true"}
	acquisition := faEntrySum(t, src, "FA0010", "COMPANY", gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost, "2020-01-01", "2026-12-31")
	depreciation := faEntrySum(t, src, "FA0010", "COMPANY", gtables.FALedgerEntry_Fa_posting_type.Depreciation, "2020-01-01", "2026-12-31")
	if acquisition.IsZero() || depreciation.IsZero() {
		t.Fatalf("the demo's FA0010 has no entries: %s %s", acquisition, depreciation)
	}
	texts := printFA(t, src, reports.FixedAssetGLAnalysisID, fa, options)
	wantTexts(t, texts, "Fixed Asset - G/L Analysis", "FA0010", "Acquisition Cost", "Depreciation", formatted(acquisition), formatted(depreciation))
	wantWrapped(t, texts, "Last Acquisition Cost Date", "Posting Date Filter: 01/01/2020..12/31/2026")

	options["period_1"] = "3" // Bal. Disposal with Acquisition Cost
	err := printFAErr(src, reports.FixedAssetGLAnalysisID, fa, options)
	if err == nil || !strings.Contains(err.Error(), "must be specified only together with the types") {
		t.Errorf("Bal. Disposal with Acquisition Cost: %v", err)
	}
}

// Report 5601 Fixed Asset - List: the asset's master data and its depreciation book
func TestFixedAssetListReport(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	fa := gtables.NewFixedAsset(src)
	fa.SetRange("no", types.Code("FA0010"))
	texts := printFA(t, src, reports.FixedAssetListID, fa, map[string]string{"depreciation_book": "COMPANY"})
	wantTexts(t, texts, "Fixed Asset - List", "Depreciation Book: COMPANY", "FA0010", "Delivery van VW Crafter",
		"WV1ZZZSYZL9012345", "VEHICLES", "Straight-Line", "No. of Depreciation Years", "5.00")
}

// The insurance printouts on the demo's insurance INS010: its analysis (the total value insured,
// over/under insured), list, coverage details, the value insured per asset and the uninsured
// assets with their book values
func TestInsuranceReports(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	insurance := gtables.NewInsurance(src)
	must(t, insurance.GetOrError(types.Code("INS010")))
	insurance.CalcFields("total_value_insured")
	if insurance.Total_value_insured.IsZero() {
		t.Fatal("the demo's INS010 insures nothing")
	}
	ins := gtables.NewInsurance(src)
	ins.SetRange("no", types.Code("INS010"))

	texts := printFA(t, src, reports.InsuranceAnalysisID, ins, map[string]string{"print_per_insurance_no": "true"})
	wantTexts(t, texts, "Insurance - Analysis", "INS010", formatted(insurance.Total_value_insured),
		formatted(insurance.Policy_coverage.Sub(insurance.Total_value_insured)))

	texts = printFA(t, src, reports.InsuranceListID, ins, nil)
	wantTexts(t, texts, "Insurance - List", "INS010", insurance.Policy_no.String())

	texts = printFA(t, src, reports.InsuranceCoverageDetailsID, ins, nil)
	wantTexts(t, texts, "Insurance - Coverage Details", "INS010", "FA0010")

	fa := gtables.NewFixedAsset(src)
	fa.SetRange("no", types.Code("FA0010"))
	coverage := gtables.NewInsCoverageLedgerEntry(src)
	coverage.SetRange("fa_no", types.Code("FA0010"))
	coverage.SetRange("insurance_no", types.Code("INS010"))
	coverage.SetRange("disposed_fa", false)
	must(t, coverage.CalcSums("amount"))
	texts = printFA(t, src, reports.InsuranceTotValueInsuredID, fa, nil)
	wantTexts(t, texts, "Insurance - Tot. Value Insured", "FA0010", "INS010", formatted(coverage.Amount))

	// FA0040 is not insured: listed with its book value in the Insurance Depr. Book (COMPANY)
	uninsured := gtables.NewFixedAsset(src)
	texts = printFA(t, src, reports.InsuranceUninsuredFAsID, uninsured, nil)
	wantTexts(t, texts, "Insurance - Uninsured Fixed Assets", "FA0040", "Total")
	if strings.Contains(texts, "|FA0010|") {
		t.Errorf("the insured FA0010 printed as uninsured: %s", texts)
	}
}

// The maintenance printouts: a TAX book maintenance posted for FA0040 is in the maintenance
// analysis (by maintenance code and in all), the register of its posting, the details; the next
// service list takes the assets whose Next Service Date is in the dates
func TestMaintenanceReports(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	_, batch := journal(t, src)
	addLine(t, batch, 10000, gtables.FAJournalLine_Fa_posting_type.AcquisitionCost, "2026-01-02", "1000")
	line := addLine(t, batch, 20000, gtables.FAJournalLine_Fa_posting_type.Maintenance, "2026-03-01", "250")
	must(t, line.Validate("maintenance_code", types.Code("SERVICE")))
	must(t, line.ModifyOrError(true))
	_, err := postBatch(t, src, batch)
	must(t, err)

	fa := gtables.NewFixedAsset(src)
	fa.SetRange("no", types.Code("FA0040"))
	options := map[string]string{"depreciation_book": "TAX", "starting_date": "2026-02-01", "ending_date": "2026-12-31",
		"amount_field_1": "SERVICE", "period_1": "2", // SERVICE Net Change
		"period_2":              "1", // all codes before the starting date
		"period_3":              "3", // all codes at the ending date
		"print_per_fixed_asset": "true"}
	texts := printFA(t, src, reports.MaintenanceAnalysisID, fa, options)
	wantTexts(t, texts, "Maintenance - Analysis", "FA0040", "250.00")
	wantWrapped(t, texts, "SERVICE Net Change", "before Starting Date", "at Ending Date", "FA Posting Date Filter: 02/01/2026..12/31/2026")
	n := 0
	for _, text := range strings.Split(texts, "|") {
		if text == "250.00" {
			n++
		}
	}
	if n != 4 { // net change and at the ending date, asset and total
		t.Errorf("250.00 printed %d times, want 4: %s", n, texts)
	}

	faReg := gtables.NewFARegister(src)
	if !faReg.FindLast() || faReg.From_maintenance_entry_no == 0 {
		t.Fatal("no FA register with maintenance entries")
	}
	reg := gtables.NewFARegister(src)
	reg.SetRange("no", faReg.No)
	texts = printFA(t, src, reports.MaintenanceRegisterID, reg, nil)
	wantTexts(t, texts, "Maintenance Register", "Register No.", "FA0040", "SERVICE", "250.00")

	texts = printFA(t, src, reports.MaintenanceDetailsID, fa, map[string]string{"depreciation_book": "TAX"})
	wantTexts(t, texts, "Maintenance - Details", "FA0040", "SERVICE", "250.00", "FA-TAX-1")

	asset := gtables.NewFixedAsset(src)
	must(t, asset.GetOrError(types.Code("FA0040")))
	asset.Next_service_date = date("2026-11-15")
	must(t, asset.ModifyOrError(false))
	all := gtables.NewFixedAsset(src)
	texts = printFA(t, src, reports.MaintenanceNextServiceID, all, map[string]string{"starting_date": "2026-11-01", "ending_date": "2026-11-30"})
	wantTexts(t, texts, "Maintenance - Next Service", "FA0040")
	texts = printFA(t, src, reports.MaintenanceNextServiceID, all, map[string]string{"starting_date": "2026-12-01", "ending_date": "2026-12-31"})
	if strings.Contains(texts, "|FA0040|") {
		t.Errorf("FA0040's next service in November printed for December: %s", texts)
	}
}
