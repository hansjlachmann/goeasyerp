package itemjnl

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

const itemLedgEntryTable = gtables.ItemLedgerEntryTableName

var unitPrecision = types.MustDecimal("0.00001")

// ItemJnlPostLine is codeunit 22 Item Jnl.-Post Line: it posts an item journal line — an item
// ledger entry applied to the open entries of the other sign (by Applies-to Entry, else FIFO or
// LIFO by the item's costing method), its application entries, the item's unit cost (average
// and last direct cost), the Item Register and, with Automatic Cost Posting, the inventory value
// to the G/L (codeunit 12). Open it on the posting's transaction; one instance per posting keeps
// the entry numbers, the Item Register and the G/L Register.
//
// Not migrated yet: reservations, item tracking (serial and lot numbers), bins, the additional
// reporting currency, cost adjustment (Adjust Cost - Item Entries); Costing Method Specific
// (it needs serial numbers) is refused.
type ItemJnlPostLine struct {
	src ftables.Source

	itemJnlLine        *gtables.ItemJournalLine
	itemLedgEntry      *gtables.ItemLedgerEntry
	item               *gtables.Item
	itemReg            *gtables.ItemRegister
	invtSetup          *gtables.InventorySetup
	sourceCodeSetup    *gtables.SourceCodeSetup
	genPostingSetup    *gtables.GeneralPostingSetup
	itemJnlCheckLine   ItemJnlCheckLine
	genJnlPostLine     *genjnl.GenJnlPostLine
	checkLine          bool
	entryNo            int
	physInvtEntryNo    int
	applicationEntryNo int
	invtSetupRead      bool
}

// NewItemJnlPostLine opens the posting codeunit in the posting's database (its transaction)
func NewItemJnlPostLine(src ftables.Source) *ItemJnlPostLine {
	return &ItemJnlPostLine{src: src}
}

// Run checks and posts a journal line (OnRun); the line gets the posting's changes (Item Shpt.
// Entry No.)
func (p *ItemJnlPostLine) Run(itemJnlLine *gtables.ItemJournalLine) error {
	p.checkLine = true
	return p.run(itemJnlLine)
}

// RunWithoutCheck posts a journal line its caller checked (Item Jnl.-Post Batch)
func (p *ItemJnlPostLine) RunWithoutCheck(itemJnlLine *gtables.ItemJournalLine) error {
	p.checkLine = false
	return p.run(itemJnlLine)
}

func (p *ItemJnlPostLine) run(itemJnlLine *gtables.ItemJournalLine) error {
	p.itemJnlLine = gtables.NewItemJournalLine(p.src)
	p.itemJnlLine.Copy(itemJnlLine)
	err := p.code()
	if err != nil {
		return err
	}
	itemJnlLine.Item_shpt_entry_no = p.itemJnlLine.Item_shpt_entry_no
	return nil
}

// GetItemReg returns the Item Register of the posting (No. 0 before the first entry)
func (p *ItemJnlPostLine) GetItemReg() *gtables.ItemRegister {
	if p.itemReg == nil {
		return gtables.NewItemRegister(p.src)
	}
	return p.itemReg
}

// GetGLReg returns the G/L Register of the inventory postings (No. 0 when none were posted)
func (p *ItemJnlPostLine) GetGLReg() *gtables.GLRegister {
	if p.genJnlPostLine == nil {
		return gtables.NewGLRegister(p.src)
	}
	return p.genJnlPostLine.GetGLReg()
}

