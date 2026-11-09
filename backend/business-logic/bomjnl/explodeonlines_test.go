package bomjnl_test

import (
	"testing"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/bomjnl"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// answerYes: a dialog that answers every question Yes, for the functions that ask first
func answerYes(t *testing.T) {
	d := fcodeunits.OpenDialogWithID("", "explode-bom-test")
	fcodeunits.SetCurrentDialog(d)
	t.Cleanup(fcodeunits.ClearCurrentDialog)
	go func() {
		for ev := range d.Events() {
			if ev.EventType == "confirm" {
				for !d.RespondToConfirm(ev.ConfirmID, true) {
					time.Sleep(time.Millisecond)
				}
			}
		}
	}()
}

// Explode BOM on a sales line (codeunit 63): the starter kit line becomes its components, each
// for the kit quantity × its quantity per; the kit line keeps only its description
func TestSalesExplodeBOM(t *testing.T) {
	src := demoCompany(t)
	answerYes(t)
	order := gtables.NewSalesHeader(src)
	order.Document_type = gtables.SalesHeader_Document_type.Order
	must(t, order.InsertOrError(true))
	must(t, order.Validate("sell_to_customer_no", types.Code("C00010")))
	must(t, order.ModifyOrError(true))
	line := gtables.NewSalesLine(src)
	line.Document_type, line.Document_no, line.Line_no = gtables.SalesLine_Document_type.Order, order.No, 10000
	must(t, line.Validate("type", gtables.SalesLine_Type.Item))
	must(t, line.Validate("no", types.Code("1500")))
	must(t, line.Validate("quantity", dec("3")))
	must(t, line.InsertOrError(true))

	_, err := bomjnl.SalesExplodeBOM(line)
	must(t, err)

	kit := gtables.NewBOMComponent(src)
	kit.SetRange("parent_item_no", types.Code("1500"))
	lines := gtables.NewSalesLine(src)
	lines.SetRange("document_type", gtables.SalesLine_Document_type.Order)
	lines.SetRange("document_no", order.No)
	if lines.Count() != 1+kit.Count() {
		t.Fatalf("%d lines after the explode, want the kit line and its %d components", lines.Count(), kit.Count())
	}
	kitLine := gtables.NewSalesLine(src)
	must(t, kitLine.GetOrError(map[string]interface{}{"document_type": int(gtables.SalesLine_Document_type.Order), "document_no": order.No, "line_no": 10000}))
	if kitLine.Type != gtables.SalesLine_Type.Blank || kitLine.No != "" || kitLine.Description == "" {
		t.Errorf("the kit line: type %v no %q description %q", kitLine.Type, kitLine.No, kitLine.Description)
	}
	kit.SetRange("no", types.Code("1100"))
	if !kit.FindFirst() {
		t.Fatal("no pens in the kit")
	}
	lines.SetRange("no", types.Code("1100"))
	if !lines.FindFirst() || !lines.Quantity.Equal(kit.Quantity_per.Mul(dec("3"))) || lines.Line_no <= 10000 {
		t.Errorf("pens: line %d quantity %s, want 3 × %s", lines.Line_no, lines.Quantity, kit.Quantity_per)
	}
}
