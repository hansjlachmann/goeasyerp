package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Pages 16 Chart of Accounts and 17 G/L Account Card: a new account takes the Income/Balance
// of the account above it (OnNewRecord → SetupNewGLAcc)
func init() {
	for _, pageID := range []int{16, 17} {
		pages.OnNewRecord(pageID, func(rec, xRec *gtables.GLAccount, belowXRec bool) {
			tables.SetupNewGLAcc(rec, xRec, belowXRec)
		})
	}
}
