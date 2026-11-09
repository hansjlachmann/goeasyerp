package tables

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/format"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Codeunit 311 Item-Check Avail. with the warning of form 342 Check Availability: with the Sales
// & Receivables Setup's Stockout Warning, a line that takes more of an item than the inventory
// less the sales orders (at the line's location, variant and bin) gives a warning. With a
// dialog (a codeunit or report the user runs) the user is asked whether to go on — No stops with
// "The update has been interrupted"; entered on a page (an API request, no dialog to ask in) the
// warning is shown to the user and the quantity is kept. Table triggers call it only for a value
// the user entered (CurrFieldNo), as codeunit 312.

// checkAvailability is the form's state: the item with its filters and the quantities of the
// warning, in the line's unit of measure
type checkAvailability struct {
	src                 ftables.Source
	item                *gtables.Item
	variantCode         types.Code
	locationCode        types.Code
	binCode             types.Code
	unitOfMeasureCode   types.Code
	qtyPerUnitOfMeasure types.Decimal
	newItemNetChange    types.Decimal
	oldItemNetChange    types.Decimal
	itemNetChange       types.Decimal
	initialQtyAvailable types.Decimal
	quantityOnHand      types.Decimal
	qtyOnSalesOrder     types.Decimal
}

// ItemCheckAvailItemJnlLine (ItemJnlCheckLine): an item journal line — also a line made up by a
// BOM journal, project journal or Explode BOM — that takes the item out of inventory
func ItemCheckAvailItemJnlLine(itemJnlLine *gtables.ItemJournalLine) error {
	c := &checkAvailability{src: itemJnlLine}
	show, err := c.itemJnlLineShowWarning(itemJnlLine)
	if err != nil || !show {
		return err
	}
	return c.ask()
}

// ItemCheckAvailSalesLine (SalesLineCheck): a sales line's outstanding quantity instead of what
// the stored order line had
func ItemCheckAvailSalesLine(salesLine *gtables.SalesLine) error {
	c := &checkAvailability{src: salesLine}
	show, err := c.salesLineShowWarning(salesLine)
	if err != nil || !show {
		return err
	}
	return c.ask()
}

// itemJnlLineShowWarning (ItemJnlLineShowWarning): purchases and positive adjustments add to the
// inventory, sales, negative adjustments and transfers take from it
func (c *checkAvailability) itemJnlLineShowWarning(itemJnlLine *gtables.ItemJournalLine) (bool, error) {
	salesSetup := gtables.GetSalesReceivablesSetup(c.src)
	if !salesSetup.Stockout_warning {
		return false, nil
	}
	var itemNetChange types.Decimal
	switch itemJnlLine.Entry_type {
	case gtables.ItemJournalLine_Entry_type.Purchase, gtables.ItemJournalLine_Entry_type.PositiveAdjmt:
		itemNetChange = itemJnlLine.Quantity
	case gtables.ItemJournalLine_Entry_type.Sale, gtables.ItemJournalLine_Entry_type.NegativeAdjmt, gtables.ItemJournalLine_Entry_type.Transfer:
		itemNetChange = itemJnlLine.Quantity.Neg()
	}
	return c.showWarning(
		itemJnlLine.Item_no,
		itemJnlLine.Variant_code,
		itemJnlLine.Location_code,
		itemJnlLine.Bin_code,
		itemJnlLine.Unit_of_measure_code,
		itemJnlLine.Qty_per_unit_of_measure,
		itemNetChange,
		types.Decimal{})
}

// salesLineShowWarning (SalesLineShowWarning): the stored order line's outstanding quantity is
// in the item's Qty. on Sales Order already — taken out when it is at the same place
func (c *checkAvailability) salesLineShowWarning(salesLine *gtables.SalesLine) (bool, error) {
	salesSetup := gtables.GetSalesReceivablesSetup(c.src)
	if !salesSetup.Stockout_warning {
		return false, nil
	}
	oldItemNetChange := types.Decimal{}
	oldSalesLine := gtables.NewSalesLine(salesLine)
	if oldSalesLine.Get(map[string]interface{}{"document_type": int(salesLine.Document_type), "document_no": salesLine.Document_no, "line_no": salesLine.Line_no}) {
		if (oldSalesLine.Document_type == gtables.SalesLine_Document_type.Order) &&
			(oldSalesLine.Variant_code == salesLine.Variant_code) &&
			(oldSalesLine.Location_code == salesLine.Location_code) &&
			(oldSalesLine.Bin_code == salesLine.Bin_code) {
			oldItemNetChange = oldSalesLine.Outstanding_qty_base.Neg()
		}
	}
	return c.showWarning(
		salesLine.No,
		salesLine.Variant_code,
		salesLine.Location_code,
		salesLine.Bin_code,
		salesLine.Unit_of_measure_code,
		salesLine.Qty_per_unit_of_measure,
		salesLine.Outstanding_quantity.Neg(),
		oldItemNetChange)
}