func (p *ItemJnlPostLine) code() error {
	itemJnlLine := p.itemJnlLine
	if tables.ItemJnlLineEmptyLine(itemJnlLine) {
		return nil
	}

	if p.checkLine {
		err := p.itemJnlCheckLine.RunCheck(itemJnlLine)
		if err != nil {
			return err
		}
	}

	if itemJnlLine.Document_date.IsZero() {
		itemJnlLine.Document_date = itemJnlLine.Posting_date
	}
	if !p.invtSetupRead {
		p.invtSetupRead = true
		p.invtSetup = gtables.GetInventorySetup(p.src)
		if p.invtSetup.Automatic_cost_posting {
			p.sourceCodeSetup = gtables.GetSourceCodeSetup(p.src)
		}
	}

	if p.entryNo == 0 {
		itemLedgEntry := gtables.NewItemLedgerEntry(p.src)
		itemLedgEntry.LockTable()
		if itemLedgEntry.FindLast() {
			p.entryNo = itemLedgEntry.Entry_no
		}
	}

	p.item = gtables.NewItem(p.src)
	err := p.item.GetOrError(itemJnlLine.Item_no)
	if err != nil {
		return err
	}
	err = p.item.TestField("blocked", false)
	if err != nil {
		return err
	}

	if p.item.Reserve == gtables.Item_Reserve.Always &&
		!itemJnlLine.Quantity_base.IsZero() &&
		(tables.ItemJnlLineSigned(itemJnlLine, itemJnlLine.Quantity_base).IsNegative() || itemJnlLine.Entry_type == gtables.ItemJournalLine_Entry_type.Transfer) {
		return apperrors.ItemMustBeReserved(gtables.ItemTableName, "reserve", p.item.No.String(), p.item.Reserve)
	}

	if itemJnlLine.Inventory_posting_group == "" {
		err = p.item.TestField("inventory_posting_group")
		if err != nil {
			return err
		}
		itemJnlLine.Inventory_posting_group = p.item.Inventory_posting_group
	}

	if p.genPostingSetup == nil ||
		itemJnlLine.Gen_bus_posting_group != p.genPostingSetup.Gen_bus_posting_group ||
		itemJnlLine.Gen_prod_posting_group != p.genPostingSetup.Gen_prod_posting_group {
		p.genPostingSetup = gtables.NewGeneralPostingSetup(p.src)
		err = p.genPostingSetup.GetOrError(map[string]interface{}{"gen_bus_posting_group": itemJnlLine.Gen_bus_posting_group, "gen_prod_posting_group": itemJnlLine.Gen_prod_posting_group})
		if err != nil {
			return err
		}
	}

	if itemJnlLine.Quantity.IsZero() && !itemJnlLine.Invoiced_quantity.IsZero() {
		err = itemJnlLine.TestField("item_shpt_entry_no")
		if err != nil {
			return err
		}
	} else {
		if !itemJnlLine.Quantity.Equal(itemJnlLine.Invoiced_quantity) {
			err = itemJnlLine.TestField("invoiced_quantity", types.Decimal{})
			if err != nil {
				return err
			}
		}
		err = itemJnlLine.TestField("item_shpt_entry_no", 0)
		if err != nil {
			return err
		}
	}

	if itemJnlLine.Qty_per_unit_of_measure.IsZero() {
		itemJnlLine.Qty_per_unit_of_measure = types.NewDecimalFromInt(1)
	}
	itemJnlLine.Quantity = itemJnlLine.Quantity_base
	itemJnlLine.Invoiced_quantity = itemJnlLine.Invoiced_qty_base
	itemJnlLine.Unit_amount = system.Round(itemJnlLine.Unit_amount.Div(itemJnlLine.Qty_per_unit_of_measure), unitPrecision)
	itemJnlLine.Unit_cost = system.Round(itemJnlLine.Unit_cost.Div(itemJnlLine.Qty_per_unit_of_measure), unitPrecision)

	p.itemLedgEntry = gtables.NewItemLedgerEntry(p.src)
	itemLedgEntry := p.itemLedgEntry
	p.initItemLedgEntry(itemLedgEntry)

	itemLedgEntry.Remaining_quantity = itemLedgEntry.Quantity
	itemLedgEntry.Open = !itemLedgEntry.Remaining_quantity.IsZero()
	itemLedgEntry.Positive = itemLedgEntry.Remaining_quantity.IsPositive()
	if !itemLedgEntry.Invoiced_quantity.IsZero() {
		itemLedgEntry.Unit_amount = system.Round(itemLedgEntry.Amount.Div(itemLedgEntry.Invoiced_quantity), unitPrecision)
	}

	if itemLedgEntry.Entry_type == gtables.ItemLedgerEntry_Entry_type.Transfer || p.item.Inventory_value_zero {
		itemLedgEntry.Adjusted_cost_qty = types.Decimal{}
		itemLedgEntry.Adjusted_cost_invoiced_qty = types.Decimal{}
	} else {
		if p.item.Costing_method == gtables.Item_Costing_method.Average &&
			itemLedgEntry.Invoiced_quantity.IsNegative() &&
			!itemJnlLine.Drop_shipment {
			itemLedgEntry.Adjusted_cost_qty = system.Round(itemLedgEntry.Quantity.Mul(p.item.Average_cost))
			itemLedgEntry.Adjusted_cost_invoiced_qty = system.Round(itemLedgEntry.Invoiced_quantity.Mul(p.item.Average_cost))
			itemLedgEntry.Unit_cost = p.item.Average_cost
			netInvoicedQty := p.itemNetInvoicedQty(itemLedgEntry.Drop_shipment)
			if netInvoicedQty.IsNegative() {
				itemLedgEntry.Invt_qty_not_adjusted = itemLedgEntry.Invoiced_quantity
			} else {
				if netInvoicedQty.LessThan(itemLedgEntry.Invoiced_quantity.Abs()) {
					itemLedgEntry.Invt_qty_not_adjusted = itemLedgEntry.Invoiced_quantity.Add(netInvoicedQty)
				}
			}
			if itemLedgEntry.Invt_qty_not_adjusted.IsZero() {
				itemLedgEntry.Cost_is_adjusted = true
			} else {
				itemLedgEntry.Adjust_invoice_cost = true
			}
		} else {
			itemLedgEntry.Adjusted_cost_qty = system.Round(itemLedgEntry.Quantity.Mul(itemLedgEntry.Unit_cost))
			itemLedgEntry.Adjusted_cost_invoiced_qty = system.Round(itemLedgEntry.Invoiced_quantity.Mul(itemLedgEntry.Unit_cost))
		}
	}

	if itemLedgEntry.Entry_type == gtables.ItemLedgerEntry_Entry_type.Transfer {
		itemLedgEntry.Cost_is_adjusted = false
	} else {
		if itemLedgEntry.Invoiced_quantity.IsPositive() { // Purchase invoice
			itemLedgEntry.Cost_is_adjusted = true
			if p.item.Costing_method == gtables.Item_Costing_method.Average && !itemJnlLine.Drop_shipment {
				netInvoicedQty := p.itemNetInvoicedQty(itemLedgEntry.Drop_shipment)
				if netInvoicedQty.IsNegative() {
					err = p.adjustSalesInv()
					if err != nil {
						return err
					}
				}
			}
		}
	}

	itemLedgEntry.Closed_by_entry_no = itemJnlLine.Item_shpt_entry_no

	if itemLedgEntry.Entry_type == gtables.ItemLedgerEntry_Entry_type.Transfer {
		itemLedgEntry.Unit_cost = types.Decimal{}
		itemLedgEntry.Unit_amount = types.Decimal{}
	}

	err = p.applyItemLedgEntry(itemLedgEntry, false)
	if err != nil {
		return err
	}

	if itemLedgEntry.Entry_type == gtables.ItemLedgerEntry_Entry_type.Transfer && !itemLedgEntry.Quantity.IsZero() {
		itemLedgEntry.Unit_cost = system.Round(itemLedgEntry.Unit_cost.Div(itemLedgEntry.Quantity), unitPrecision)
		itemLedgEntry.Unit_amount = system.Round(itemLedgEntry.Amount.Div(itemLedgEntry.Quantity), unitPrecision)
	}

	if !itemJnlLine.Phys_inventory || !itemJnlLine.Quantity.IsZero() {
		err = p.insertItemLedgEntry(itemLedgEntry)
		if err != nil {
			return err
		}
		if itemLedgEntry.Positive {
			err = p.insertApplicationEntry(itemLedgEntry.Entry_no, itemLedgEntry.Entry_no, 0, 0, itemLedgEntry.Entry_type,
				itemLedgEntry.Posting_date, itemLedgEntry.Document_no, itemLedgEntry.Quantity)
			if err != nil {
				return err
			}
		}
		err = p.updateUnitCost()
		if err != nil {
			return err
		}
	}

	if itemJnlLine.Phys_inventory {
		err = p.insertPhysInventoryEntry()
		if err != nil {
			return err
		}
	}

	if !itemJnlLine.Quantity.IsZero() && itemJnlLine.Invoiced_quantity.IsZero() {
		itemJnlLine.Item_shpt_entry_no = itemLedgEntry.Entry_no // Entry no. is returned to shipment/receipt
	}
	return nil
}

