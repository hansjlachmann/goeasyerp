package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 795 Adjust Cost - Item Entries: the item ledger entries whose cost is not adjusted get
// their actual cost. Inbound entries take the cost of their invoices (a transfer the cost of
// the entry it came from); outbound entries take the cost of the inbound entries they were
// applied to (FIFO, LIFO, Standard) or, for an Average item, the cost of their invoices; an
// assembled item (BOM) takes the change of its components' cost. The G/L follows with report
// 1002 Post Inventory Cost to G/L. The additional-currency costs (Add.-Currency Adj. Cost (Qty.),
// Add.-Curr. Adj. Cost (Inv Qty), the item's Add.-Curr. Average Cost) follow the same way.
const AdjustCostItemEntriesID = 795

func init() {
	Register(AdjustCostItemEntriesID, "adjust-cost-item-entries", func() Report { return &adjustCostItemEntries{} })
}

type adjustCostItemEntries struct {
	run       *Run
	item      *gtables.Item
	startDate types.Date
	endDate   types.Date
	window    *system.Window

	bomLedgerEntry              *gtables.BOMLedgerEntry
	oldAdjCostQty               types.Decimal
	oldAdjCostInvQty            types.Decimal
	oldAddCurrencyAdjCostQty    types.Decimal
	oldAddCurrencyAdjCostInvQty types.Decimal
	currency                    *gtables.Currency // the additional reporting currency's rounding
}

func (r *adjustCostItemEntries) Definition() *Definition {
	return &Definition{
		Options: []Option{
			{Name: "starting_date", Type: "date"},
			{Name: "ending_date", Type: "date"},
		},
		OnPreReport: r.onPreReport,
		Transaction: true,
	}
}

// onPreReport is the classic report's one Integer record (MaxIteration=1): it updates the
// receipts, then the shipments
func (r *adjustCostItemEntries) onPreReport(run *Run) error {
	r.run = run
	r.startDate = run.OptionDate("starting_date")
	r.endDate = run.OptionDate("ending_date")
	r.item = gtables.NewItem(run)
	r.bomLedgerEntry = gtables.NewBOMLedgerEntry(run)

	itemLedgEntry := gtables.NewItemLedgerEntry(run)
	itemLedgEntry.LockTable()
	lastItemLedgEntryNo := 0
	if itemLedgEntry.FindLast() {
		lastItemLedgEntryNo = itemLedgEntry.Entry_no
	}
	r.window = system.OpenWindow(run.Text("WINDOW_INCREASE"))
	r.window.Update(2, lastItemLedgEntryNo)

	glSetup := gtables.GetGeneralLedgerSetup(run)
	r.currency = gtables.NewCurrency(run)
	if glSetup.Additional_reporting_currency != "" {
		err := r.currency.GetOrError(glSetup.Additional_reporting_currency)
		if err != nil {
			return err
		}
		err = r.currency.TestField("amount_rounding_precision")
		if err != nil {
			return err
		}
	} else {
		tables.InitRoundingPrecision(r.currency)
	}
	err := r.updateReceipts()
	if err != nil {
		return err
	}
	r.window.Close()
	r.window = system.OpenWindow(run.Text("WINDOW_DECREASE"))
	r.window.Update(2, lastItemLedgEntryNo)
	err = r.updateShipments()
	r.window.Close()
	return err
}

