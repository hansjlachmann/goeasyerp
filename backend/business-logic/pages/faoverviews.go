package pages

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The fixed asset overviews:
//   - 5603 Main Asset Statistics (classic form 5603): a main asset's depreciation book with the
//     amounts of its components in the book — the disposed ones' proceeds and gain/loss, the
//     others' book value, depreciable basis and amounts by posting type with the last dates;
//   - 5622 Total Value Insured per FA (form 5622): the fixed assets against the insurances, a cell
//     the coverage of the asset by the insurance (Ins. Coverage Ledger Entries not of a disposed
//     asset) in the period;
//   - 5662 FA Posting Types Overview (form 5662): the FA depreciation books against the posting
//     types (table 5647 FA Matrix Posting Type), a cell the book's FlowField in the period.
const (
	mainAssetStatisticsPageID    = 5603
	totalValueInsuredPerFAPageID = 5622
	faPostingTypesOverviewPageID = 5662
)

// faMatrixPostingTypes: the FA depreciation book FlowFields of the posting types overview, in the
// order of table 5647 (CreateTypes)
var faMatrixPostingTypes = []string{"book_value", "acquisition_cost", "depreciation", "write_down", "appreciation",
	"custom_1", "custom_2", "proceeds_on_disposal", "gain_loss", "depreciable_basis", "salvage_value"}

func init() {
	pages.OnAfterGetRecord(mainAssetStatisticsPageID, func(rec *gtables.FADepreciationBook, page *pages.Control) {
		mainAssetStatistics(rec, page)
	})
	pages.OnMatrixCalculate(totalValueInsuredPerFAPageID, func(card *gtables.FixedAsset, fa *gtables.FixedAsset) ([]pages.MatrixColumn, map[string][]string, error) {
		return totalValueInsuredPerFA(fa)
	})
	pages.OnMatrixCalculate(faPostingTypesOverviewPageID, func(card *gtables.FADepreciationBook, faDeprBook *gtables.FADepreciationBook) ([]pages.MatrixColumn, map[string][]string, error) {
		return faPostingTypesOverview(faDeprBook)
	})
}

// mainAssetStatistics (form 5603 OnAfterGetRecord): the totals of a main asset's components in
// its depreciation book; nothing for an asset that is not a main asset
func mainAssetStatistics(rec *gtables.FADepreciationBook, page *pages.Control) {
	var disposalPrice, gainLoss, bookValue, deprBasis types.Decimal
	var disposalDate, glAcqDate, faAcqDate types.Date
	amounts := map[string]types.Decimal{}
	lastDates := map[string]types.Date{}
	if rec.Main_asset_component == gtables.FADepreciationBook_Main_asset_component.MainAsset {
		faDeprBook := gtables.NewFADepreciationBook(rec)
		faDeprBook.SetCurrentKey("depreciation_book_code", "component_of_main_asset")
		faDeprBook.SetRange("depreciation_book_code", rec.Depreciation_book_code)
		faDeprBook.SetRange("component_of_main_asset", rec.Component_of_main_asset)
		for found := faDeprBook.FindSet(); found; found = faDeprBook.Next() {
			if !faDeprBook.Disposal_date.IsZero() {
				faDeprBook.CalcFields("proceeds_on_disposal", "gain_loss")
				disposalPrice = disposalPrice.Add(faDeprBook.Proceeds_on_disposal)
				gainLoss = gainLoss.Add(faDeprBook.Gain_loss)
				disposalDate = getMinDate(disposalDate, faDeprBook.Disposal_date)
			}
			if faDeprBook.Disposal_date.IsZero() {
				if !faDeprBook.Last_acquisition_cost_date.IsZero() {
					faDeprBook.CalcFields("book_value", "depreciable_basis")
					bookValue = bookValue.Add(faDeprBook.Book_value)
					deprBasis = deprBasis.Add(faDeprBook.Depreciable_basis)
					glAcqDate = getMinDate(glAcqDate, faDeprBook.G_l_acquisition_date)
					faAcqDate = getMinDate(faAcqDate, faDeprBook.Acquisition_date)
				}
				// CalcAmount: the posting type's amount of the books that have a last date of it
				for _, c := range []struct {
					field    string
					lastDate types.Date
					amount   *types.Decimal
				}{
					{"acquisition_cost", faDeprBook.Last_acquisition_cost_date, &faDeprBook.Acquisition_cost},
					{"depreciation", faDeprBook.Last_depreciation_date, &faDeprBook.Depreciation},
					{"write_down", faDeprBook.Last_write_down_date, &faDeprBook.Write_down},
					{"appreciation", faDeprBook.Last_appreciation_date, &faDeprBook.Appreciation},
					{"custom_1", faDeprBook.Last_custom_1_date, &faDeprBook.Custom_1},
					{"custom_2", faDeprBook.Last_custom_2_date, &faDeprBook.Custom_2},
					{"maintenance", faDeprBook.Last_maintenance_date, &faDeprBook.Maintenance},
					{"salvage_value", faDeprBook.Last_salvage_value_date, &faDeprBook.Salvage_value},
				} {
					if c.lastDate.IsZero() {
						continue
					}
					faDeprBook.CalcFields(c.field)
					amounts[c.field] = amounts[c.field].Add(*c.amount)
					if lastDates[c.field].Before(c.lastDate) {
						lastDates[c.field] = c.lastDate
					}
				}
			}
		}
	}
	for _, field := range []string{"acquisition_cost", "depreciation", "write_down", "appreciation", "custom_1", "custom_2", "maintenance", "salvage_value"} {
		page.SetValue("total_"+field, amounts[field])
		page.SetValue("last_"+field+"_date", lastDates[field])
	}
	page.SetValue("total_book_value", bookValue)
	page.SetValue("total_depreciable_basis", deprBasis)
	page.SetValue("total_proceeds_on_disposal", disposalPrice)
	page.SetValue("total_gain_loss", gainLoss)
	page.SetValue("first_disposal_date", disposalDate)
	page.SetValue("first_acquisition_date", faAcqDate)
	page.SetValue("first_g_l_acquisition_date", glAcqDate)
}

