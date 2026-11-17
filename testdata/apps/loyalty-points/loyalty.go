// Package loyaltypoints is the Go code of the test extension app: a posted invoice gives the
// customer loyalty points, and a customer's loyalty tier follows the points it has collected.
package loyaltypoints

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesposting"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// pointsPerInvoice: the points a posted sales invoice gives its bill-to customer
const pointsPerInvoice = 10

// Register registers the app's business logic (the build calls it at startup, after the base
// application's)
func Register() {
	gtables.RegisterCustomerLogic(customerLogic{})
	salesposting.OnAfterPost.Subscribe(addInvoicePoints)
}

// addInvoicePoints: a posted invoice gives the bill-to customer points (inside the posting's
// transaction: an error here undoes the posting)
func addInvoicePoints(e *salesposting.AfterPost) error {
	if e.InvoiceNo == "" {
		return nil
	}
	cust := gtables.NewCustomer(e.SalesHeader)
	err := cust.GetOrError(e.SalesHeader.Bill_to_customer_no)
	if err != nil {
		return err
	}
	cust.Lp_loyalty_points += pointsPerInvoice
	return cust.ModifyOrError(true)
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
