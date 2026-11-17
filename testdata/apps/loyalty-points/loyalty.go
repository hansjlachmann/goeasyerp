// Package loyaltypoints is the Go code of the test extension app: a customer's loyalty tier
// follows the points it has collected.
package loyaltypoints

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Register registers the app's business logic (the build calls it at startup, after the base
// application's)
func Register() {
	gtables.RegisterCustomerLogic(customerLogic{})
}

type customerLogic struct{}

// OnModify: the customer's tier is the highest one whose minimum the points reach (runs after
// the base application's Customer OnModify)
func (customerLogic) OnModify(rec *gtables.Customer) error {
	tier := gtables.NewLPLoyaltyTier(rec)
	rec.Lp_tier_code = ""
	best := -1
	for found := tier.FindSet(); found; found = tier.Next() {
		if tier.Min_points <= rec.Lp_loyalty_points && tier.Min_points > best {
			best = tier.Min_points
			rec.Lp_tier_code = tier.Code
		}
	}
	return nil
}