// getMinDate (GetMinDate): the earlier of two dates, a blank first date counting as none
func getMinDate(date1, date2 types.Date) types.Date {
	if date1.IsZero() || date2.Before(date1) {
		return date2
	}
	return date1
}

// totalValueInsuredPerFA (form 5622): a column per insurance, a cell the asset's coverage by it —
// the Ins. Coverage Ledger Entries of the asset not disposed, posted in the FA Posting Date Filter
func totalValueInsuredPerFA(fa *gtables.FixedAsset) ([]pages.MatrixColumn, map[string][]string, error) {
	insurance := gtables.NewInsurance(fa)
	var columns []pages.MatrixColumn
	column := map[types.Code]int{}
	for found := insurance.FindSet(); found; found = insurance.Next() {
		column[insurance.No] = len(columns)
		columns = append(columns, pages.MatrixColumn{Name: insurance.No.String(), DecimalPlaces: "2:2",
			DrilldownFilters: map[string]string{"insurance_no": insurance.No.String()}})
	}

	amounts := map[types.Code][]types.Decimal{}
	insCoverageLedgEntry := gtables.NewInsCoverageLedgerEntry(fa)
	insCoverageLedgEntry.SetRange("disposed_fa", false)
	if fa.Fa_posting_date_filter != "" {
		err := ftables.ApplyFilter(insCoverageLedgEntry, ftables.FieldFilter{Field: "posting_date", Expression: fa.Fa_posting_date_filter})
		if err != nil {
			return nil, nil, err
		}
	}
	for found := insCoverageLedgEntry.FindSetBuffered(); found; found = insCoverageLedgEntry.Next() {
		i, ok := column[insCoverageLedgEntry.Insurance_no]
		if !ok {
			continue
		}
		if amounts[insCoverageLedgEntry.Fa_no] == nil {
			amounts[insCoverageLedgEntry.Fa_no] = make([]types.Decimal, len(columns))
		}
		amounts[insCoverageLedgEntry.Fa_no][i] = amounts[insCoverageLedgEntry.Fa_no][i].Add(insCoverageLedgEntry.Amount)
	}

	values := map[string][]string{}
	for found := fa.FindSet(); found; found = fa.Next() {
		cells := make([]string, len(columns))
		for i, amount := range amounts[fa.No] {
			if !amount.IsZero() {
				cells[i] = amount.String()
			}
		}
		values[fa.No.String()] = cells
	}
	return columns, values, nil
}

// faPostingTypesOverview (form 5662): a column per posting type, a cell the depreciation book's
// FlowField of it in the FA Posting Date Filter (UpdateAmounts)
func faPostingTypesOverview(faDeprBook *gtables.FADepreciationBook) ([]pages.MatrixColumn, map[string][]string, error) {
	language := fcodeunits.CurrentLanguage()
	var columns []pages.MatrixColumn
	for _, field := range faMatrixPostingTypes {
		columns = append(columns, pages.MatrixColumn{Name: faDeprBook.GetFieldCaption(field, language), DecimalPlaces: "2:2"})
	}
	var keys []string
	var records []map[string]interface{}
	for found := faDeprBook.FindSet(); found; found = faDeprBook.Next() {
		keys = append(keys, ftables.KeyString(faDeprBook))
		records = append(records, faDeprBook.ToMap())
	}
	faDeprBook.CalcFieldsForRecords(records, faMatrixPostingTypes...)
	values := map[string][]string{}
	for i, key := range keys {
		cells := make([]string, len(faMatrixPostingTypes))
		for j, field := range faMatrixPostingTypes {
			amount := decimalOf(records[i][field])
			if !amount.IsZero() {
				cells[j] = amount.String()
			}
		}
		values[key] = cells
	}
	return columns, values, nil
}