// showWarning (ShowWarning): whether the inventory less the sales orders, with this net change,
// goes below zero
func (c *checkAvailability) showWarning(itemNo, itemVariantCode, itemLocationCode, itemBinCode, unitOfMeasureCode types.Code,
	qtyPerUnitOfMeasure, newItemNetChange, oldItemNetChange types.Decimal) (bool, error) {
	c.variantCode = itemVariantCode
	c.locationCode = itemLocationCode
	c.binCode = itemBinCode
	c.unitOfMeasureCode = unitOfMeasureCode
	c.qtyPerUnitOfMeasure = qtyPerUnitOfMeasure
	c.newItemNetChange = newItemNetChange
	c.oldItemNetChange = c.convertQty(oldItemNetChange)

	if !newItemNetChange.IsNegative() {
		return false, nil
	}

	c.item = gtables.NewItem(c.src)
	err := c.item.GetOrError(itemNo)
	if err != nil {
		return false, err
	}
	for _, flowFilter := range []struct {
		field string
		value types.Code
	}{
		{"variant_filter", itemVariantCode},
		{"location_filter", itemLocationCode},
		{"bin_filter", itemBinCode},
	} {
		err = c.item.SetFlowFilter(flowFilter.field, exactCodeFilter(flowFilter.value))
		if err != nil {
			return false, err
		}
	}
	err = c.item.SetFlowFilter("drop_shipment_filter", system.Format(false))
	if err != nil {
		return false, err
	}

	c.calculate()

	return c.initialQtyAvailable.Add(c.itemNetChange).IsNegative(), nil
}

// calculate (Calculate): the inventory and the quantity on sales orders in the line's unit,
// without what the stored line had, and the quantity available before this line
func (c *checkAvailability) calculate() {
	c.item.CalcFields("quantity_on_hand", "qty_on_sales_order")
	c.quantityOnHand = c.convertQty(c.item.Quantity_on_hand)
	c.qtyOnSalesOrder = c.convertQty(c.item.Qty_on_sales_order).Neg()

	c.itemNetChange = c.newItemNetChange
	c.qtyOnSalesOrder = c.qtyOnSalesOrder.Sub(c.oldItemNetChange)
	c.initialQtyAvailable = c.quantityOnHand.Add(c.qtyOnSalesOrder)
	c.qtyOnSalesOrder = c.qtyOnSalesOrder.Neg()
}

// convertQty (ConvertQty): a quantity in the base unit in the line's unit of measure
func (c *checkAvailability) convertQty(qty types.Decimal) types.Decimal {
	if c.qtyPerUnitOfMeasure.IsZero() {
		c.qtyPerUnitOfMeasure = types.NewDecimalFromInt(1)
	}
	return system.Round(qty.Div(c.qtyPerUnitOfMeasure), types.MustDecimal("0.00001"))
}

// ask (RUNMODAL = ACTION::Yes): the warning with the item's quantities — with a dialog the user
// decides (No: interrupted), else it is shown and the code goes on
func (c *checkAvailability) ask() error {
	language := fcodeunits.CurrentLanguage()
	ts := i18n.GetInstance()
	quantity := func(d types.Decimal) string { return format.Decimal(d, "0:5", language) }
	details := system.StrSubstNo(ts.Message("ITEM_AVAIL_DETAILS", language), c.item.No, c.item.Description,
		shownCode(c.locationCode), shownCode(c.variantCode), shownCode(c.binCode)) + " " +
		system.StrSubstNo(ts.Message("ITEM_AVAIL_QUANTITIES", language), c.unitOfMeasureCode,
			quantity(c.quantityOnHand), quantity(c.qtyOnSalesOrder), quantity(c.itemNetChange.Neg()),
			quantity(c.initialQtyAvailable.Add(c.itemNetChange)))
	heading := ts.Message("ITEM_AVAIL_WARNING", language)
	if !system.GuiAllowed() {
		fcodeunits.Notify(heading + " " + details)
		return nil
	}
	if !system.Confirm(system.StrSubstNo(ts.Message("ITEM_AVAIL_QUESTION", language), heading, details)) {
		return apperrors.JnlUpdateInterrupted()
	}
	return nil
}

