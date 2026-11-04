package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Pages 213 Resource Capacity and 214 Res. Group Capacity: a capacity typed into the matrix is the
// resource's / group's Capacity in the cell's period (a capacity entry with the difference)
func init() {
	pages.OnMatrixValidate(213, func(rec *gtables.Resource, value types.Decimal) error {
		return tables.SetResCapacity(rec, value)
	})
	pages.OnMatrixValidate(214, func(rec *gtables.ResourceGroup, value types.Decimal) error {
		return tables.SetResGroupCapacity(rec, value)
	})
}
