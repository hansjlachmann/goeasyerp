package tables

import (
	"database/sql"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func newFADB(t *testing.T) (*sql.DB, ftables.Source) {
	t.Helper()
	db := newTestDB(t)
	createTables(t, db, &gtables.FixedAsset{}, &gtables.FALedgerEntry{}, &gtables.FASetup{}, &gtables.DepreciationBook{}, &gtables.FADepreciationBook{},
		&gtables.MaintenanceLedgerEntry{}, &gtables.MaintenanceRegistration{}, &gtables.MainAssetComponent{}, &gtables.Insurance{},
		&gtables.InsCoverageLedgerEntry{}, &gtables.CommentLine{}, &gtables.AccountingPeriod{},
		&gtables.NoSeries{}, &gtables.NoSeriesLine{}, &gtables.NoSeriesRelationship{})
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	for _, s := range []struct{ code, start string }{{"FA", "FA0010"}, {"INS", "INS010"}} {
		series := gtables.NewNoSeries(src)
		series.Code, series.Default_nos = types.Code(s.code), true
		line := gtables.NewNoSeriesLine(src)
		line.InitRecord()
		line.Series_code, line.Line_no, line.Starting_no = types.Code(s.code), 10000, types.Code(s.start)
		if !series.Insert(false) || !line.Insert(false) {
			t.Fatal("insert series")
		}
	}
	faSetup := gtables.NewFASetup(src)
	faSetup.InitRecord()
	faSetup.Fixed_asset_nos, faSetup.Insurance_nos = "FA", "INS"
	book := gtables.NewDepreciationBook(src)
	book.InitRecord()
	book.Code, book.Description = "COMPANY", "Company book"
	if !faSetup.Insert(false) || !book.Insert(false) {
		t.Fatal("insert setup")
	}
	return db, src
}

func newFA(t *testing.T, src ftables.Source, description string) *gtables.FixedAsset {
	t.Helper()
	fa := gtables.NewFixedAsset(src)
	fa.InitRecord()
	_ = fa.Validate("description", types.Text(description))
	if err := fa.InsertOrError(true); err != nil {
		t.Fatal(err)
	}
	return fa
}

// Fixed Asset: numbers from the FA Setup, Search Description; a depreciation book takes the
// description; the depreciation period from the years (360-day years) and back from the dates
func TestFixedAssetAndDepreciationBook(t *testing.T) {
	_, src := newFADB(t)
	fa := newFA(t, src, "Delivery van")
	if fa.No != "FA0010" || fa.No_series != "FA" || fa.Search_description != "DELIVERY VAN" {
		t.Fatalf("fixed asset %q %q %q", fa.No, fa.No_series, fa.Search_description)
	}
	faDeprBook := gtables.NewFADepreciationBook(src)
	faDeprBook.InitRecord()
	faDeprBook.Fa_no, faDeprBook.Depreciation_book_code = fa.No, "COMPANY"
	if err := faDeprBook.InsertOrError(true); err != nil || faDeprBook.Description != "Delivery van" {
		t.Fatalf("depreciation book %q, %v", faDeprBook.Description, err)
	}

	// Years need a starting date; 5 years from 1 January 2025 end on 31 December 2029
	if err := faDeprBook.Validate("no_of_depreciation_years", types.NewDecimal(5)); err == nil {
		t.Error("years without a starting date")
	}
	_ = faDeprBook.Validate("depreciation_starting_date", types.NewDate(2025, 1, 1))
	if err := faDeprBook.Validate("no_of_depreciation_years", types.NewDecimal(5)); err != nil {
		t.Fatal(err)
	}
	if !faDeprBook.No_of_depreciation_months.Equal(types.NewDecimal(60)) || !faDeprBook.Depreciation_ending_date.Equal(types.NewDate(2029, 12, 31)) {
		t.Errorf("5 years: %s months, ends %s", faDeprBook.No_of_depreciation_months, faDeprBook.Depreciation_ending_date)
	}
	// An ending date gives the period; one before the start is refused
	if err := faDeprBook.Validate("depreciation_ending_date", types.NewDate(2027, 6, 30)); err != nil || !faDeprBook.No_of_depreciation_months.Equal(types.NewDecimal(30)) {
		t.Errorf("ending 30 June 2027: %s months, %v", faDeprBook.No_of_depreciation_months, err)
	}
	if err := faDeprBook.Validate("depreciation_ending_date", types.NewDate(2024, 1, 1)); err == nil || errCode(t, err) != apperrors.ErrDateLater {
		t.Errorf("ending before the start: %v", err)
	}
	// Declining-balance fields only with a declining-balance method
	if err := faDeprBook.Validate("declining_balance_percent", types.NewDecimal(20)); err == nil {
		t.Error("declining-balance % with straight-line")
	}
	_ = faDeprBook.Validate("depreciation_method", gtables.FADepreciationBook_Depreciation_method.DecliningBalance1)
	if !faDeprBook.No_of_depreciation_years.IsZero() || !faDeprBook.Depreciation_ending_date.IsZero() {
		t.Error("declining-balance keeps the straight-line fields")
	}
	if err := faDeprBook.Validate("declining_balance_percent", types.NewDecimal(100)); err == nil {
		t.Error("declining-balance 100%")
	}
	if err := faDeprBook.Validate("declining_balance_percent", types.NewDecimal(20)); err != nil {
		t.Error(err)
	}
	if err := faDeprBook.ModifyOrError(true); err != nil {
		t.Fatal(err)
	}

	// The description follows to the depreciation books
	if err := fa.Validate("description", types.Text("Delivery van, white")); err != nil {
		t.Fatal(err)
	}
	if !faDeprBook.Get(map[string]interface{}{"fa_no": fa.No.String(), "depreciation_book_code": "COMPANY"}) || faDeprBook.Description != "Delivery van, white" {
		t.Errorf("book description %q", faDeprBook.Description)
	}
	if faDeprBook.Rename(fa.No, types.Code("TAX")) {
		t.Error("a depreciation book of an asset was renamed")
	}
	if !faDeprBook.Get(map[string]interface{}{"fa_no": fa.No.String(), "depreciation_book_code": "COMPANY"}) {
		t.Fatal("the depreciation book")
	}

	// The book is in use; acquisition cost and book value are the entries' totals
	book := gtables.NewDepreciationBook(src)
	if !book.Get(types.Code("COMPANY")) || book.Delete(true) {
		t.Error("a depreciation book in use was deleted")
	}
	for i, e := range []struct {
		postingType gtables.FALedgerEntryFa_posting_type
		amount      int64
	}{{gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost, 300000}, {gtables.FALedgerEntry_Fa_posting_type.Depreciation, -60000}} {
		entry := gtables.NewFALedgerEntry(src)
		entry.Entry_no, entry.Fa_no, entry.Depreciation_book_code = i+1, fa.No, "COMPANY"
		entry.Fa_posting_type, entry.Amount, entry.Part_of_book_value = e.postingType, types.NewDecimalFromInt(e.amount), true
		entry.Fa_posting_date = types.NewDate(2025, 3, 1)
		if err := entry.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}
	faDeprBook.CalcFields("acquisition_cost", "depreciation", "book_value")
	if !faDeprBook.Acquisition_cost.Equal(types.NewDecimal(300000)) || !faDeprBook.Book_value.Equal(types.NewDecimal(240000)) {
		t.Errorf("acquisition cost %s, book value %s", faDeprBook.Acquisition_cost, faDeprBook.Book_value)
	}
	// Budgeted Asset cannot change with entries; an asset that is not disposed cannot be deleted
	if err := fa.Validate("budgeted_asset", true); err == nil || errCode(t, err) != apperrors.ErrFABudgetEntries {
		t.Errorf("budgeted asset with entries: %v", err)
	}
	if fa.Delete(true) || errCode(t, fa.TriggerError()) != apperrors.ErrFANotDisposed {
		t.Errorf("delete an asset that is not disposed: %v", fa.TriggerError())
	}
	// Disposed, but the entries are in an open year
	faDeprBook.Disposal_date = types.NewDate(2025, 6, 1)
	if err := faDeprBook.ModifyOrError(false); err != nil {
		t.Fatal(err)
	}
	if fa.Delete(true) || errCode(t, fa.TriggerError()) != apperrors.ErrFADeleteEntries {
		t.Errorf("delete with entries in an open year: %v", fa.TriggerError())
	}
	// 2025 closed: the asset goes, its books with it, the entries lose it
	addPeriods(t, db(t, src), 2025, 2, types.NewDate(2025, 12, 1))
	if err := fa.DeleteOrError(true); err != nil {
		t.Fatalf("delete a disposed asset: %v", err)
	}
	entry := gtables.NewFALedgerEntry(src)
	if gtables.NewFADepreciationBook(src).Count() != 0 || !entry.Get(1) || entry.Fa_no != "" {
		t.Errorf("after the delete: books %d, entry fa %q", gtables.NewFADepreciationBook(src).Count(), entry.Fa_no)
	}
}

// db gives the database of a source (the tests' helpers take *sql.DB)
func db(t *testing.T, src ftables.Source) *sql.DB {
	t.Helper()
	d, ok := src.GetDB().(*sql.DB)
	if !ok {
		t.Fatal("not a *sql.DB")
	}
	return d
}

// Main asset components: a component line makes the asset a component and the main asset a
// main asset; a main asset cannot be deleted, nor become a component itself; deleting the
// component line undoes it
func TestMainAssetComponents(t *testing.T) {
	_, src := newFADB(t)
	mainAsset := newFA(t, src, "Production line")
	component := newFA(t, src, "Conveyor belt")
	other := newFA(t, src, "Robot arm")

	comp := gtables.NewMainAssetComponent(src)
	comp.Main_asset_no = mainAsset.No
	if err := comp.Validate("fa_no", component.No); err != nil {
		t.Fatal(err)
	}
	if err := comp.InsertOrError(true); err != nil || comp.Description != "Conveyor belt" {
		t.Fatalf("component line %q, %v", comp.Description, err)
	}
	if !mainAsset.Get(mainAsset.No) || mainAsset.Main_asset_component != gtables.FixedAsset_Main_asset_component.MainAsset ||
		!component.Get(component.No) || component.Main_asset_component != gtables.FixedAsset_Main_asset_component.Component || component.Component_of_main_asset != mainAsset.No {
		t.Fatalf("main %v, component %v of %q", mainAsset.Main_asset_component, component.Main_asset_component, component.Component_of_main_asset)
	}
	// The main asset cannot be a component of another; a component not of a second main asset
	again := gtables.NewMainAssetComponent(src)
	again.Main_asset_no = other.No
	if err := again.Validate("fa_no", mainAsset.No); err == nil || errCode(t, err) != apperrors.ErrFAIsA {
		t.Errorf("main asset as a component: %v", err)
	}
	if err := again.Validate("fa_no", component.No); err == nil {
		t.Error("a component of two main assets")
	} else if msg := err.(*apperrors.AppError).Message("en-US"); msg != "Fixed Asset "+component.No.String()+" is a Component." {
		t.Errorf("message %q", msg)
	}
	if mainAsset.Delete(true) || errCode(t, mainAsset.TriggerError()) != apperrors.ErrFAMainAssetDelete {
		t.Errorf("delete a main asset: %v", mainAsset.TriggerError())
	}
	// Deleting the line: neither is main asset nor component any more
	if err := comp.DeleteOrError(true); err != nil {
		t.Fatal(err)
	}
	if !mainAsset.Get(mainAsset.No) || mainAsset.Main_asset_component != gtables.FixedAsset_Main_asset_component.Blank ||
		!component.Get(component.No) || component.Main_asset_component != gtables.FixedAsset_Main_asset_component.Blank {
		t.Errorf("after the delete: main %v, component %v", mainAsset.Main_asset_component, component.Main_asset_component)
	}
}

// Insurance: numbers from the FA Setup; Total Value Insured from the coverage entries (not of
// disposed assets); deleting it clears its coverage entries and comments
func TestInsurance(t *testing.T) {
	_, src := newFADB(t)
	insurance := gtables.NewInsurance(src)
	insurance.InitRecord()
	_ = insurance.Validate("description", types.Text("Vehicle fleet"))
	if err := insurance.InsertOrError(true); err != nil || insurance.No != "INS010" || insurance.Search_description != "VEHICLE FLEET" {
		t.Fatalf("insurance %q %q, %v", insurance.No, insurance.Search_description, err)
	}
	for i, e := range []struct {
		amount   int64
		disposed bool
	}{{400000, false}, {150000, true}} {
		entry := gtables.NewInsCoverageLedgerEntry(src)
		entry.Entry_no, entry.Insurance_no, entry.Fa_no, entry.Amount, entry.Disposed_fa = i+1, insurance.No, "FA0010", types.NewDecimalFromInt(e.amount), e.disposed
		if err := entry.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}
	insurance.CalcFields("total_value_insured")
	if !insurance.Total_value_insured.Equal(types.NewDecimal(400000)) {
		t.Errorf("total value insured %s", insurance.Total_value_insured)
	}
	if err := insurance.DeleteOrError(true); err != nil {
		t.Fatal(err)
	}
	entry := gtables.NewInsCoverageLedgerEntry(src)
	if !entry.Get(1) || entry.Insurance_no != "" || entry.Fa_no != "" {
		t.Errorf("coverage entry after the delete: %q %q", entry.Insurance_no, entry.Fa_no)
	}
}

// The depreciation calendar (360-day years)
func TestDepreciationDates(t *testing.T) {
	for _, c := range []struct {
		from, to types.Date
		days     int
	}{
		{types.NewDate(2025, 1, 1), types.NewDate(2025, 12, 31), 360},
		{types.NewDate(2025, 1, 31), types.NewDate(2025, 2, 28), 31}, // the 31st counts as the 30th, February's end as the 30th
		{types.NewDate(2025, 3, 15), types.NewDate(2025, 3, 14), 0},
	} {
		if got := DeprDays(c.from, c.to); got != c.days {
			t.Errorf("DeprDays(%s, %s) = %d, want %d", c.from, c.to, got, c.days)
		}
	}
	if got := CalculateDate(types.NewDate(2025, 1, 31), 30); !got.Equal(types.NewDate(2025, 2, 28)) {
		t.Errorf("30 days after 31 January: %s", got)
	}
	if got := Yesterday(types.NewDate(2025, 3, 31)); !got.Equal(types.NewDate(2025, 3, 29)) {
		t.Errorf("yesterday of 31 March: %s", got)
	}
}
