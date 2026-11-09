package reports_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// enteredByUser validates a field as the user entering it does (CurrFieldNo) and returns the
// notes it gave
func enteredByUser(t *testing.T, rec interface {
	Validate(string, interface{}) error
	FieldNo(string) int
}, field string, value interface{}) ([]string, error) {
	t.Helper()
	fcodeunits.StartNotes()
	fcodeunits.SetCurrFieldNo(rec.FieldNo(field))
	err := rec.Validate(field, value)
	fcodeunits.SetCurrFieldNo(0)
	return fcodeunits.TakeNotes(), err
}

func stockoutWarning(t *testing.T, src ftables.Source, on bool) {
	t.Helper()
	salesSetup := gtables.GetSalesReceivablesSetup(src)
	salesSetup.Stockout_warning = on
	mustOK(t, salesSetup.ModifyOrError(false))
}

// availableAt: the item's inventory less its sales orders at a location (blank variant and bin)
func availableAt(t *testing.T, src ftables.Source, itemNo, location types.Code) types.Decimal {
	t.Helper()
	item := gtables.NewItem(src)
	mustOK(t, item.GetOrError(itemNo))
	mustOK(t, item.SetFlowFilter("location_filter", map[bool]string{true: "''", false: string(location)}[location == ""]))
	mustOK(t, item.SetFlowFilter("variant_filter", "''"))
	mustOK(t, item.SetFlowFilter("bin_filter", "''"))
	item.CalcFields("quantity_on_hand", "qty_on_sales_order")
	return item.Quantity_on_hand.Sub(item.Qty_on_sales_order)
}

// Codeunit 311 on a sales order line: a quantity the user enters that takes more than the
// inventory less the sales orders warns (the line's own stored quantity is not counted twice);
// a quantity code sets, one that is available, or the setup without Stockout Warning do not
func TestItemAvailabilityWarningSalesLine(t *testing.T) {
	src := documentsCompany(t)
	stockoutWarning(t, src, true)

	order := gtables.NewSalesHeader(src)
	order.Document_type = gtables.SalesHeader_Document_type.Order
	mustOK(t, order.InsertOrError(true))
	mustOK(t, order.Validate("sell_to_customer_no", types.Code("C00010")))
	mustOK(t, order.ModifyOrError(true))
	line := gtables.NewSalesLine(src)
	line.Document_type = gtables.SalesLine_Document_type.Order
	line.Document_no = order.No
	line.Line_no = 10000
	mustOK(t, line.Validate("type", gtables.SalesLine_Type.Item))
	mustOK(t, line.Validate("no", types.Code("1000")))
	mustOK(t, line.InsertOrError(true))

	available := availableAt(t, src, "1000", line.Location_code)
	if !available.IsPositive() {
		t.Fatalf("item 1000 has nothing available at %q: %s", line.Location_code, available)
	}
	tooMuch := available.Add(types.NewDecimalFromInt(1))

	// set by code: no warning
	fcodeunits.StartNotes()
	mustOK(t, line.Validate("quantity", tooMuch))
	if notes := fcodeunits.TakeNotes(); len(notes) != 0 {
		t.Errorf("a quantity set by code warned: %v", notes)
	}

	// all that is available, stored on the order, and entered again: no warning
	notes, err := enteredByUser(t, line, "quantity", available)
	mustOK(t, err)
	if len(notes) != 0 {
		t.Errorf("the available quantity warned: %v", notes)
	}
	mustOK(t, line.ModifyOrError(true))
	notes, err = enteredByUser(t, line, "quantity", available)
	mustOK(t, err)
	if len(notes) != 0 {
		t.Errorf("the stored line was counted twice: %v", notes)
	}

	// one more: the warning, the quantity kept
	notes, err = enteredByUser(t, line, "quantity", tooMuch)
	mustOK(t, err)
	if len(notes) != 1 || !strings.Contains(notes[0], "not sufficient") || !strings.Contains(notes[0], "1000") {
		t.Errorf("availability warning: %v", notes)
	}
	if !line.Quantity.Equal(tooMuch) {
		t.Errorf("the quantity was not kept: %s", line.Quantity)
	}

	// without Stockout Warning
	stockoutWarning(t, src, false)
	notes, err = enteredByUser(t, line, "quantity", tooMuch.Add(types.NewDecimalFromInt(1)))
	mustOK(t, err)
	if len(notes) != 0 {
		t.Errorf("warned without Stockout Warning: %v", notes)
	}
}

