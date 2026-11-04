package resprice_test

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/resprice"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func dec(s string) types.Decimal { return types.MustDecimal(s) }

var (
	resType   = gtables.ResourceCost_Type
	priceType = gtables.ResourcePrice_Type
	costType  = gtables.ResourceCost_Cost_type
)

// Resources R1 (group CONSULT; direct cost 100, unit cost 110, price 200) and R2 (no group;
// 50/55/90)
func resCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	must(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	must(t, tables.CreateCompanyTables(db, "c1", database.DBTypeSQLite))
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	group := gtables.NewResourceGroup(src)
	group.No, group.Name = "CONSULT", "Consultants"
	must(t, group.InsertOrError(false))
	for _, r := range []struct{ no, group, direct, cost, price string }{
		{"R1", "CONSULT", "100", "110", "200"},
		{"R2", "", "50", "55", "90"},
	} {
		res := gtables.NewResource(src)
		res.InitRecord()
		res.No, res.Resource_group_no = types.Code(r.no), types.Code(r.group)
		res.Direct_unit_cost, res.Unit_cost, res.Unit_price = dec(r.direct), dec(r.cost), dec(r.price)
		must(t, res.InsertOrError(false))
	}
	return src
}

func addCost(t *testing.T, src ftables.Source, typ gtables.ResourceCostType, code, workType string, ct gtables.ResourceCostCost_type, direct, cost string) {
	t.Helper()
	resCost := gtables.NewResourceCost(src)
	resCost.Type, resCost.Code, resCost.Work_type_code = typ, types.Code(code), types.Code(workType)
	resCost.Cost_type, resCost.Direct_unit_cost, resCost.Unit_cost = ct, dec(direct), dec(cost)
	must(t, resCost.InsertOrError(false))
}

func addPrice(t *testing.T, src ftables.Source, typ gtables.ResourcePriceType, code, workType, currency, price string) {
	t.Helper()
	resPrice := gtables.NewResourcePrice(src)
	resPrice.Type, resPrice.Code, resPrice.Work_type_code, resPrice.Currency_code = typ, types.Code(code), types.Code(workType), types.Code(currency)
	resPrice.Unit_price = dec(price)
	must(t, resPrice.InsertOrError(false))
}

// Find Cost: the resource's own cost for the work type, else the group's, else the cost of all
// resources, else the resource card; % Extra and LCY Extra add to the cost without the work
// type
func TestFindResCost(t *testing.T) {
	src := resCompany(t)
	addCost(t, src, resType.Resource, "R1", "DAY", costType.Fixed, "700", "800")
	addCost(t, src, resType.GroupResource, "CONSULT", "OVERTIME", costType.Extra, "50", "50")
	addCost(t, src, resType.All, "", "TRAVEL", costType.LCYExtra, "-10", "-5")
	addCost(t, src, resType.Resource, "R2", "", costType.Fixed, "60", "66")

	find := func(no, workType string) (string, string) {
		t.Helper()
		resCost := gtables.NewResourceCost(src)
		resCost.Code, resCost.Work_type_code = types.Code(no), types.Code(workType)
		must(t, resprice.FindResCost(resCost))
		return resCost.Direct_unit_cost.String(), resCost.Unit_cost.String()
	}
	for _, c := range []struct{ no, workType, direct, cost string }{
		{"R1", "DAY", "700", "800"},      // own cost for the work type
		{"R1", "OVERTIME", "150", "165"}, // the group's 50 % on the card's 100/110
		{"R1", "TRAVEL", "90", "105"},    // all resources: -10/-5 on the card's cost
		{"R1", "", "100", "110"},         // the card
		{"R2", "", "60", "66"},           // own cost without work type
		{"R2", "TRAVEL", "50", "61"},     // all resources: -10/-5 on R2's own 60/66
		{"R2", "OVERTIME", "50", "55"},   // R2 has no group: the card
	} {
		direct, cost := find(c.no, c.workType)
		if !dec(direct).Equal(dec(c.direct)) || !dec(cost).Equal(dec(c.cost)) {
			t.Errorf("%s %s: cost %s/%s, want %s/%s", c.no, c.workType, direct, cost, c.direct, c.cost)
		}
	}
}

// Find Price: the resource's own, its group's, then the price of all resources, each in the
// currency or else in local currency; else the card's unit price
func TestFindResPrice(t *testing.T) {
	src := resCompany(t)
	addPrice(t, src, priceType.Resource, "R1", "", "EUR", "20")
	addPrice(t, src, priceType.GroupResource, "CONSULT", "OVERTIME", "", "300")
	addPrice(t, src, priceType.All, "", "TRAVEL", "", "120")

	find := func(no, workType, currency string) (string, string) {
		t.Helper()
		resPrice := gtables.NewResourcePrice(src)
		resPrice.Code, resPrice.Work_type_code, resPrice.Currency_code = types.Code(no), types.Code(workType), types.Code(currency)
		must(t, resprice.FindResPrice(resPrice))
		return resPrice.Unit_price.String(), resPrice.Currency_code.String()
	}
	for _, c := range []struct{ no, workType, currency, price, priceCurrency string }{
		{"R1", "", "EUR", "20", "EUR"},
		{"R1", "", "", "200", ""},            // the card
		{"R1", "OVERTIME", "EUR", "300", ""}, // the group's price in local currency
		{"R2", "OVERTIME", "", "90", ""},     // no group: the card
		{"R2", "TRAVEL", "USD", "120", ""},   // all resources
	} {
		price, currency := find(c.no, c.workType, c.currency)
		if !dec(price).Equal(dec(c.price)) || currency != c.priceCurrency {
			t.Errorf("%s %s %s: price %s %s, want %s %s", c.no, c.workType, c.currency, price, currency, c.price, c.priceCurrency)
		}
	}
}