// shownCode: a code in the warning, a dash for a blank one
func shownCode(code types.Code) string {
	if code == "" {
		return "-"
	}
	return string(code)
}

// exactCodeFilter: a FlowFilter on one code value, ” for the blank one (classic SETRANGE)
func exactCodeFilter(code types.Code) string {
	if code == "" {
		return "''"
	}
	return string(code)
}

// The lines' CheckItemAvailable: only for a value the user entered (CurrFieldNo)

// checkSalesLineItemAvailable (Sales Line CheckItemAvailable): an item on an order or invoice
// with an outstanding quantity, not applied to a project's entries
func checkSalesLineItemAvailable(rec *gtables.SalesLine) error {
	if (fcodeunits.CurrFieldNo() != 0) &&
		(rec.Document_type == gtables.SalesLine_Document_type.Order || rec.Document_type == gtables.SalesLine_Document_type.Invoice) &&
		(rec.Type == gtables.SalesLine_Type.Item) && (rec.No != "") &&
		rec.Outstanding_quantity.IsPositive() &&
		(rec.Appl_to_project_entry == 0) && (rec.Project_applies_to_id == "") {
		return ItemCheckAvailSalesLine(rec)
	}
	return nil
}

// checkItemJnlItemAvailable (Item Journal Line CheckItemAvailable)
func checkItemJnlItemAvailable(rec *gtables.ItemJournalLine) error {
	if (fcodeunits.CurrFieldNo() != 0) && (rec.Item_no != "") && !rec.Quantity.IsZero() {
		return ItemCheckAvailItemJnlLine(rec)
	}
	return nil
}

// checkBOMJnlItemAvailable (BOM Journal Line CheckItemAvailable): each item component the
// assembly takes, as a negative adjustment at the line's location and bin
func checkBOMJnlItemAvailable(rec *gtables.BOMJournalLine) error {
	if (fcodeunits.CurrFieldNo() != 0) && (rec.Item_no != "") && !rec.Quantity.IsZero() {
		bomComp := gtables.NewBOMComponent(rec)
		bomComp.SetRange("parent_item_no", rec.Item_no)
		bomComp.SetRange("type", gtables.BOMComponent_Type.Item)
		bomComp.SetFilter("no", "<>''")
		itemJnlLine := gtables.NewItemJournalLine(rec)
		itemJnlLine.Entry_type = gtables.ItemJournalLine_Entry_type.NegativeAdjmt
		itemJnlLine.Location_code = rec.Location_code
		itemJnlLine.Bin_code = rec.Bin_code
		for found := bomComp.FindSetBuffered(); found; found = bomComp.Next() {
			item := gtables.NewItem(rec)
			err := item.GetOrError(bomComp.No)
			if err != nil {
				return err
			}
			err = item.TestField("blocked", false)
			if err != nil {
				return err
			}
			itemJnlLine.Item_no = bomComp.No
			itemJnlLine.Variant_code = bomComp.Variant_code
			itemJnlLine.Unit_of_measure_code = bomComp.Unit_of_measure_code
			itemJnlLine.Qty_per_unit_of_measure, err = GetQtyPerUnitOfMeasure(item, bomComp.Unit_of_measure_code)
			if err != nil {
				return err
			}
			itemJnlLine.Quantity = system.Round(rec.Quantity_base.Mul(bomComp.Quantity_per), types.MustDecimal("0.00001"))
			if !itemJnlLine.Quantity.IsZero() {
				err = ItemCheckAvailItemJnlLine(itemJnlLine)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// checkProjectJnlItemAvailable (Project Journal Line CheckItemAvailable): an item the project
// uses, as a negative adjustment
func checkProjectJnlItemAvailable(rec *gtables.ProjectJournalLine) error {
	if (fcodeunits.CurrFieldNo() != 0) &&
		(rec.Type == gtables.ProjectJournalLine_Type.Item) && rec.Quantity.IsPositive() {
		itemJnlLine := gtables.NewItemJournalLine(rec)
		itemJnlLine.Item_no = rec.No
		itemJnlLine.Entry_type = gtables.ItemJournalLine_Entry_type.NegativeAdjmt
		itemJnlLine.Location_code = rec.Location_code
		itemJnlLine.Variant_code = rec.Variant_code
		itemJnlLine.Bin_code = rec.Bin_code
		itemJnlLine.Unit_of_measure_code = rec.Unit_of_measure_code
		itemJnlLine.Qty_per_unit_of_measure = rec.Qty_per_unit_of_measure
		itemJnlLine.Quantity = rec.Quantity
		return ItemCheckAvailItemJnlLine(itemJnlLine)
	}
	return nil
}