// itemNetInvoicedQty is the item's Net Invoiced Qty. for drop shipments or not
func (p *ItemJnlPostLine) itemNetInvoicedQty(dropShipment bool) types.Decimal {
	err := p.item.SetFlowFilter("drop_shipment_filter", system.Format(dropShipment))
	if err != nil {
		return types.Decimal{}
	}
	p.item.CalcFields("net_invoiced_qty")
	_ = p.item.SetFlowFilter("drop_shipment_filter", "")
	return p.item.Net_invoiced_qty
}

// updateUnitCost: an increase of the invoiced inventory updates the item's last direct cost
// (purchases and positive adjustments) and average cost
func (p *ItemJnlPostLine) updateUnitCost() error {
	itemLedgEntry := p.itemLedgEntry
	if itemLedgEntry.Invoiced_quantity.IsPositive() { // Any increase in Net Invoiced Quantity
		p.item.LockTable()
		err := p.item.GetOrError(p.item.No)
		if err != nil {
			return err
		}

		if (itemLedgEntry.Entry_type == gtables.ItemLedgerEntry_Entry_type.Purchase || itemLedgEntry.Entry_type == gtables.ItemLedgerEntry_Entry_type.PositiveAdjmt) &&
			itemLedgEntry.Amount.Add(itemLedgEntry.Discount_amount).IsPositive() {
			p.item.Last_direct_cost = system.Round(itemLedgEntry.Amount.Add(itemLedgEntry.Discount_amount).Div(itemLedgEntry.Invoiced_quantity), unitPrecision)
		}

		if !p.itemJnlLine.Drop_shipment {
			var averageCost types.Decimal
			netInvoicedQty := p.itemNetInvoicedQty(itemLedgEntry.Drop_shipment)
			if netInvoicedQty.GreaterThan(itemLedgEntry.Invoiced_quantity) {
				averageCost = netInvoicedQty.Sub(itemLedgEntry.Invoiced_quantity).Mul(p.item.Average_cost) // Inventory valuation for this purchase
				averageCost = averageCost.Add(itemLedgEntry.Adjusted_cost_invoiced_qty)                    // Purchase value
				averageCost = averageCost.Div(netInvoicedQty)
			} else {
				averageCost = itemLedgEntry.Unit_cost
			}
			err = p.item.Validate("average_cost", system.Round(averageCost, unitPrecision))
			if err != nil {
				return err
			}
		}

		return p.item.ModifyOrError(false)
	}
	return nil
}

