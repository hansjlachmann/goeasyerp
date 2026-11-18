package loyaltypoints

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/extensions"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// appID is the app's id (application.yaml)
const appID = "0f72f6dc-d125-44df-90c1-e61420d854fe"

// The app's OnModify runs after the base application's: the tier follows the points, and the
// base's Last Date Modified is still set
func TestTierFollowsPoints(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := tables.CreateCompanyTables(db, "test", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, "test", database.DBTypeSQLite)
	extensions.Install(appID, Register)
	t.Cleanup(func() { extensions.SetDisabled() })

	for code, min := range map[string]int{"SILVER": 100, "GOLD": 500} {
		tier := gtables.NewLPLoyaltyTier(src)
		tier.Code = types.Code(code)
		tier.Min_points = min
		if !tier.Insert(true) {
			t.Fatalf("insert tier %s: %v", code, tier.TriggerError())
		}
	}
	cust := gtables.NewCustomer(src)
	cust.No = "C1"
	if !cust.Insert(false) {
		t.Fatal("insert customer")
	}
	for points, want := range map[int]string{50: "", 120: "SILVER", 700: "GOLD"} {
		cust.Lp_loyalty_points = points
		if !cust.Modify(true) {
			t.Fatalf("modify: %v", cust.TriggerError())
		}
		if string(cust.Lp_tier_code) != want {
			t.Errorf("%d points: tier %q, want %q", points, cust.Lp_tier_code, want)
		}
		if cust.Last_date_modified.IsZero() {
			t.Error("the base application's OnModify did not run")
		}
	}

	// Negative points are refused by the field's check (validation min 0)
	cust.Lp_loyalty_points = -1
	if cust.Modify(true) {
		t.Error("negative points accepted")
	}

	// Disabled (Installed Apps), the app's logic and field checks do not run; its data stays
	extensions.SetDisabled(appID)
	cust.Lp_loyalty_points = 50
	cust.Lp_tier_code = "GOLD"
	if !cust.Modify(true) {
		t.Fatalf("modify with the app disabled: %v", cust.TriggerError())
	}
	if cust.Lp_tier_code != "GOLD" {
		t.Errorf("the disabled app's OnModify ran: tier %q", cust.Lp_tier_code)
	}
	cust.Lp_loyalty_points = -1
	if !cust.Modify(true) {
		t.Errorf("the disabled app's field check ran: %v", cust.TriggerError())
	}
	// … nor its fields' table relation checks (Validate)
	if err := cust.Validate("lp_tier_code", types.Code("NOSUCH")); err != nil {
		t.Errorf("the disabled app's relation check ran: %v", err)
	}
	extensions.SetDisabled()
	if err := cust.Validate("lp_tier_code", types.Code("NOSUCH")); err == nil {
		t.Error("a tier that does not exist accepted with the app enabled")
	}
}
