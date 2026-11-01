package demodata

import (
	"fmt"
	"math"
	"time"

	"github.com/hansjlachmann/openerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Demo fixed assets (data/fixedassets.yaml): each asset with its COMPANY depreciation book
// (straight-line, through the table's own validation), the acquisition and a monthly
// depreciation up to last month, a yearly service for the vehicles, one insurance with the
// coverage of the insured assets, and one main asset with a component. With Options.GenJnlPostLine
// the ledgers are posted (fixedassetposting.go: acquisitions, payments, depreciation, services
// and a disposal); without it their entries are inserted as they would be.

type fixedAssetData struct {
	No                string  `yaml:"no"`
	Description       string  `yaml:"description"`
	Subclass          string  `yaml:"subclass"`
	Location          string  `yaml:"location"`
	PostingGroup      string  `yaml:"posting_group"`
	Vendor            string  `yaml:"vendor"`
	MaintenanceVendor string  `yaml:"maintenance_vendor"`
	MonthsAgo         int     `yaml:"months_ago"`
	Cost              float64 `yaml:"cost"`
	Years             int     `yaml:"years"`
	SerialNo          string  `yaml:"serial_no"`
	Insured           bool    `yaml:"insured"`
	ComponentOf       string  `yaml:"component_of"`
}

type fixedAssetsFile struct {
	FixedAssets []fixedAssetData `yaml:"fixed_assets"`
	// Disposal: the asset sold (posted ledgers only)
	Disposal struct {
		FANo     string  `yaml:"fa_no"`
		Day      int     `yaml:"day"` // of last month
		Proceeds float64 `yaml:"proceeds"`
	} `yaml:"disposal"`
	Insurance struct {
		No             string  `yaml:"no"`
		Description    string  `yaml:"description"`
		InsuranceType  string  `yaml:"insurance_type"`
		Vendor         string  `yaml:"vendor"`
		PolicyNo       string  `yaml:"policy_no"`
		AnnualPremium  float64 `yaml:"annual_premium"`
		PolicyCoverage float64 `yaml:"policy_coverage"`
	} `yaml:"insurance"`
}

const demoDeprBook = types.Code("COMPANY")

// insertFixedAssets inserts the insurance, the assets with their COMPANY book and the main asset
// components — and, unless the ledgers are posted (posted), the assets' ledger entries
func insertFixedAssets(exec database.Executor, company string, dbType database.DBType, data fixedAssetsFile, today time.Time, posted bool, counts *Counts) error {
	src := ftables.DBSource(exec, company, dbType)
	monthStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
	faEntryNo, maintEntryNo, insEntryNo := 0, 0, 0
	var faEntries []*gtables.FALedgerEntry
	var maintEntries []*gtables.MaintenanceLedgerEntry
	var insEntries []*gtables.InsCoverageLedgerEntry

	insurance := gtables.NewInsurance(src)
	insurance.InitRecord()
	insurance.No = types.NewCode(data.Insurance.No)
	_ = insurance.Validate("description", types.NewText(data.Insurance.Description))
	insurance.Insurance_type = types.NewCode(data.Insurance.InsuranceType)
	insurance.Insurance_vendor_no = types.NewCode(data.Insurance.Vendor)
	insurance.Policy_no = types.NewText(data.Insurance.PolicyNo)
	insurance.Annual_premium = types.NewDecimal(data.Insurance.AnnualPremium)
	insurance.Policy_coverage = types.NewDecimal(data.Insurance.PolicyCoverage)
	insurance.Effective_date = types.NewDateFromTime(monthStart.AddDate(-2, 0, 0))
	err := insurance.InsertOrError(true)
	if err != nil {
		return &InsertError{Table: gtables.InsuranceTableName, Key: data.Insurance.No, Err: err}
	}

	for _, a := range data.FixedAssets {
		fa := gtables.NewFixedAsset(src)
		fa.InitRecord()
		fa.No = types.NewCode(a.No)
		_ = fa.Validate("description", types.NewText(a.Description))
		fa.Fa_class_code = "TANGIBLE"
		fa.Fa_subclass_code = types.NewCode(a.Subclass)
		fa.Fa_location_code = types.NewCode(a.Location)
		fa.Vendor_no = types.NewCode(a.Vendor)
		fa.Maintenance_vendor_no = types.NewCode(a.MaintenanceVendor)
		fa.Serial_no = types.NewText(a.SerialNo)
		acquired := monthStart.AddDate(0, -a.MonthsAgo, 0)
		if a.MaintenanceVendor != "" {
			fa.Next_service_date = types.NewDateFromTime(acquired.AddDate(int(today.Sub(acquired).Hours()/24/365)+1, 0, 0))
		}
		err := fa.InsertOrError(true)
		if err != nil {
			return &InsertError{Table: gtables.FixedAssetTableName, Key: a.No, Err: err}
		}
		counts.FixedAssets++

		book := gtables.NewFADepreciationBook(src)
		book.InitRecord()
		book.Fa_no, book.Depreciation_book_code = fa.No, demoDeprBook
		book.Fa_posting_group = types.NewCode(a.PostingGroup)
		err = book.InsertOrError(true)
		if err != nil {
			return &InsertError{Table: gtables.FADepreciationBookTableName, Key: a.No, Err: err}
		}
		err = book.Validate("depreciation_starting_date", types.NewDateFromTime(acquired))
		if err == nil {
			err = book.Validate("no_of_depreciation_years", types.NewDecimalFromInt(int64(a.Years)))
		}
		if err != nil {
			return &InsertError{Table: gtables.FADepreciationBookTableName, Key: a.No, Err: err}
		}
		if posted {
			err = book.ModifyOrError(true)
			if err != nil {
				return &InsertError{Table: gtables.FADepreciationBookTableName, Key: a.No, Err: err}
			}
			continue
		}

		newEntry := func(date time.Time, postingType gtables.FALedgerEntryFa_posting_type, amount float64, description string) {
			faEntryNo++
			e := gtables.NewFALedgerEntry(src)
			e.Entry_no = faEntryNo
			e.Fa_no, e.Depreciation_book_code = fa.No, demoDeprBook
			e.Fa_posting_date, e.Posting_date, e.Document_date = types.NewDateFromTime(date), types.NewDateFromTime(date), types.NewDateFromTime(date)
			e.Fa_posting_type = postingType
			e.Amount = types.NewDecimal(amount)
			e.Amount_lcy = e.Amount
			if amount > 0 {
				e.Debit_amount = e.Amount
			} else {
				e.Credit_amount = e.Amount.Abs()
			}
			e.Part_of_book_value = true
			e.Part_of_depreciable_basis = postingType == gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost
			e.Description = types.NewText(description)
			e.Fa_class_code, e.Fa_subclass_code, e.Fa_location_code = fa.Fa_class_code, fa.Fa_subclass_code, fa.Fa_location_code
			e.Fa_posting_group = book.Fa_posting_group
			e.Fa_no_budgeted_fa_no = fa.No
			e.Source_code = "GENJNL"
			e.Depreciation_method = gtables.FALedgerEntryDepreciation_method(book.Depreciation_method)
			e.Depreciation_starting_date = book.Depreciation_starting_date
			e.No_of_depreciation_years = book.No_of_depreciation_years
			e.Use_fa_ledger_check = true
			if postingType == gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost {
				e.Document_type = gtables.FALedgerEntry_Document_type.Invoice
				e.Document_no = types.NewCode(fmt.Sprintf("FA-%s", a.No))
				e.Bal_account_type, e.Bal_account_no = gtables.FALedgerEntry_Bal_account_type.Vendor, fa.Vendor_no
			} else {
				e.Document_no = types.NewCode(date.Format("DEPR2006-01"))
				e.No_of_depreciation_days = 30
			}
			faEntries = append(faEntries, e)
		}
		newEntry(acquired, gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost, a.Cost, a.Description)
		monthly := math.Round(a.Cost/float64(a.Years*12)*100) / 100
		var lastDepr time.Time
		for m := acquired.AddDate(0, 1, -1); m.Before(monthStart); m = time.Date(m.Year(), m.Month()+2, 0, 0, 0, 0, 0, m.Location()) {
			newEntry(m, gtables.FALedgerEntry_Fa_posting_type.Depreciation, -monthly, "Depreciation "+m.Format("01/2006"))
			lastDepr = m
		}
		book.Acquisition_date = types.NewDateFromTime(acquired)
		book.G_l_acquisition_date = book.Acquisition_date
		book.Last_acquisition_cost_date = book.Acquisition_date
		if !lastDepr.IsZero() {
			book.Last_depreciation_date = types.NewDateFromTime(lastDepr)
		}
		err = book.ModifyOrError(false)
		if err != nil {
			return &InsertError{Table: gtables.FADepreciationBookTableName, Key: a.No, Err: err}
		}

		// A yearly service of the assets with a maintenance vendor
		if a.MaintenanceVendor != "" {
			for s := acquired.AddDate(1, 0, 14); s.Before(today); s = s.AddDate(1, 0, 0) {
				maintEntryNo++
				e := gtables.NewMaintenanceLedgerEntry(src)
				e.Entry_no = maintEntryNo
				e.Fa_no, e.Depreciation_book_code, e.Maintenance_code = fa.No, demoDeprBook, "SERVICE"
				e.Fa_posting_date, e.Posting_date, e.Document_date = types.NewDateFromTime(s), types.NewDateFromTime(s), types.NewDateFromTime(s)
				e.Document_type = gtables.MaintenanceLedgerEntry_Document_type.Invoice
				e.Document_no = types.NewCode(fmt.Sprintf("MS-%s-%d", a.No, s.Year()))
				e.Description = types.NewText("Periodic service " + s.Format("2006"))
				e.Amount = types.NewDecimal(math.Round(a.Cost*0.012*100) / 100)
				e.Amount_lcy, e.Debit_amount = e.Amount, e.Amount
				e.Fa_posting_group = book.Fa_posting_group
				e.Fa_subclass_code, e.Fa_location_code, e.Fa_class_code = fa.Fa_subclass_code, fa.Fa_location_code, fa.Fa_class_code
				e.Fa_no_budgeted_fa_no = fa.No
				e.Bal_account_type, e.Bal_account_no = gtables.MaintenanceLedgerEntry_Bal_account_type.Vendor, fa.Maintenance_vendor_no
				e.Source_code = "GENJNL"
				maintEntries = append(maintEntries, e)
				book.Last_maintenance_date = e.Fa_posting_date
			}
			if !book.Last_maintenance_date.IsZero() {
				err = book.ModifyOrError(false)
				if err != nil {
					return &InsertError{Table: gtables.FADepreciationBookTableName, Key: a.No, Err: err}
				}
			}
		}

		if a.Insured {
			insEntryNo++
			e := gtables.NewInsCoverageLedgerEntry(src)
			e.Entry_no = insEntryNo
			e.Insurance_no, e.Fa_no, e.Fa_description = insurance.No, fa.No, fa.Description
			e.Posting_date, e.Document_date = types.NewDateFromTime(acquired), types.NewDateFromTime(acquired)
			e.Document_no = types.NewCode(fmt.Sprintf("INS-%s", a.No))
			e.Description = types.NewText("Coverage " + a.Description)
			e.Amount = types.NewDecimal(a.Cost)
			e.Fa_class_code, e.Fa_subclass_code, e.Fa_location_code = fa.Fa_class_code, fa.Fa_subclass_code, fa.Fa_location_code
			e.Source_code = "GENJNL"
			insEntries = append(insEntries, e)
		}
	}

	if len(faEntries) > 0 {
		err = gtables.NewFALedgerEntry(src).InsertAll(faEntries, true)
		if err != nil {
			return &InsertError{Table: gtables.FALedgerEntryTableName, Err: err}
		}
	}
	if len(maintEntries) > 0 {
		err = gtables.NewMaintenanceLedgerEntry(src).InsertAll(maintEntries, true)
		if err != nil {
			return &InsertError{Table: gtables.MaintenanceLedgerEntryTableName, Err: err}
		}
	}
	if len(insEntries) > 0 {
		err = gtables.NewInsCoverageLedgerEntry(src).InsertAll(insEntries, true)
		if err != nil {
			return &InsertError{Table: gtables.InsCoverageLedgerEntryTableName, Err: err}
		}
	}
	counts.FALedgerEntries += len(faEntries)
	counts.MaintenanceLedgerEntries += len(maintEntries)

	// Main asset and its components (Main Asset Component validates and updates both assets)
	for _, a := range data.FixedAssets {
		if a.ComponentOf == "" {
			continue
		}
		comp := gtables.NewMainAssetComponent(src)
		comp.Main_asset_no = types.NewCode(a.ComponentOf)
		err = comp.Validate("fa_no", types.NewCode(a.No))
		if err == nil {
			err = comp.InsertOrError(true)
		}
		if err != nil {
			return &InsertError{Table: gtables.MainAssetComponentTableName, Key: a.No, Err: err}
		}
	}
	return nil
}
