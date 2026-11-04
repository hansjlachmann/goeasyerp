package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 5646 Insurance Statistics: Over/Underinsured is the policy coverage less the total value
// insured (OnAfterGetRecord)
func init() {
	pages.OnAfterGetRecord(5646, func(rec *gtables.Insurance, page *pages.Control) {
		insurance := gtables.NewInsurance(rec)
		insurance.No = rec.No
		insurance.CalcFields("total_value_insured")
		page.SetValue("over_underinsured", rec.Policy_coverage.Sub(insurance.Total_value_insured))
	})
}