// Codeunit 311 on item journal lines: what a negative adjustment takes out warns, a positive
// adjustment adds and does not; with a dialog No interrupts the change
func TestItemAvailabilityWarningItemJnlLine(t *testing.T) {
	src := documentsCompany(t)
	stockoutWarning(t, src, true)

	itemJnlLine := gtables.NewItemJournalLine(src)
	itemJnlLine.Entry_type = gtables.ItemJournalLine_Entry_type.NegativeAdjmt
	mustOK(t, itemJnlLine.Validate("item_no", types.Code("1000")))
	tooMuch := availableAt(t, src, "1000", itemJnlLine.Location_code).Add(types.NewDecimalFromInt(1000))

	notes, err := enteredByUser(t, itemJnlLine, "quantity", tooMuch)
	mustOK(t, err)
	if len(notes) != 1 || !strings.Contains(notes[0], "not sufficient") {
		t.Errorf("negative adjustment: %v", notes)
	}

	itemJnlLine.Entry_type = gtables.ItemJournalLine_Entry_type.PositiveAdjmt
	notes, err = enteredByUser(t, itemJnlLine, "quantity", tooMuch)
	mustOK(t, err)
	if len(notes) != 0 {
		t.Errorf("a positive adjustment warned: %v", notes)
	}

	// with a dialog the user is asked: No stops the change
	itemJnlLine.Entry_type = gtables.ItemJournalLine_Entry_type.Sale
	d := fcodeunits.OpenDialogWithID("", "item-check-avail-test")
	fcodeunits.SetCurrentDialog(d)
	t.Cleanup(fcodeunits.ClearCurrentDialog)
	asked := make(chan string, 1)
	go func() {
		for ev := range d.Events() {
			if ev.EventType == "confirm" {
				asked <- ev.Message
				for !d.RespondToConfirm(ev.ConfirmID, false) {
					time.Sleep(time.Millisecond)
				}
			}
		}
	}()
	_, err = enteredByUser(t, itemJnlLine, "quantity", tooMuch)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.ErrJnlUpdateInterrupted {
		t.Errorf("No did not interrupt: %v", err)
	}
	select {
	case question := <-asked:
		if !strings.Contains(question, "Do you still want to record the quantity?") {
			t.Errorf("question: %q", question)
		}
	default:
		t.Errorf("the user was not asked")
	}
}

// Codeunit 311 on a BOM journal line: assembling more kits than the components on inventory
// allow warns for the components
func TestItemAvailabilityWarningBOMJnlLine(t *testing.T) {
	src := documentsCompany(t)
	stockoutWarning(t, src, true)

	bomJnlLine := gtables.NewBOMJournalLine(src)
	mustOK(t, bomJnlLine.Validate("item_no", types.Code("2100")))
	notes, err := enteredByUser(t, bomJnlLine, "quantity", types.NewDecimalFromInt(1000000))
	mustOK(t, err)
	if len(notes) == 0 || !strings.Contains(notes[0], "not sufficient") || strings.Contains(notes[0], "2100 ") {
		t.Errorf("BOM journal line: %v", notes)
	}
	notes, err = enteredByUser(t, bomJnlLine, "quantity", types.NewDecimalFromInt(0))
	mustOK(t, err)
	if len(notes) != 0 {
		t.Errorf("quantity 0 warned: %v", notes)
	}
}

