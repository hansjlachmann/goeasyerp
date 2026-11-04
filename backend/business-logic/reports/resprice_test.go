package reports_test

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func mustOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// Resources R1 (cost 100, price 200) and R2 (cost 600, price 1234); rounding method R rounds
// to whole amounts, from 1000 up to tens
func resPriceCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	mustOK(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	mustOK(t, tables.CreateCompanyTables(db, "c1", database.DBTypeSQLite))
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	for _, r := range []struct{ no, cost, price string }{{"R1", "100", "200"}, {"R2", "600", "1234"}} {
		res := gtables.NewResource(src)
		res.InitRecord()
		res.No, res.Unit_cost, res.Direct_unit_cost, res.Unit_price = types.Code(r.no), types.MustDecimal(r.cost), types.MustDecimal(r.cost), types.MustDecimal(r.price)
		mustOK(t, res.InsertOrError(false))
	}
	for _, m := range []struct {
		min, precision string
		typ            gtables.RoundingMethodType
	}{{"0", "1", gtables.RoundingMethod_Type.Nearest}, {"1000", "10", gtables.RoundingMethod_Type.Up}} {
		roundingMethod := gtables.NewRoundingMethod(src)
		roundingMethod.Code, roundingMethod.Minimum_amount, roundingMethod.Precision, roundingMethod.Type = "R", types.MustDecimal(m.min), types.MustDecimal(m.precision), m.typ
		mustOK(t, roundingMethod.InsertOrError(false))
	}
	return src
}

func runReport(t *testing.T, src ftables.Source, id int, options map[string]string) {
	t.Helper()
	_, err := reports.Execute(id, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Options: options})
	mustOK(t, err)
}

// Adjust Resource Costs/Prices: Unit Price × 1.1, rounded by the method (220; 1357.4 → up to
// 1360), the profit % follows
func TestAdjustResourceCostsPrices(t *testing.T) {
	src := resPriceCompany(t)
	runReport(t, src, reports.AdjustResCostsPricesID, map[string]string{"adjust_field": "5", "adjustment_factor": "1.1", "rounding_method": "R"})
	for no, want := range map[string]string{"R1": "220", "R2": "1360"} {
		res := gtables.NewResource(src)
		mustOK(t, res.GetOrError(types.Code(no)))
		if !res.Unit_price.Equal(types.MustDecimal(want)) {
			t.Errorf("%s unit price %s, want %s", no, res.Unit_price, want)
		}
	}
	res := gtables.NewResource(src)
	mustOK(t, res.GetOrError(types.Code("R1")))
	if !res.Profit_percent.Equal(types.MustDecimal("54.54545")) {
		t.Errorf("R1 profit %% %s, want 54.54545", res.Profit_percent)
	}
}

// Suggest Res. Price Chg. (Res.) proposes the resources' prices (only where a price exists,
// unless Create New Prices); Implement makes them the resource prices; Suggest (Price) copies
// the existing prices to a work type, multiplied
func TestResourcePriceChange(t *testing.T) {
	src := resPriceCompany(t)
	changes := func() map[string]string {
		got := map[string]string{}
		resPriceChg := gtables.NewResourcePriceChange(src)
		for found := resPriceChg.FindSet(); found; found = resPriceChg.Next() {
			got[resPriceChg.Code.String()+"/"+resPriceChg.Work_type_code.String()] = resPriceChg.Current_unit_price.String() + "→" + resPriceChg.New_unit_price.String()
		}
		return got
	}

	runReport(t, src, reports.SuggestResPriceChgResID, map[string]string{"unit_price_factor": "1", "create_new_prices": "false"})
	if got := changes(); len(got) != 0 {
		t.Errorf("suggestions without prices and without Create New Prices: %v", got)
	}
	runReport(t, src, reports.SuggestResPriceChgResID, map[string]string{"unit_price_factor": "2", "price_lower_limit": "1000", "rounding_method": "R", "create_new_prices": "true"})
	// R1: 200 is not above the limit; R2: 1234 × 2 = 2468 → up to 2470
	if got := changes(); got["R1/"] != "0→200" || got["R2/"] != "0→2470" {
		t.Errorf("suggestions = %v", got)
	}

	runReport(t, src, reports.ImplementResPriceChangeID, nil)
	resPrice := gtables.NewResourcePrice(src)
	if !resPrice.Get(map[string]interface{}{"type": gtables.ResourcePrice_Type.Resource, "code": "R2", "work_type_code": "", "currency_code": ""}) || !resPrice.Unit_price.Equal(types.MustDecimal("2470")) {
		t.Errorf("R2 price after Implement: %s", resPrice.Unit_price)
	}

	work := gtables.NewWorkType(src)
	work.Code = "OVERTIME"
	mustOK(t, work.InsertOrError(false))
	gtables.NewResourcePriceChange(src).DeleteAll()
	runReport(t, src, reports.SuggestResPriceChgPriceID, map[string]string{"currency_code": "ALL", "work_type_code": "OVERTIME", "unit_price_factor": "1.5", "create_new_prices": "true"})
	if got := changes(); got["R1/OVERTIME"] != "0→300" || got["R2/OVERTIME"] != "0→3705" {
		t.Errorf("suggestions from the prices = %v", got)
	}
}
