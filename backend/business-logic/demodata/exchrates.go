package demodata

import (
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The demo exchange rate adjustment (Options.AdjustExchRates, codeunit 901): from the first of
// last month the euro, the dollar and the pound have new rates; Adjust Exchange Rates at last
// month's end brings the GBP bank account to them (document EXCHyyyymm).
var demoNewRates = map[types.Code]int64{"EUR": 1165, "USD": 1035, "GBP": 1370}

func adjustDemoExchRates(src ftables.Source, today types.Date, adjust func(src ftables.Source, postingDate types.Date, documentNo types.Code) error, counts *Counts) error {
	firstOfMonth := types.NewDate(today.Year(), today.Month(), 1)
	from := firstOfMonth.AddMonths(-1)
	for _, code := range []types.Code{"EUR", "USD", "GBP"} {
		rate := gtables.NewCurrencyExchangeRate(src)
		rate.Currency_code = code
		rate.Starting_date = from
		rate.Exchange_rate_amount = types.NewDecimalFromInt(100)
		rate.Adjustment_exch_rate_amount = types.NewDecimalFromInt(100)
		rate.Relational_exch_rate_amount = types.NewDecimalFromInt(demoNewRates[code])
		rate.Relational_adjmt_exch_rate_amt = types.NewDecimalFromInt(demoNewRates[code])
		err := rate.InsertOrError(false)
		if err != nil {
			return err
		}
	}
	postingDate := firstOfMonth.AddDays(-1)
	err := adjust(src, postingDate, types.Code("EXCH"+postingDate.Format("200601")))
	if err != nil {
		return err
	}
	counts.ExchRateAdjmtRegs = gtables.NewExchRateAdjmtReg(src).Count()
	return nil
}