// pageAnswers answers the pages a function opens (RunModal) with the given answers in turn and
// every question with yes; it returns the run_page events it got
func pageAnswers(t *testing.T, answers ...map[string]string) *[]fcodeunits.ProgressEvent {
	t.Helper()
	d := fcodeunits.OpenDialogWithID("", "item-availability-test-"+t.Name())
	fcodeunits.SetCurrentDialog(d)
	t.Cleanup(fcodeunits.ClearCurrentDialog)
	var pages []fcodeunits.ProgressEvent
	next := 0
	go func() {
		for ev := range d.Events() {
			switch ev.EventType {
			case "confirm":
				for !d.RespondToConfirm(ev.ConfirmID, true) {
					time.Sleep(time.Millisecond)
				}
			case "run_page":
				pages = append(pages, ev)
				answer := map[string]string{}
				if next < len(answers) {
					answer = answers[next]
				}
				next++
				for !d.RespondToInput(ev.ConfirmID, answer) {
					time.Sleep(time.Millisecond)
				}
			}
		}
	}()
	return &pages
}

// Item Availability by on a sales line: the page of the type opens on the item with the line's
// other places and its date as FlowFilters; the location or period picked becomes the line's
// after the question; Cancel changes nothing
func TestSalesLineItemAvailability(t *testing.T) {
	src := documentsCompany(t)
	order := gtables.NewSalesHeader(src)
	order.Document_type = gtables.SalesHeader_Document_type.Order
	mustOK(t, order.InsertOrError(true))
	mustOK(t, order.Validate("sell_to_customer_no", types.Code("C00010")))
	mustOK(t, order.ModifyOrError(true))
	line := gtables.NewSalesLine(src)
	line.Document_type = gtables.SalesLine_Document_type.Order
	line.Document_no = order.No
	line.Line_no = 10000
	mustOK(t, line.Validate("type", gtables.SalesLine_Type.Item))
	mustOK(t, line.Validate("no", types.Code("1000")))
	mustOK(t, line.Validate("location_code", types.Code("MAIN")))
	mustOK(t, line.Validate("quantity", types.NewDecimalFromInt(1)))
	mustOK(t, line.InsertOrError(true))
	key := map[string]interface{}{"document_type": int(line.Document_type), "document_no": line.Document_no, "line_no": line.Line_no}

	// by location: OSLO picked
	pages := pageAnswers(t, map[string]string{"action": "LookupOK", "key": "1000", "row": "OSLO"})
	result, err := tables.SalesLineItemAvailability(line, tables.ItemAvailabilityByLocation)
	mustOK(t, err)
	if !result.Refresh {
		t.Errorf("the lines are not reloaded: %+v", result)
	}
	stored := gtables.NewSalesLine(src)
	mustOK(t, stored.GetOrError(key))
	if stored.Location_code != "OSLO" {
		t.Errorf("location %q, want OSLO", stored.Location_code)
	}
	if len(*pages) != 1 {
		t.Fatalf("pages opened: %d", len(*pages))
	}
	ev := (*pages)[0]
	flowFilters, _ := ev.Data["flow_filters"].([]ftables.FieldFilter)
	got := map[string]string{}
	for _, f := range flowFilters {
		got[f.Field] = f.Expression
	}
	if ev.Data["page_id"] != tables.ItemAvailByLocationPageID || ev.Data["lookup_mode"] != true ||
		got["variant_filter"] != "''" || got["bin_filter"] != "''" || got["location_filter"] != "" ||
		got["date_filter"] != ".."+line.Shipment_date.String() {
		t.Errorf("page %v lookup %v flow filters %v", ev.Data["page_id"], ev.Data["lookup_mode"], got)
	}

	// by period: the period's start becomes the shipment date
	_ = pageAnswers(t, map[string]string{"action": "LookupOK", "key": "1000", "row": "2027-01-01"})
	_, err = tables.SalesLineItemAvailability(stored, tables.ItemAvailabilityByDate)
	mustOK(t, err)
	mustOK(t, stored.GetOrError(key))
	if stored.Shipment_date.String() != "2027-01-01" {
		t.Errorf("shipment date %s, want 2027-01-01", stored.Shipment_date)
	}

	// Cancel: nothing changes
	_ = pageAnswers(t)
	_, err = tables.SalesLineItemAvailability(stored, tables.ItemAvailabilityByLocation)
	mustOK(t, err)
	mustOK(t, stored.GetOrError(key))
	if stored.Location_code != "OSLO" {
		t.Errorf("cancel changed the location to %q", stored.Location_code)
	}
}