// applyItemLedgEntry applies an entry to the open entries of the other sign — the Applies-to
// Entry, else the item's entries in posting date order (FIFO, LIFO from the last) at the same
// location and variant — and writes the application entries; a transfer moves the quantity it
// takes to the new location (InsertTransferEntry)
func (p *ItemJnlPostLine) applyItemLedgEntry(itemLedgEntry *gtables.ItemLedgerEntry, causedByTransfer bool) error {
	if itemLedgEntry.Remaining_quantity.IsZero() ||
		itemLedgEntry.Drop_shipment && itemLedgEntry.Applies_to_entry == 0 {
		return nil
	}

	oldItemLedgEntry := gtables.NewItemLedgerEntry(p.src)
	itemLedgEntry6 := gtables.NewItemLedgerEntry(p.src)
	entryFindMethod := "-"
	firstApplication := true
	for {
		var appliedQty types.Decimal
		if itemLedgEntry.Applies_to_entry != 0 {
			if firstApplication {
				firstApplication = false
				err := oldItemLedgEntry.GetOrError(itemLedgEntry.Applies_to_entry)
				if err != nil {
					return err
				}
				err = oldItemLedgEntry.TestField("item_no", itemLedgEntry.Item_no)
				if err != nil {
					return err
				}
				err = oldItemLedgEntry.TestField("variant_code", itemLedgEntry.Variant_code)
				if err != nil {
					return err
				}
				err = oldItemLedgEntry.TestField("open", true)
				if err != nil {
					return err
				}
				err = oldItemLedgEntry.TestField("positive", !itemLedgEntry.Positive)
				if err != nil {
					return err
				}
				err = oldItemLedgEntry.TestField("location_code", itemLedgEntry.Location_code)
				if err != nil {
					return err
				}
				if oldItemLedgEntry.Remaining_quantity.Add(oldItemLedgEntry.Reserved_quantity).Abs().LessThan(
					itemLedgEntry.Remaining_quantity.Add(itemLedgEntry.Reserved_quantity).Abs()) {
					return apperrors.FieldTooLow(itemLedgEntryTable, "remaining_quantity")
				}
			} else {
				return nil
			}
		} else {
			if firstApplication {
				firstApplication = false
				itemLedgEntry6.SetCurrentKey("item_no", "variant_code", "open", "positive", "location_code", "posting_date")
				itemLedgEntry6.SetRange("item_no", itemLedgEntry.Item_no)
				itemLedgEntry6.SetRange("variant_code", itemLedgEntry.Variant_code)
				itemLedgEntry6.SetRange("open", true)
				itemLedgEntry6.SetRange("positive", !itemLedgEntry.Positive)
				itemLedgEntry6.SetRange("location_code", itemLedgEntry.Location_code)
				if p.item.Costing_method == gtables.Item_Costing_method.Specific {
					return apperrors.ItemSpecificNotYet(gtables.ItemTableName, "costing_method", p.item.Costing_method)
				}
				if p.item.Costing_method == gtables.Item_Costing_method.LIFO {
					entryFindMethod = "+"
				} else {
					entryFindMethod = "-"
				}
				if !itemLedgEntry6.Find(entryFindMethod) {
					return nil
				}
			} else {
				switch entryFindMethod {
				case "-":
					if !itemLedgEntry6.Next() {
						return nil
					}
				case "+":
					if !itemLedgEntry6.Next(-1) {
						return nil
					}
				}
			}
			oldItemLedgEntry.Copy(itemLedgEntry6)
		}

		if oldItemLedgEntry.Remaining_quantity.Add(oldItemLedgEntry.Reserved_quantity).Abs().GreaterThan(
			itemLedgEntry.Remaining_quantity.Add(itemLedgEntry.Reserved_quantity).Abs()) {
			appliedQty = itemLedgEntry.Remaining_quantity.Add(itemLedgEntry.Reserved_quantity)
		} else {
			appliedQty = oldItemLedgEntry.Remaining_quantity.Neg().Sub(oldItemLedgEntry.Reserved_quantity)
		}

		if !appliedQty.IsZero() {
			oldItemLedgEntry.Remaining_quantity = oldItemLedgEntry.Remaining_quantity.Add(appliedQty)
			oldItemLedgEntry.Open = !oldItemLedgEntry.Remaining_quantity.IsZero()
			var err error
			if itemLedgEntry.Positive {
				err = p.insertApplicationEntry(oldItemLedgEntry.Entry_no, itemLedgEntry.Entry_no, oldItemLedgEntry.Entry_no, 0,
					oldItemLedgEntry.Entry_type, itemLedgEntry.Posting_date, oldItemLedgEntry.Document_no, appliedQty.Neg())
			} else {
				err = p.insertApplicationEntry(itemLedgEntry.Entry_no, oldItemLedgEntry.Entry_no, itemLedgEntry.Entry_no, 0,
					itemLedgEntry.Entry_type, itemLedgEntry.Posting_date, itemLedgEntry.Document_no, appliedQty)
			}
			if err != nil {
				return err
			}
			err = oldItemLedgEntry.ModifyOrError(false)
			if err != nil {
				return err
			}

			if itemLedgEntry.Entry_type == gtables.ItemLedgerEntry_Entry_type.Transfer &&
				appliedQty.IsNegative() &&
				!causedByTransfer {
				err = p.insertTransferEntry(itemLedgEntry, oldItemLedgEntry, appliedQty)
				if err != nil {
					return err
				}
			}

			itemLedgEntry.Remaining_quantity = itemLedgEntry.Remaining_quantity.Sub(appliedQty)
			itemLedgEntry.Open = !itemLedgEntry.Remaining_quantity.IsZero()

			if itemLedgEntry.Entry_type == gtables.ItemLedgerEntry_Entry_type.Transfer && !causedByTransfer {
				itemLedgEntry.Unit_cost = itemLedgEntry.Unit_cost.Add(oldItemLedgEntry.Unit_cost.Mul(appliedQty))
			}

			if itemLedgEntry.Remaining_quantity.Add(itemLedgEntry.Reserved_quantity).IsZero() {
				return nil
			}
		}
	}
}

