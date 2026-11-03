package reservmgt_test

import (
	"testing"

	"github.com/hansjlachmann/openerp/backend/foundation/functions"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// A Reserve = Always sales order line reserves when its quantity is entered (the page's
// OnAfterValidate runs ReserveAlways) — with the setup's Auto Sales Order; a line that may
// reserve but not always does not
func TestReserveAlwaysAutoReserves(t *testing.T) {
	src := demoCompany(t)
	reservSetup := gtables.GetReservationSetup(src)
	reservSetup.Auto_sales_order = true
	must(t, reservSetup.ModifyOrError(false))
	fn, args, err := functions.Resolve("table:Sales_Line.ReserveAlways")
	must(t, err)

	line := salesOrderLine(t, src, "2")
	line.Reserve = gtables.SalesLine_Reserve.Optional
	must(t, line.ModifyOrError(false))
	result, err := fn.Codeunit(args).Run(line)
	must(t, err)
	reload(t, line)
	if !line.Reserved_qty_base.IsZero() || result.Refresh {
		t.Fatalf("Optional line reserved %s", line.Reserved_qty_base)
	}

	line.Reserve = gtables.SalesLine_Reserve.Always
	must(t, line.ModifyOrError(false))
	result, err = fn.Codeunit(args).Run(line)
	must(t, err)
	reload(t, line)
	if !line.Reserved_qty_base.Equal(dec("2")) || !result.Refresh {
		t.Errorf("Always line reserved %s, refresh %v", line.Reserved_qty_base, result.Refresh)
	}
}
