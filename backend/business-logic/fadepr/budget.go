package fadepr

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// BudgetDepreciation is codeunit 5615 Budget Depreciation: the depreciation a budget report adds
// to an asset's posted depreciation — the normal depreciation (codeunit 5611) up to two dates. A
// book with Use Custom 1 Depreciation cannot be used; assets depreciated by Declining-Balance 1,
// DB1/SL or DB2/SL are not calculated (the user is told once). One BudgetDepreciation serves one
// report run.
type BudgetDepreciation struct {
	noOfErrors int
	callNo     int
}

// Calculate (Calculate): the asset's depreciation up to endingDate1 and up to endingDate2 (a blank
// date: 0) in the book; 0 when the asset does not have the book
func (b *BudgetDepreciation) Calculate(src ftables.Source, faNo types.Code, endingDate1, endingDate2 types.Date, deprBookCode types.Code) (deprAmount1, deprAmount2 types.Decimal, err error) {
	if b.callNo == 0 {
		b.callNo = 1
		deprBook := gtables.NewDepreciationBook(src)
		err = deprBook.GetOrError(deprBookCode)
		if err != nil {
			return
		}
		if deprBook.Use_custom_1_depreciation {
			err = apperrors.FABudgetCustom1(gtables.DepreciationBookTableName, "use_custom_1_depreciation")
			return
		}
	}
	faDeprBook := gtables.NewFADepreciationBook(src)
	if !faDeprBook.Get(map[string]interface{}{"fa_no": faNo, "depreciation_book_code": deprBookCode}) {
		return
	}
	switch faDeprBook.Depreciation_method {
	case gtables.FADepreciationBook_Depreciation_method.DecliningBalance1,
		gtables.FADepreciationBook_Depreciation_method.DB1SL,
		gtables.FADepreciationBook_Depreciation_method.DB2SL:
		if b.noOfErrors == 0 {
			b.createMessage()
			b.noOfErrors = 1
		}
	default:
		if !endingDate1.IsZero() {
			deprAmount1, _, err = calculateNormal(src, faNo, deprBookCode, endingDate1, EntryAmounts{}, types.Date{}, 0)
			if err != nil {
				return
			}
		}
		if !endingDate2.IsZero() {
			deprAmount2, _, err = calculateNormal(src, faNo, deprBookCode, endingDate2, EntryAmounts{}, types.Date{}, 0)
		}
	}
	return
}

// createMessage (CreateMessage): the declining-balance methods are left out of the budget
func (b *BudgetDepreciation) createMessage() {
	ts := i18n.GetInstance()
	language := fcodeunits.CurrentLanguage()
	method := func(key string) string {
		return ts.OptionCaption(gtables.FADepreciationBookTableName, "depreciation_method", key, language)
	}
	fcodeunits.Notify(ts.MessageWithParams("FA_BUDGET_DECLINING_BALANCE", language,
		method("declining_balance_1"), method("db1_sl"), method("db2_sl")))
}