// adjustSalesInv: a purchase invoice of an average cost item adjusts the cost of the sales that
// were invoiced before there was inventory (Invt. Qty. Not Adjusted) to its unit cost
func (p *ItemJnlPostLine) adjustSalesInv() error {
	itemLedgEntry := p.itemLedgEntry
	qtyAvailableForCostAdj := itemLedgEntry.Invoiced_quantity
	salesInvItemEntry := gtables.NewItemLedgerEntry(p.src)
	salesInvItemEntry.SetCurrentKey("cost_is_adjusted", "item_no", "adjust_invoice_cost")
	salesInvItemEntry.SetRange("cost_is_adjusted", false)
	salesInvItemEntry.SetRange("item_no", itemLedgEntry.Item_no)
	salesInvItemEntry.SetRange("adjust_invoice_cost", true)
	for salesInvItemEntry.FindFirst() && !qtyAvailableForCostAdj.IsZero() {
		if qtyAvailableForCostAdj.GreaterThan(salesInvItemEntry.Invt_qty_not_adjusted.Abs()) {
			salesInvItemEntry.Adjusted_cost_invoiced_qty = system.Round(
				salesInvItemEntry.Adjusted_cost_invoiced_qty.Sub(salesInvItemEntry.Invt_qty_not_adjusted.Mul(salesInvItemEntry.Unit_cost)).Add(
					salesInvItemEntry.Invt_qty_not_adjusted.Mul(itemLedgEntry.Unit_cost)))
			qtyAvailableForCostAdj = qtyAvailableForCostAdj.Add(salesInvItemEntry.Invt_qty_not_adjusted)
			salesInvItemEntry.Invt_qty_not_adjusted = types.Decimal{}
		} else {
			salesInvItemEntry.Adjusted_cost_invoiced_qty = system.Round(
				salesInvItemEntry.Adjusted_cost_invoiced_qty.Add(qtyAvailableForCostAdj.Mul(salesInvItemEntry.Unit_cost)).Sub(
					qtyAvailableForCostAdj.Mul(itemLedgEntry.Unit_cost)))
			salesInvItemEntry.Invt_qty_not_adjusted = salesInvItemEntry.Invt_qty_not_adjusted.Add(qtyAvailableForCostAdj)
			qtyAvailableForCostAdj = types.Decimal{}
		}
		if salesInvItemEntry.Invt_qty_not_adjusted.IsZero() {
			salesInvItemEntry.Adjust_invoice_cost = false
			salesInvItemEntry.Cost_is_adjusted = true
		}
		if salesInvItemEntry.Invoiced_quantity.Equal(salesInvItemEntry.Quantity) {
			salesInvItemEntry.Adjusted_cost_qty = salesInvItemEntry.Adjusted_cost_invoiced_qty
		}

		if p.invtSetup.Automatic_cost_posting &&
			!salesInvItemEntry.Cost_posted_to_gl.Equal(salesInvItemEntry.Adjusted_cost_invoiced_qty) {
			err := p.postInventoryToGL(salesInvItemEntry)
			if err != nil {
				return err
			}
		}

		err := salesInvItemEntry.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}

// insertTransferEntry writes the entry of a transfer at its new location (and department and
// project) for the quantity taken from an entry, with that entry's cost
func (p *ItemJnlPostLine) insertTransferEntry(itemLedgEntry, oldItemLedgEntry *gtables.ItemLedgerEntry, appliedQty types.Decimal) error {
	itemJnlLine := p.itemJnlLine
	itemLedgEntry4 := gtables.NewItemLedgerEntry(p.src)
	p.initItemLedgEntry(itemLedgEntry4)
	itemLedgEntry4.Applies_to_entry = 0

	itemLedgEntry4.Source_type = oldItemLedgEntry.Source_type
	itemLedgEntry4.Source_no = oldItemLedgEntry.Source_no
	itemLedgEntry4.Inventory_posting_group = oldItemLedgEntry.Inventory_posting_group
	itemLedgEntry4.Source_posting_group = oldItemLedgEntry.Source_posting_group

	itemLedgEntry4.Quantity = appliedQty.Neg()
	itemLedgEntry4.Invoiced_quantity = itemLedgEntry4.Quantity
	itemLedgEntry4.Amount = system.Round(oldItemLedgEntry.Amount.Mul(appliedQty).Div(oldItemLedgEntry.Quantity)).Neg()
	itemLedgEntry4.Discount_amount = system.Round(oldItemLedgEntry.Discount_amount.Mul(appliedQty).Div(oldItemLedgEntry.Quantity)).Neg()

	itemLedgEntry.Amount = itemLedgEntry.Amount.Sub(itemLedgEntry4.Amount)
	itemLedgEntry.Discount_amount = itemLedgEntry.Discount_amount.Sub(itemLedgEntry4.Discount_amount)

	itemLedgEntry4.Salespers_purch_code = oldItemLedgEntry.Salespers_purch_code
	itemLedgEntry4.Remaining_quantity = itemLedgEntry4.Quantity
	itemLedgEntry4.Open = !itemLedgEntry4.Remaining_quantity.IsZero()
	itemLedgEntry4.Positive = itemLedgEntry4.Remaining_quantity.IsPositive()
	itemLedgEntry4.Unit_amount = oldItemLedgEntry.Unit_amount
	itemLedgEntry4.Unit_cost = oldItemLedgEntry.Unit_cost

	itemLedgEntry4.Location_code = itemJnlLine.New_location_code
	itemLedgEntry4.Department_code = itemJnlLine.New_department_code
	itemLedgEntry4.Project_code = itemJnlLine.New_project_code

	itemLedgEntry4.Adjusted_cost_qty = system.Round(itemLedgEntry4.Quantity.Mul(itemLedgEntry4.Unit_cost))
	itemLedgEntry4.Adjusted_cost_invoiced_qty = system.Round(itemLedgEntry4.Invoiced_quantity.Mul(itemLedgEntry4.Unit_cost))

	itemLedgEntry.Adjusted_cost_qty = itemLedgEntry.Adjusted_cost_qty.Sub(itemLedgEntry4.Adjusted_cost_qty)
	itemLedgEntry.Adjusted_cost_invoiced_qty = itemLedgEntry.Adjusted_cost_invoiced_qty.Sub(itemLedgEntry4.Adjusted_cost_invoiced_qty)

	err := p.insertApplicationEntry(itemLedgEntry4.Entry_no, itemLedgEntry4.Entry_no, itemLedgEntry.Entry_no,
		oldItemLedgEntry.Entry_no, itemLedgEntry4.Entry_type, itemLedgEntry4.Posting_date,
		itemLedgEntry4.Document_no, itemLedgEntry4.Quantity)
	if err != nil {
		return err
	}

	err = p.applyItemLedgEntry(itemLedgEntry4, true)
	if err != nil {
		return err
	}

	return p.insertItemLedgEntry(itemLedgEntry4)
}

// initItemLedgEntry starts an item ledger entry with the next entry number and the line's
// values; a sale, negative adjustment or transfer takes the quantity out (negative)
func (p *ItemJnlPostLine) initItemLedgEntry(itemLedgEntry2 *gtables.ItemLedgerEntry) {
	p.entryNo = p.entryNo + 1

	itemJnlLine := p.itemJnlLine
	itemLedgEntry2.Init()
	itemLedgEntry2.Entry_no = p.entryNo
	itemLedgEntry2.Item_no = itemJnlLine.Item_no
	itemLedgEntry2.Posting_date = itemJnlLine.Posting_date
	itemLedgEntry2.Document_date = itemJnlLine.Document_date
	itemLedgEntry2.Entry_type = gtables.ItemLedgerEntryEntry_type(itemJnlLine.Entry_type)
	itemLedgEntry2.Source_no = itemJnlLine.Source_no
	itemLedgEntry2.Document_no = itemJnlLine.Document_no
	itemLedgEntry2.External_document_no = itemJnlLine.External_document_no
	itemLedgEntry2.Description = itemJnlLine.Description
	itemLedgEntry2.Location_code = itemJnlLine.Location_code
	itemLedgEntry2.Transaction_type = itemJnlLine.Transaction_type
	itemLedgEntry2.Transport_method = itemJnlLine.Transport_method
	itemLedgEntry2.Country_code = itemJnlLine.Country_code
	itemLedgEntry2.Entry_exit_point = itemJnlLine.Entry_exit_point
	itemLedgEntry2.Area = itemJnlLine.Area
	itemLedgEntry2.Transaction_specification = itemJnlLine.Transaction_specification
	itemLedgEntry2.Inventory_posting_group = itemJnlLine.Inventory_posting_group
	itemLedgEntry2.Source_posting_group = itemJnlLine.Source_posting_group
	itemLedgEntry2.Unit_cost = itemJnlLine.Unit_cost
	itemLedgEntry2.Amount = itemJnlLine.Amount
	itemLedgEntry2.Discount_amount = itemJnlLine.Discount_amount
	itemLedgEntry2.Salespers_purch_code = itemJnlLine.Salespers_purch_code
	itemLedgEntry2.Source_code = itemJnlLine.Source_code
	itemLedgEntry2.Applies_to_entry = itemJnlLine.Applies_to_entry
	itemLedgEntry2.Department_code = itemJnlLine.Department_code
	itemLedgEntry2.Project_code = itemJnlLine.Project_code
	itemLedgEntry2.Source_type = gtables.ItemLedgerEntrySource_type(itemJnlLine.Source_type)
	itemLedgEntry2.Journal_batch_name = itemJnlLine.Journal_batch_name
	itemLedgEntry2.Reason_code = itemJnlLine.Reason_code
	itemLedgEntry2.Drop_shipment = itemJnlLine.Drop_shipment
	itemLedgEntry2.Gen_bus_posting_group = itemJnlLine.Gen_bus_posting_group
	itemLedgEntry2.Gen_prod_posting_group = itemJnlLine.Gen_prod_posting_group
	itemLedgEntry2.User_id = types.Code(system.UserId())
	itemLedgEntry2.No_series = itemJnlLine.Posting_no_series
	if itemLedgEntry2.Description == p.item.Description {
		itemLedgEntry2.Description = ""
	}
	itemLedgEntry2.Variant_code = itemJnlLine.Variant_code
	itemLedgEntry2.Unit_of_measure_code = itemJnlLine.Unit_of_measure_code
	itemLedgEntry2.Qty_per_unit_of_measure = itemJnlLine.Qty_per_unit_of_measure

	if itemJnlLine.Entry_type == gtables.ItemJournalLine_Entry_type.Sale ||
		itemJnlLine.Entry_type == gtables.ItemJournalLine_Entry_type.NegativeAdjmt ||
		itemJnlLine.Entry_type == gtables.ItemJournalLine_Entry_type.Transfer {
		itemLedgEntry2.Quantity = itemJnlLine.Quantity.Neg()
		itemLedgEntry2.Invoiced_quantity = itemJnlLine.Invoiced_quantity.Neg()
		if itemJnlLine.Entry_type == gtables.ItemJournalLine_Entry_type.Transfer {
			itemLedgEntry2.Amount = types.Decimal{}
			itemLedgEntry2.Discount_amount = types.Decimal{}
		} else {
			itemLedgEntry2.Amount = itemLedgEntry2.Amount.Neg()
			itemLedgEntry2.Discount_amount = itemLedgEntry2.Discount_amount.Neg()
		}
	} else {
		itemLedgEntry2.Quantity = itemJnlLine.Quantity
		itemLedgEntry2.Invoiced_quantity = itemJnlLine.Invoiced_quantity
	}
}

// insertItemLedgEntry writes an item ledger entry — refused when a negative adjustment of an item
// source or a transfer takes more than is on inventory, or a Reserve Always item goes negative —
// its inventory value to the G/L (Automatic Cost Posting) and the Item Register
func (p *ItemJnlPostLine) insertItemLedgEntry(itemLedgEntry3 *gtables.ItemLedgerEntry) error {
	if itemLedgEntry3.Open {
		if itemLedgEntry3.Entry_type == gtables.ItemLedgerEntry_Entry_type.NegativeAdjmt &&
			itemLedgEntry3.Source_type == gtables.ItemLedgerEntry_Source_type.Item ||
			itemLedgEntry3.Entry_type == gtables.ItemLedgerEntry_Entry_type.Transfer &&
				itemLedgEntry3.Quantity.IsNegative() {
			return apperrors.ItemNotOnInventory(itemLedgEntry3.Item_no.String())
		}
		if p.item.Reserve == gtables.Item_Reserve.Always && itemLedgEntry3.Quantity.IsNegative() {
			return apperrors.ItemNotOnInventory(itemLedgEntry3.Item_no.String())
		}
	}
	if p.invtSetup.Automatic_cost_posting &&
		!itemLedgEntry3.Cost_posted_to_gl.Equal(itemLedgEntry3.Adjusted_cost_invoiced_qty) {
		err := p.postInventoryToGL(itemLedgEntry3)
		if err != nil {
			return err
		}
	}

	err := itemLedgEntry3.InsertOrError(false)
	if err != nil {
		return err
	}

	return p.insertItemReg(itemLedgEntry3.Entry_no, 0)
}

// insertItemReg: the posting's Item Register — the first entry starts it (the next register
// number), later entries widen its entry ranges
func (p *ItemJnlPostLine) insertItemReg(itemLedgEntryNo, physInvtEntryNo int) error {
	itemJnlLine := p.itemJnlLine
	if p.itemReg == nil || p.itemReg.No == 0 {
		p.itemReg = gtables.NewItemRegister(p.src)
		p.itemReg.LockTable()
		no := 1
		if p.itemReg.FindLast() {
			no = p.itemReg.No + 1
		}
		p.itemReg.Init()
		p.itemReg.No = no
		p.itemReg.From_entry_no = itemLedgEntryNo
		p.itemReg.To_entry_no = itemLedgEntryNo
		p.itemReg.From_phys_inventory_entry_no = physInvtEntryNo
		p.itemReg.To_phys_inventory_entry_no = physInvtEntryNo
		p.itemReg.Creation_date = system.Today()
		p.itemReg.Source_code = itemJnlLine.Source_code
		p.itemReg.Journal_batch_name = itemJnlLine.Journal_batch_name
		p.itemReg.User_id = types.Code(system.UserId())
		return p.itemReg.InsertOrError(false)
	}
	if itemLedgEntryNo < p.itemReg.From_entry_no && itemLedgEntryNo != 0 ||
		p.itemReg.From_entry_no == 0 && itemLedgEntryNo > 0 {
		p.itemReg.From_entry_no = itemLedgEntryNo
	}
	if itemLedgEntryNo > p.itemReg.To_entry_no {
		p.itemReg.To_entry_no = itemLedgEntryNo
	}

	if physInvtEntryNo < p.itemReg.From_phys_inventory_entry_no && physInvtEntryNo != 0 ||
		p.itemReg.From_phys_inventory_entry_no == 0 && physInvtEntryNo > 0 {
		p.itemReg.From_phys_inventory_entry_no = physInvtEntryNo
	}
	if physInvtEntryNo > p.itemReg.To_phys_inventory_entry_no {
		p.itemReg.To_phys_inventory_entry_no = physInvtEntryNo
	}
	return p.itemReg.ModifyOrError(false)
}

// postInventoryToGL posts the change of an entry's inventory value (its adjusted cost not yet
// posted) to the inventory account of its posting group, against the COGS account (sales) or
// the inventory adjustment account of the General Posting Setup
func (p *ItemJnlPostLine) postInventoryToGL(itemLedgEntry *gtables.ItemLedgerEntry) error {
	itemJnlLine := p.itemJnlLine
	genJnlLine := gtables.NewGenJournalLine(p.src)
	genJnlLine.Init()
	genJnlLine.Posting_date = itemJnlLine.Posting_date
	genJnlLine.Document_date = itemJnlLine.Document_date
	genJnlLine.Document_no = itemJnlLine.Document_no
	genJnlLine.External_document_no = itemJnlLine.External_document_no
	genJnlLine.Source_code = p.sourceCodeSetup.Inventory_post_cost
	genJnlLine.System_created_entry = true
	invPostingGr := gtables.NewInventoryPostingGroup(p.src)
	err := invPostingGr.GetOrError(itemLedgEntry.Inventory_posting_group)
	if err != nil {
		return err
	}
	err = invPostingGr.TestField("inventory_account")
	if err != nil {
		return err
	}
	genJnlLine.Account_no = invPostingGr.Inventory_account
	genPostingSetup2 := p.genPostingSetup
	if p.genPostingSetup.Gen_bus_posting_group != itemLedgEntry.Gen_bus_posting_group ||
		p.genPostingSetup.Gen_prod_posting_group != itemLedgEntry.Gen_prod_posting_group {
		genPostingSetup2 = gtables.NewGeneralPostingSetup(p.src)
		err = genPostingSetup2.GetOrError(map[string]interface{}{"gen_bus_posting_group": itemLedgEntry.Gen_bus_posting_group, "gen_prod_posting_group": itemLedgEntry.Gen_prod_posting_group})
		if err != nil {
			return err
		}
	}
	genJnlLine.Description = itemJnlLine.Description
	genJnlLine.Department_code = itemLedgEntry.Department_code
	genJnlLine.Project_code = itemLedgEntry.Project_code
	err = genJnlLine.Validate("amount", itemLedgEntry.Adjusted_cost_invoiced_qty.Sub(itemLedgEntry.Cost_posted_to_gl))
	if err != nil {
		return err
	}
	if p.genJnlPostLine == nil {
		p.genJnlPostLine = genjnl.NewGenJnlPostLine(p.src)
	}
	err = p.genJnlPostLine.Run(genJnlLine)
	if err != nil {
		return err
	}

	if itemLedgEntry.Entry_type == gtables.ItemLedgerEntry_Entry_type.Sale {
		err = genPostingSetup2.TestField("cogs_account")
		if err != nil {
			return err
		}
		genJnlLine.Account_no = genPostingSetup2.Cogs_account
	} else {
		err = genPostingSetup2.TestField("inventory_adjmt_account")
		if err != nil {
			return err
		}
		genJnlLine.Account_no = genPostingSetup2.Inventory_adjmt_account
	}
	err = genJnlLine.Validate("amount", genJnlLine.Amount.Neg())
	if err != nil {
		return err
	}
	err = p.genJnlPostLine.Run(genJnlLine)
	if err != nil {
		return err
	}

	itemLedgEntry.Cost_posted_to_gl = itemLedgEntry.Adjusted_cost_invoiced_qty
	return nil
}

// insertApplicationEntry writes an item application entry: the quantity of an item ledger
// entry applied to (from) another
func (p *ItemJnlPostLine) insertApplicationEntry(itemLedgEntryNo, appliedToEntryNo, appliedFromEntryNo, transferedFromEntryNo int,
	entryType gtables.ItemLedgerEntryEntry_type, postingDate types.Date, documentNo types.Code, quantity types.Decimal) error {
	itemApplicationEntry := gtables.NewItemApplicationEntry(p.src)
	if p.applicationEntryNo == 0 {
		itemApplicationEntry.LockTable()
		if itemApplicationEntry.FindLast() {
			p.applicationEntryNo = itemApplicationEntry.Entry_no
		}
	}

	itemApplicationEntry.Init()
	p.applicationEntryNo = p.applicationEntryNo + 1
	itemApplicationEntry.Entry_no = p.applicationEntryNo
	itemApplicationEntry.Item_no = p.itemJnlLine.Item_no
	itemApplicationEntry.Item_ledger_entry_no = itemLedgEntryNo
	itemApplicationEntry.Applied_to_entry_no = appliedToEntryNo
	itemApplicationEntry.Applied_from_entry_no = appliedFromEntryNo
	itemApplicationEntry.Transfered_from_entry_no = transferedFromEntryNo
	if itemLedgEntryNo != appliedToEntryNo {
		itemApplicationEntry.Fixed_applied_to_entry_no = appliedToEntryNo
		itemApplicationEntry.Fixed_application = true
	}
	itemApplicationEntry.Quantity = quantity
	itemApplicationEntry.Entry_type = gtables.ItemApplicationEntryEntry_type(entryType)
	itemApplicationEntry.Posting_date = postingDate
	itemApplicationEntry.Document_no = documentNo
	itemApplicationEntry.Valid = true
	return itemApplicationEntry.InsertOrError(false)
}

// insertPhysInventoryEntry writes the phys. inventory entry of a counted line (the calculated
// and counted quantities) and widens the Item Register's phys. inventory range
func (p *ItemJnlPostLine) insertPhysInventoryEntry() error {
	itemJnlLine := p.itemJnlLine
	physInvtLedgEntry := gtables.NewPhysInventoryLedgerEntry(p.src)
	if p.physInvtEntryNo == 0 {
		physInvtLedgEntry.LockTable()
		if physInvtLedgEntry.FindLast() {
			p.physInvtEntryNo = physInvtLedgEntry.Entry_no
		}
	}

	p.physInvtEntryNo = p.physInvtEntryNo + 1

	physInvtLedgEntry.Init()
	physInvtLedgEntry.Entry_no = p.physInvtEntryNo
	physInvtLedgEntry.Item_no = itemJnlLine.Item_no
	physInvtLedgEntry.Posting_date = itemJnlLine.Posting_date
	physInvtLedgEntry.Document_date = itemJnlLine.Document_date
	physInvtLedgEntry.Entry_type = gtables.PhysInventoryLedgerEntryEntry_type(itemJnlLine.Entry_type)
	physInvtLedgEntry.Document_no = itemJnlLine.Document_no
	physInvtLedgEntry.External_document_no = itemJnlLine.External_document_no
	physInvtLedgEntry.Description = itemJnlLine.Description
	physInvtLedgEntry.Location_code = itemJnlLine.Location_code
	physInvtLedgEntry.Inventory_posting_group = itemJnlLine.Inventory_posting_group
	physInvtLedgEntry.Unit_cost = itemJnlLine.Unit_cost
	physInvtLedgEntry.Amount = itemJnlLine.Amount
	physInvtLedgEntry.Salespers_purch_code = itemJnlLine.Salespers_purch_code
	physInvtLedgEntry.Source_code = itemJnlLine.Source_code
	physInvtLedgEntry.Department_code = itemJnlLine.Department_code
	physInvtLedgEntry.Project_code = itemJnlLine.Project_code
	physInvtLedgEntry.Journal_batch_name = itemJnlLine.Journal_batch_name
	physInvtLedgEntry.Reason_code = itemJnlLine.Reason_code
	physInvtLedgEntry.User_id = types.Code(system.UserId())
	physInvtLedgEntry.No_series = itemJnlLine.Posting_no_series
	if physInvtLedgEntry.Description == p.item.Description {
		physInvtLedgEntry.Description = ""
	}
	physInvtLedgEntry.Variant_code = itemJnlLine.Variant_code
	physInvtLedgEntry.Unit_of_measure_code = itemJnlLine.Unit_of_measure_code

	physInvtLedgEntry.Quantity = itemJnlLine.Quantity
	physInvtLedgEntry.Unit_amount = itemJnlLine.Unit_amount
	physInvtLedgEntry.Qty_calculated = itemJnlLine.Qty_calculated
	physInvtLedgEntry.Qty_phys_inventory = itemJnlLine.Qty_phys_inventory
	physInvtLedgEntry.Last_item_ledger_entry_no = itemJnlLine.Last_item_ledger_entry_no

	err := physInvtLedgEntry.InsertOrError(false)
	if err != nil {
		return err
	}

	return p.insertItemReg(0, physInvtLedgEntry.Entry_no)
}

// AutomaticCostPosting reports whether the posting puts the inventory value in the G/L (the
// Inventory Setup's Automatic Cost Posting, read with the first posted line)
func (p *ItemJnlPostLine) AutomaticCostPosting() bool {
	return p.invtSetup != nil && p.invtSetup.Automatic_cost_posting
}

// PostInventoryToGL posts the part of an item ledger entry's adjusted cost not yet in the G/L
// (an entry whose cost changed after it was posted, the BOM journal's assembled item); the
// entry's Cost Posted to G/L follows, the caller modifies it
func (p *ItemJnlPostLine) PostInventoryToGL(itemLedgEntry *gtables.ItemLedgerEntry) error {
	return p.postInventoryToGL(itemLedgEntry)
}

// AdjustSalesInv adjusts the cost of the sales invoiced before an inbound entry of an Average
// item (the BOM journal's assembled item, whose cost is set after codeunit 22 posted it)
func (p *ItemJnlPostLine) AdjustSalesInv(itemLedgEntry *gtables.ItemLedgerEntry) error {
	p.itemLedgEntry = itemLedgEntry
	return p.adjustSalesInv()
}