// updateReceipts: the inbound entries not adjusted take the cost of their invoices, a transfer
// the cost of the entry it was transferred from
func (r *adjustCostItemEntries) updateReceipts() error {
	run := r.run
	purchRcptItemEntry := gtables.NewItemLedgerEntry(run)
	purchRcptItemEntry.SetCurrentKey("cost_is_adjusted")
	purchRcptItemEntry.SetRange("cost_is_adjusted", false)
	purchRcptItemEntry.SetFilter("quantity", ">0")
	for found := purchRcptItemEntry.FindSetBuffered(); found; found = purchRcptItemEntry.Next() {
		r.window.Update(1, purchRcptItemEntry.Entry_no)
		purchReceiptEntry2 := gtables.NewItemLedgerEntry(run)
		purchReceiptEntry2.Copy(purchRcptItemEntry)
		if purchRcptItemEntry.Entry_type != gtables.ItemLedgerEntry_Entry_type.Transfer {
			// Receipt invoices
			purchCostTracedBack := purchRcptItemEntry.Adjusted_cost_invoiced_qty
			purchCostTracedBackAddCurr := purchRcptItemEntry.Add_curr_adj_cost_inv_qty
			purchQtyTracedBack := purchRcptItemEntry.Invoiced_quantity
			if !purchRcptItemEntry.Quantity.Equal(purchRcptItemEntry.Invoiced_quantity) {
				purchInvItemEntry := gtables.NewItemLedgerEntry(run)
				purchInvItemEntry.SetCurrentKey("closed_by_entry_no")
				purchInvItemEntry.SetRange("closed_by_entry_no", purchRcptItemEntry.Entry_no)
				purchInvItemEntry.SetFilter("invoiced_quantity", ">0")
				for found2 := purchInvItemEntry.FindSetBuffered(); found2; found2 = purchInvItemEntry.Next() {
					purchCostTracedBack = purchCostTracedBack.Add(purchInvItemEntry.Adjusted_cost_invoiced_qty)
					purchCostTracedBackAddCurr = purchCostTracedBackAddCurr.Add(purchInvItemEntry.Add_curr_adj_cost_inv_qty)
					purchQtyTracedBack = purchQtyTracedBack.Add(purchInvItemEntry.Invoiced_quantity)
				}
			}
			purchReceiptEntry2.Adjusted_cost_qty = purchCostTracedBack.Add(
				system.Round(purchReceiptEntry2.Unit_cost.Mul(purchReceiptEntry2.Quantity.Sub(purchQtyTracedBack))))
			purchReceiptEntry2.Add_currency_adj_cost_qty = purchCostTracedBackAddCurr.Add(
				r.roundAmount(purchReceiptEntry2.Additional_currency_unit_cost.Mul(purchReceiptEntry2.Quantity.Sub(purchQtyTracedBack))))
			if purchQtyTracedBack.Equal(purchReceiptEntry2.Quantity) {
				r.setCostIsAdjustedToTRUE(purchReceiptEntry2)
			}
		} else {
			// Receipt transfer
			itemApplicationEntry := gtables.NewItemApplicationEntry(run)
			itemApplicationEntry.SetCurrentKey("item_ledger_entry_no", "applied_to_entry_no", "applied_from_entry_no")
			itemApplicationEntry.SetRange("item_ledger_entry_no", purchRcptItemEntry.Entry_no)
			itemApplicationEntry.SetRange("applied_to_entry_no", purchRcptItemEntry.Entry_no)
			itemApplicationEntry.SetFilter("applied_from_entry_no", "<>0")
			if !itemApplicationEntry.FindFirst() {
				continue
			}
			purchRcptItemEntry2 := gtables.NewItemLedgerEntry(run)
			err := purchRcptItemEntry2.GetOrError(itemApplicationEntry.Transfered_from_entry_no)
			if err != nil {
				return err
			}
			purchReceiptEntry2.Adjusted_cost_qty = system.Round(
				purchRcptItemEntry2.Adjusted_cost_qty.Mul(itemApplicationEntry.Quantity).Div(purchRcptItemEntry2.Quantity))
			purchReceiptEntry2.Add_currency_adj_cost_qty = r.roundAmount(
				purchRcptItemEntry2.Add_currency_adj_cost_qty.Mul(itemApplicationEntry.Quantity).Div(purchRcptItemEntry2.Quantity))
			purchReceiptEntry2.Cost_is_adjusted = purchRcptItemEntry2.Cost_is_adjusted
			purchReceiptEntry2.Adjusted_cost_invoiced_qty = purchReceiptEntry2.Adjusted_cost_qty
			purchReceiptEntry2.Add_curr_adj_cost_inv_qty = purchReceiptEntry2.Add_currency_adj_cost_qty
		}
		err := purchReceiptEntry2.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}

// updateShipments: the outbound entries not adjusted take the cost of the inbound entries
// they were applied to (Average: their invoices' cost) and pass it on to their invoices; a
// component's change goes to the assembled item
func (r *adjustCostItemEntries) updateShipments() error {
	run := r.run
	salesRcptItemEntry := gtables.NewItemLedgerEntry(run)
	salesRcptItemEntry.SetCurrentKey("cost_is_adjusted")
	salesRcptItemEntry.SetRange("cost_is_adjusted", false)
	salesRcptItemEntry.SetFilter("quantity", "<0")
	for found := salesRcptItemEntry.FindSetBuffered(); found; found = salesRcptItemEntry.Next() {
		r.window.Update(1, salesRcptItemEntry.Entry_no)
		itemCostAtAvg := r.getItem(salesRcptItemEntry.Item_no) &&
			r.item.Costing_method == gtables.Item_Costing_method.Average &&
			!salesRcptItemEntry.Drop_shipment &&
			salesRcptItemEntry.Entry_type != gtables.ItemLedgerEntry_Entry_type.Transfer
		inPeriod := !salesRcptItemEntry.Posting_date.Before(r.startDate) &&
			(!salesRcptItemEntry.Posting_date.After(r.endDate) || r.endDate.IsZero())
		if !salesRcptItemEntry.Invoiced_quantity.Equal(salesRcptItemEntry.Quantity) || (inPeriod && !itemCostAtAvg) {
			err := r.adjustShipment(salesRcptItemEntry, itemCostAtAvg)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// adjustShipment adjusts one outbound entry and its invoices (the body of the shipment loop)
func (r *adjustCostItemEntries) adjustShipment(salesRcptItemEntry *gtables.ItemLedgerEntry, itemCostAtAvg bool) error {
	run := r.run
	purchCostTracedBack := types.Decimal{}
	salesCostTracedBack := types.Decimal{}
	purchCostTracedBackAddCurr := types.Decimal{}
	salesCostTracedBackAddCurr := types.Decimal{}
	purchQtyTracedBack := types.Decimal{}
	salesQtyTracedBack := types.Decimal{}
	purchCostIsAdjusted := true
	if !itemCostAtAvg {
		// Entries closed by the shipment
		itemApplicationEntry := gtables.NewItemApplicationEntry(run)
		itemApplicationEntry.SetCurrentKey("item_ledger_entry_no", "applied_to_entry_no", "applied_from_entry_no")
		itemApplicationEntry.SetRange("item_ledger_entry_no", salesRcptItemEntry.Entry_no)
		itemApplicationEntry.SetRange("applied_from_entry_no", salesRcptItemEntry.Entry_no)
		purchRcptItemEntry := gtables.NewItemLedgerEntry(run)
		for found := itemApplicationEntry.FindSetBuffered(); found; found = itemApplicationEntry.Next() {
			err := purchRcptItemEntry.GetOrError(itemApplicationEntry.Applied_to_entry_no)
			if err != nil {
				return err
			}
			purchCostTracedBack = purchCostTracedBack.Add(
				purchRcptItemEntry.Adjusted_cost_qty.Div(purchRcptItemEntry.Quantity).Mul(itemApplicationEntry.Quantity))
			purchCostTracedBackAddCurr = purchCostTracedBackAddCurr.Add(
				purchRcptItemEntry.Add_currency_adj_cost_qty.Div(purchRcptItemEntry.Quantity).Mul(itemApplicationEntry.Quantity))
			purchQtyTracedBack = purchQtyTracedBack.Add(itemApplicationEntry.Quantity)
			if !purchRcptItemEntry.Cost_is_adjusted {
				purchCostIsAdjusted = false
			}
		}
		// Update the shipment
		r.saveOldValues(salesRcptItemEntry)
		salesRcptItemEntry.Adjusted_cost_qty = system.Round(
			purchCostTracedBack.Add(salesRcptItemEntry.Unit_cost.Mul(salesRcptItemEntry.Quantity.Sub(purchQtyTracedBack))))
		salesRcptItemEntry.Add_currency_adj_cost_qty = r.roundAmount(
			purchCostTracedBackAddCurr.Add(salesRcptItemEntry.Additional_currency_unit_cost.Mul(salesRcptItemEntry.Quantity.Sub(purchQtyTracedBack))))
	}

	qtyToDistribute := types.Decimal{}
	costToDistribute := types.Decimal{}
	costToDistributeAddCurr := types.Decimal{}
	if salesRcptItemEntry.Invoiced_quantity.Equal(salesRcptItemEntry.Quantity) {
		salesRcptItemEntry.Adjusted_cost_invoiced_qty = salesRcptItemEntry.Adjusted_cost_qty
		salesRcptItemEntry.Add_curr_adj_cost_inv_qty = salesRcptItemEntry.Add_currency_adj_cost_qty
		salesQtyTracedBack = salesRcptItemEntry.Invoiced_quantity
	} else {
		qtyToDistribute = salesRcptItemEntry.Quantity
		costToDistribute = salesRcptItemEntry.Adjusted_cost_qty
		costToDistributeAddCurr = salesRcptItemEntry.Add_currency_adj_cost_qty
	}
	if !qtyToDistribute.IsZero() {
		// Adjust shipment invoices
		salesInvItemEntry := gtables.NewItemLedgerEntry(run)
		salesInvItemEntry.SetCurrentKey("closed_by_entry_no")
		salesInvItemEntry.SetRange("closed_by_entry_no", salesRcptItemEntry.Entry_no)
		salesInvItemEntry.SetFilter("invoiced_quantity", "<0")
		if r.endDate.IsZero() {
			endOfTime, _ := system.DMY2Date(31, 12, 9999)
			salesInvItemEntry.SetRange("posting_date", r.startDate, endOfTime)
		} else {
			salesInvItemEntry.SetRange("posting_date", r.startDate, r.endDate)
		}
		for found := salesInvItemEntry.FindSetBuffered(); found; found = salesInvItemEntry.Next() {
			if itemCostAtAvg {
				if salesInvItemEntry.Cost_is_adjusted {
					salesCostTracedBack = salesCostTracedBack.Add(salesInvItemEntry.Adjusted_cost_invoiced_qty)
					salesCostTracedBackAddCurr = salesCostTracedBackAddCurr.Add(salesInvItemEntry.Add_curr_adj_cost_inv_qty)
					salesQtyTracedBack = salesQtyTracedBack.Add(salesInvItemEntry.Invoiced_quantity)
				}
			} else {
				salesInvItemEntry.Adjusted_cost_invoiced_qty = system.Round(
					costToDistribute.Div(qtyToDistribute).Mul(salesInvItemEntry.Invoiced_quantity))
				salesInvItemEntry.Add_curr_adj_cost_inv_qty = r.roundAmount(
					costToDistributeAddCurr.Div(qtyToDistribute).Mul(salesInvItemEntry.Invoiced_quantity))
				costToDistribute = costToDistribute.Sub(salesInvItemEntry.Adjusted_cost_invoiced_qty)
				costToDistributeAddCurr = costToDistributeAddCurr.Sub(salesInvItemEntry.Add_curr_adj_cost_inv_qty)
				qtyToDistribute = qtyToDistribute.Sub(salesInvItemEntry.Invoiced_quantity)
				salesQtyTracedBack = salesQtyTracedBack.Add(salesInvItemEntry.Invoiced_quantity)
				if purchQtyTracedBack.Equal(salesRcptItemEntry.Quantity) && purchCostIsAdjusted {
					salesInvItemEntry.Cost_is_adjusted = true
				}
				err := salesInvItemEntry.ModifyOrError(false)
				if err != nil {
					return err
				}
			}
		}
	}

	// Adjust shipment
	salesShipmentEntry2 := gtables.NewItemLedgerEntry(run)
	salesShipmentEntry2.Copy(salesRcptItemEntry)
	if itemCostAtAvg {
		salesShipmentEntry2.Adjusted_cost_qty = system.Round(
			salesCostTracedBack.Add(salesShipmentEntry2.Unit_cost.Mul(salesShipmentEntry2.Quantity.Sub(salesQtyTracedBack))))
		salesShipmentEntry2.Add_currency_adj_cost_qty = r.roundAmount(
			salesCostTracedBackAddCurr.Add(salesShipmentEntry2.Additional_currency_unit_cost.Mul(salesShipmentEntry2.Quantity.Sub(salesQtyTracedBack))))
		if salesQtyTracedBack.Equal(salesShipmentEntry2.Quantity) {
			salesShipmentEntry2.Cost_is_adjusted = true
		}
	} else {
		if purchQtyTracedBack.Equal(salesShipmentEntry2.Quantity) &&
			salesQtyTracedBack.Equal(salesShipmentEntry2.Quantity) &&
			purchCostIsAdjusted {
			salesShipmentEntry2.Cost_is_adjusted = true
		}
	}
	err := salesShipmentEntry2.ModifyOrError(false)
	if err != nil {
		return err
	}
	return r.updateBOMFinishedGood(salesShipmentEntry2)
}

// getItem (GetItem): the item of an entry, kept between entries
func (r *adjustCostItemEntries) getItem(itemNo types.Code) bool {
	if itemNo != "" && itemNo == r.item.No {
		return true
	}
	return r.item.Get(itemNo)
}

// saveOldValues (SaveOldValues): an entry's adjusted costs before it is adjusted
func (r *adjustCostItemEntries) saveOldValues(itemLedgerEntry *gtables.ItemLedgerEntry) {
	r.oldAdjCostQty = itemLedgerEntry.Adjusted_cost_qty
	r.oldAdjCostInvQty = itemLedgerEntry.Adjusted_cost_invoiced_qty
	r.oldAddCurrencyAdjCostQty = itemLedgerEntry.Add_currency_adj_cost_qty
	r.oldAddCurrencyAdjCostInvQty = itemLedgerEntry.Add_curr_adj_cost_inv_qty
}

// roundAmount rounds an additional-currency amount (the currency's Amount Rounding Precision)
func (r *adjustCostItemEntries) roundAmount(amount types.Decimal) types.Decimal {
	return system.Round(amount, r.currency.Amount_rounding_precision)
}

// setCostIsAdjustedToTRUE (SetCostIsAdjustedToTRUE): an assembled item's entry is adjusted only
// when its components are
func (r *adjustCostItemEntries) setCostIsAdjustedToTRUE(itemLedgerEntry *gtables.ItemLedgerEntry) {
	if r.isBOMProduct(itemLedgerEntry) {
		if !r.allBOMComponentsAdjusted() {
			return
		}
	}
	itemLedgerEntry.Cost_is_adjusted = true
}

// updateBOMFinishedGood (UpdateBOMFinishedGood): a component's cost change goes to the entry
// of the item it was assembled into (with the item's indirect cost) and to the item's average
// cost; not for a Standard item or an entry already adjusted
func (r *adjustCostItemEntries) updateBOMFinishedGood(itemLedgerEntry *gtables.ItemLedgerEntry) error {
	if !r.isBOMComponent(itemLedgerEntry) {
		return nil
	}
	// the BOM ledger entry is a component
	bomRegister := gtables.NewBOMRegister(r.run)
	if !r.getBOMRegisterLine(bomRegister) {
		return nil
	}
	bomLedgerEntry2 := gtables.NewBOMLedgerEntry(r.run)
	bomLedgerEntry2.SetRange("entry_no", bomRegister.From_entry_no, r.bomLedgerEntry.Entry_no)
	bomLedgerEntry2.SetRange("entry_type", gtables.BOMLedgerEntry_Entry_type.BOM)
	bomLedgerEntry2.SetRange("reference_type", gtables.BOMLedgerEntry_Reference_type.Item)
	if !bomLedgerEntry2.FindLast() {
		return nil
	}
	// the finished good
	itemLedgerEntry2 := gtables.NewItemLedgerEntry(r.run)
	if !itemLedgerEntry2.Get(bomLedgerEntry2.Reference_entry_no) {
		return nil
	}
	if itemLedgerEntry2.Cost_is_adjusted {
		return nil
	}
	r.item.LockTable()
	if !r.getItem(itemLedgerEntry2.Item_no) {
		return nil
	}
	if r.item.Costing_method == gtables.Item_Costing_method.Standard {
		return nil
	}
	hundred := types.NewDecimalFromInt(100)
	one := types.NewDecimalFromInt(1)
	indirectFactor := one.Add(r.item.Indirect_cost_percent.Div(hundred))
	costQtyChange := r.oldAdjCostQty.Sub(itemLedgerEntry.Adjusted_cost_qty).Mul(indirectFactor)
	costInvQtyChange := r.oldAdjCostInvQty.Sub(itemLedgerEntry.Adjusted_cost_invoiced_qty).Mul(indirectFactor)
	itemLedgerEntry2.Adjusted_cost_qty = system.Round(itemLedgerEntry2.Adjusted_cost_qty.Add(costQtyChange))
	itemLedgerEntry2.Adjusted_cost_invoiced_qty = system.Round(itemLedgerEntry2.Adjusted_cost_invoiced_qty.Add(costInvQtyChange))
	addCurrencyCostQtyChange := r.oldAddCurrencyAdjCostQty.Sub(itemLedgerEntry.Add_currency_adj_cost_qty).Mul(indirectFactor)
	addCurrencyCostInvQtyChange := r.oldAddCurrencyAdjCostInvQty.Sub(itemLedgerEntry.Add_curr_adj_cost_inv_qty).Mul(indirectFactor)
	itemLedgerEntry2.Add_currency_adj_cost_qty = r.roundAmount(itemLedgerEntry2.Add_currency_adj_cost_qty.Add(addCurrencyCostQtyChange))
	itemLedgerEntry2.Add_curr_adj_cost_inv_qty = r.roundAmount(itemLedgerEntry2.Add_curr_adj_cost_inv_qty.Add(addCurrencyCostInvQtyChange))
	err := itemLedgerEntry2.ModifyOrError(false)
	if err != nil {
		return err
	}
	// adjust average cost
	r.item.CalcFields("net_invoiced_qty")
	if r.item.Net_invoiced_qty.GreaterThan(itemLedgerEntry2.Invoiced_quantity) {
		r.item.Average_cost = r.item.Net_invoiced_qty.Mul(r.item.Average_cost)
		r.item.Average_cost = r.item.Average_cost.Add(costInvQtyChange)
		r.item.Average_cost = r.item.Average_cost.Div(r.item.Net_invoiced_qty)

		r.item.Add_curr_average_cost = r.item.Net_invoiced_qty.Mul(r.item.Add_curr_average_cost)
		r.item.Add_curr_average_cost = r.item.Add_curr_average_cost.Add(addCurrencyCostInvQtyChange)
		r.item.Add_curr_average_cost = r.item.Add_curr_average_cost.Div(r.item.Net_invoiced_qty)
	} else {
		r.item.Average_cost = itemLedgerEntry2.Adjusted_cost_invoiced_qty.Div(itemLedgerEntry2.Invoiced_quantity)
		r.item.Add_curr_average_cost = itemLedgerEntry2.Add_curr_adj_cost_inv_qty.Div(itemLedgerEntry2.Invoiced_quantity)
	}
	// the additional-currency average first: the average cost's trigger keeps it (see Item)
	err = r.item.Validate("add_curr_average_cost", system.Round(r.item.Add_curr_average_cost, r.currency.Unit_amount_rounding_precision))
	if err != nil {
		return err
	}
	err = r.item.Validate("average_cost", system.Round(r.item.Average_cost, types.MustDecimal("0.00001")))
	if err != nil {
		return err
	}
	return r.item.ModifyOrError(false)
}

// isBOMProduct (IsBOMProduct): the entry is an assembled item
func (r *adjustCostItemEntries) isBOMProduct(itemLedgerEntry *gtables.ItemLedgerEntry) bool {
	if r.getBOMLedgerEntry(itemLedgerEntry) {
		return r.bomLedgerEntry.Entry_type == gtables.BOMLedgerEntry_Entry_type.BOM
	}
	return false
}

// isBOMComponent (IsBOMComponent): the entry is a component of an assembly
func (r *adjustCostItemEntries) isBOMComponent(itemLedgerEntry *gtables.ItemLedgerEntry) bool {
	if r.getBOMLedgerEntry(itemLedgerEntry) {
		return r.bomLedgerEntry.Entry_type == gtables.BOMLedgerEntry_Entry_type.Component
	}
	return false
}

// getBOMLedgerEntry (GetBOMLedgerEntry): the BOM ledger entry pointing at an item entry
func (r *adjustCostItemEntries) getBOMLedgerEntry(itemLedgerEntry *gtables.ItemLedgerEntry) bool {
	bomLedgerEntry := r.bomLedgerEntry
	if bomLedgerEntry.Reference_type == gtables.BOMLedgerEntry_Reference_type.Item &&
		bomLedgerEntry.Reference_entry_no == itemLedgerEntry.Entry_no && bomLedgerEntry.Entry_no != 0 {
		return true
	}
	bomLedgerEntry.SetCurrentKey("reference_type", "reference_entry_no")
	bomLedgerEntry.SetRange("reference_type", gtables.BOMLedgerEntry_Reference_type.Item)
	bomLedgerEntry.SetRange("reference_entry_no", itemLedgerEntry.Entry_no)
	if bomLedgerEntry.FindFirst() {
		return true
	}
	bomLedgerEntry.Init()
	return false
}

// allBOMComponentsAdjusted (AllBOMComponentsAdjusted): the components of the assembly in the BOM
// ledger entry are all adjusted
func (r *adjustCostItemEntries) allBOMComponentsAdjusted() bool {
	bomRegister := gtables.NewBOMRegister(r.run)
	if !r.getBOMRegisterLine(bomRegister) {
		return true
	}
	bomLedgerEntry2 := gtables.NewBOMLedgerEntry(r.run)
	bomLedgerEntry2.SetRange("entry_no", r.bomLedgerEntry.Entry_no, bomRegister.To_entry_no)
	bomLedgerEntry2.SetRange("entry_type", gtables.BOMLedgerEntry_Entry_type.BOM)
	bomLedgerEntry2.SetRange("reference_type", gtables.BOMLedgerEntry_Reference_type.Item)
	if !bomLedgerEntry2.FindFirst() {
		return true
	}
	firstThisBuild := bomLedgerEntry2.Entry_no + 1
	var lastThisBuild int
	if !bomLedgerEntry2.Next() {
		lastThisBuild = bomRegister.To_entry_no
	} else {
		lastThisBuild = bomLedgerEntry2.Entry_no - 1
	}
	bomLedgerEntry2.SetRange("entry_no", firstThisBuild, lastThisBuild)
	bomLedgerEntry2.SetRange("entry_type", gtables.BOMLedgerEntry_Entry_type.Component)
	itemLedgerEntry2 := gtables.NewItemLedgerEntry(r.run)
	for found := bomLedgerEntry2.FindSetBuffered(); found; found = bomLedgerEntry2.Next() {
		if itemLedgerEntry2.Get(bomLedgerEntry2.Reference_entry_no) {
			if !itemLedgerEntry2.Cost_is_adjusted {
				return false
			}
		}
	}
	return true
}

// getBOMRegisterLine (GetBOMRegisterLine): the BOM register holding the BOM ledger entry
func (r *adjustCostItemEntries) getBOMRegisterLine(bomRegister *gtables.BOMRegister) bool {
	bomRegister.SetCurrentKey("from_entry_no", "to_entry_no")
	bomRegister.SetFilter("from_entry_no", ".."+system.Format(r.bomLedgerEntry.Entry_no))
	bomRegister.SetFilter("to_entry_no", system.Format(r.bomLedgerEntry.Entry_no)+"..")
	return bomRegister.FindFirst()
}
