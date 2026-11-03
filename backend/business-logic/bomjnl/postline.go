package bomjnl

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/reservmgt"
	"github.com/hansjlachmann/openerp/backend/business-logic/resjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

var unitPrecision = types.MustDecimal("0.00001")

// BOMJnlPostLine is codeunit 32 BOM Jnl.-Post Line: it posts a BOM journal line — the assembled
// item as a positive adjustment (codeunit 22), each component as a negative adjustment of the
// item or as usage of the resource (codeunit 212) — with BOM ledger entries pointing at those
// entries and the BOM Register. The assembled item's entry costs what its components cost (plus
// the item's indirect cost; a Standard item its standard cost), which also becomes its last
// direct cost and goes into its average cost. Open it on the posting's transaction; one instance
// per posting keeps the entry numbers and the registers.
//
// With an additional reporting currency the assembled item's cost is exchanged at the posting
// date's rate (its Additional-Currency Unit Cost, adjusted costs and the item's Add.-Curr.
// Average Cost).
//
// The assembled item's reservations go to the item journal line posting it (and from it to
// the item ledger entry). Not migrated yet: item tracking (Lot No.), bins.
type BOMJnlPostLine struct {
	src ftables.Source

	bomJnlLine      *gtables.BOMJournalLine
	bomReg          *gtables.BOMRegister
	bomJnlCheckLine BOMJnlCheckLine
	itemJnlPostLine *itemjnl.ItemJnlPostLine
	resJnlPostLine  *resjnl.ResJnlPostLine
	nextEntryNo     int
}

// NewBOMJnlPostLine opens the posting codeunit in the posting's database (its transaction)
func NewBOMJnlPostLine(src ftables.Source) *BOMJnlPostLine {
	return &BOMJnlPostLine{src: src, itemJnlPostLine: itemjnl.NewItemJnlPostLine(src), resJnlPostLine: resjnl.NewResJnlPostLine(src)}
}

// Run checks and posts a journal line (OnRun)
func (p *BOMJnlPostLine) Run(bomJnlLine *gtables.BOMJournalLine) error {
	p.bomJnlLine = gtables.NewBOMJournalLine(p.src)
	p.bomJnlLine.Copy(bomJnlLine)
	return p.code()
}

// GetBOMReg returns the BOM Register of the posting (No. 0 before the first entry)
func (p *BOMJnlPostLine) GetBOMReg() *gtables.BOMRegister {
	if p.bomReg == nil {
		return gtables.NewBOMRegister(p.src)
	}
	return p.bomReg
}

func (p *BOMJnlPostLine) code() error {
	bomJnlLine := p.bomJnlLine
	if tables.BOMJnlLineEmptyLine(bomJnlLine) {
		return nil
	}
	err := p.bomJnlCheckLine.RunCheck(bomJnlLine)
	if err != nil {
		return err
	}
	item := gtables.NewItem(p.src)
	err = item.GetOrError(bomJnlLine.Item_no)
	if err != nil {
		return err
	}
	if item.Costing_method == gtables.Item_Costing_method.Specific {
		return apperrors.BOMCostingMethod(gtables.ItemTableName, "costing_method", bomJnlLine.Item_no.String(), item.Costing_method)
	}

	// The assembled item: a positive adjustment
	itemJnlLine := gtables.NewItemJournalLine(p.src)
	itemJnlLine.Init()
	err = itemJnlLine.Validate("posting_date", bomJnlLine.Posting_date)
	if err != nil {
		return err
	}
	itemJnlLine.Document_no = bomJnlLine.Document_no
	itemJnlLine.External_document_no = bomJnlLine.External_document_no
	itemJnlLine.Entry_type = gtables.ItemJournalLine_Entry_type.PositiveAdjmt
	err = itemJnlLine.Validate("item_no", bomJnlLine.Item_no)
	if err != nil {
		return err
	}
	err = itemJnlLine.Validate("variant_code", bomJnlLine.Variant_code)
	if err != nil {
		return err
	}
	itemJnlLine.Description = bomJnlLine.Description
	itemJnlLine.Department_code = bomJnlLine.Department_code
	itemJnlLine.Project_code = bomJnlLine.Project_code
	itemJnlLine.Location_code = bomJnlLine.Location_code
	itemJnlLine.Source_code = bomJnlLine.Source_code
	itemJnlLine.Journal_batch_name = bomJnlLine.Journal_batch_name
	itemJnlLine.Reason_code = bomJnlLine.Reason_code
	itemJnlLine.Source_type = gtables.ItemJournalLine_Source_type.Item
	err = itemJnlLine.Validate("unit_of_measure_code", bomJnlLine.Unit_of_measure_code)
	if err != nil {
		return err
	}
	itemJnlLine.Qty_per_unit_of_measure = bomJnlLine.Qty_per_unit_of_measure
	err = itemJnlLine.Validate("quantity", bomJnlLine.Quantity)
	if err != nil {
		return err
	}
	if item.Costing_method == gtables.Item_Costing_method.Average {
		itemJnlLine.Invoiced_quantity = types.Decimal{} // to delay Sales Invoice Adjustment
		itemJnlLine.Invoiced_qty_base = types.Decimal{}
	}
	itemJnlLine.Posting_no_series = bomJnlLine.Posting_no_series
	itemJnlLine.Bin_code = bomJnlLine.Bin_code
	itemJnlLine.Lot_no = bomJnlLine.Lot_no

	if !bomJnlLine.Reserved_qty_base.IsZero() {
		err = reservmgt.New(p.src).TransfBOMJnlLineToItemJnlLine(bomJnlLine, itemJnlLine)
		if err != nil {
			return err
		}
	}
	err = p.itemJnlPostLine.Run(itemJnlLine)
	if err != nil {
		return err
	}

	if p.nextEntryNo == 0 {
		bomLedgEntry := gtables.NewBOMLedgerEntry(p.src)
		bomLedgEntry.LockTable()
		if bomLedgEntry.FindLast() {
			p.nextEntryNo = bomLedgEntry.Entry_no
		}
		p.nextEntryNo = p.nextEntryNo + 1
	}
	itemLedgEntry := gtables.NewItemLedgerEntry(p.src)
	itemLedgEntry.FindLast()
	bomItemEntryNo := itemLedgEntry.Entry_no
	bomLedgEntry := gtables.NewBOMLedgerEntry(p.src)
	bomLedgEntry.Entry_no = p.nextEntryNo
	bomLedgEntry.Entry_type = gtables.BOMLedgerEntry_Entry_type.BOM
	bomLedgEntry.Reference_type = gtables.BOMLedgerEntry_Reference_type.Item
	bomLedgEntry.Reference_entry_no = itemLedgEntry.Entry_no
	bomLedgEntry.Posting_date = bomJnlLine.Posting_date
	bomLedgEntry.Document_date = bomJnlLine.Document_date
	err = bomLedgEntry.InsertOrError(false)
	if err != nil {
		return err
	}
	p.nextEntryNo = p.nextEntryNo + 1
	if p.bomReg == nil || p.bomReg.No == 0 {
		p.bomReg = gtables.NewBOMRegister(p.src)
		p.bomReg.LockTable()
		no := 1
		if p.bomReg.FindLast() {
			no = p.bomReg.No + 1
		}
		p.bomReg.Init()
		p.bomReg.No = no
		p.bomReg.From_entry_no = bomLedgEntry.Entry_no
		p.bomReg.To_entry_no = p.nextEntryNo
		p.bomReg.Creation_date = system.Today()
		p.bomReg.Source_code = bomJnlLine.Source_code
		p.bomReg.Journal_batch_name = bomJnlLine.Journal_batch_name
		p.bomReg.User_id = types.Code(system.UserId())
		err = p.bomReg.InsertOrError(false)
	} else {
		p.bomReg.To_entry_no = p.nextEntryNo
		err = p.bomReg.ModifyOrError(false)
	}
	if err != nil {
		return err
	}

	// The components: negative adjustments of the items, usage of the resources
	bomTotalCost := types.Decimal{}
	bomComp := gtables.NewBOMComponent(p.src)
	bomComp.SetRange("parent_item_no", bomJnlLine.Item_no)
	item.Bill_of_materials = bomComp.FindFirst()
	err = item.TestField("bill_of_materials", true)
	if err != nil {
		return err
	}
	for found := bomComp.FindSet(); found; found = bomComp.Next() {
		compQty := system.Round(bomJnlLine.Quantity_base.Mul(bomComp.Quantity_per), unitPrecision)
		if !compQty.IsZero() {
			switch bomComp.Type {
			case gtables.BOMComponent_Type.Item:
				item2 := gtables.NewItem(p.src)
				err = item2.GetOrError(bomComp.No)
				if err != nil {
					return err
				}
				if item2.Costing_method == gtables.Item_Costing_method.Specific {
					return apperrors.BOMCompCostingMethod(gtables.ItemTableName, "costing_method", bomComp.No.String(), item2.Costing_method)
				}
				compCost, err := p.postItemComponent(bomComp, compQty)
				if err != nil {
					return err
				}
				bomTotalCost = bomTotalCost.Add(compCost)
				itemLedgEntry = gtables.NewItemLedgerEntry(p.src)
				itemLedgEntry.FindLast()
				bomLedgEntry.Reference_entry_no = itemLedgEntry.Entry_no
			case gtables.BOMComponent_Type.Resource:
				compCost, err := p.postResComponent(bomComp, compQty)
				if err != nil {
					return err
				}
				bomTotalCost = bomTotalCost.Add(compCost)
				resLedgEntry := gtables.NewResLedgerEntry(p.src)
				resLedgEntry.FindLast()
				bomLedgEntry.Reference_entry_no = resLedgEntry.Entry_no
			}
			bomLedgEntry.Entry_no = p.nextEntryNo
			bomLedgEntry.Entry_type = gtables.BOMLedgerEntry_Entry_type.Component
			bomLedgEntry.Reference_type = gtables.BOMLedgerEntryReference_type(bomComp.Type - 1)
			bomLedgEntry.Posting_date = bomJnlLine.Posting_date
			bomLedgEntry.Document_date = bomJnlLine.Document_date
			err = bomLedgEntry.InsertOrError(false)
			if err != nil {
				return err
			}
			p.nextEntryNo = p.nextEntryNo + 1
			p.bomReg.To_entry_no = bomLedgEntry.Entry_no
			err = p.bomReg.ModifyOrError(false)
			if err != nil {
				return err
			}
		}
	}

	// The assembled item's entry costs what its components cost
	itemLedgEntry = gtables.NewItemLedgerEntry(p.src)
	err = itemLedgEntry.GetOrError(bomItemEntryNo)
	if err != nil {
		return err
	}
	itemLedgEntry.Cost_is_adjusted = false // components might be adjusted later
	if item.Costing_method == gtables.Item_Costing_method.Standard {
		itemLedgEntry.Unit_cost = item.Unit_cost
		itemLedgEntry.Additional_currency_unit_cost, err = p.itemJnlPostLine.AddCurrencyUnitCost(bomJnlLine.Posting_date, itemLedgEntry.Unit_cost)
		if err != nil {
			return err
		}
		itemLedgEntry.Cost_is_adjusted = true
	} else {
		if item.Costing_method == gtables.Item_Costing_method.Average {
			itemLedgEntry.Invoiced_quantity = itemLedgEntry.Quantity // restore from delay
			itemLedgEntry.Adjusted_cost_invoiced_qty = itemLedgEntry.Adjusted_cost_qty
			// the classic leaves the additional-currency cost out here, so the average below
			// would not take the delayed amount off
			itemLedgEntry.Add_curr_adj_cost_inv_qty = itemLedgEntry.Add_currency_adj_cost_qty
			err = itemLedgEntry.ModifyOrError(false)
			if err != nil {
				return err
			}
		}
		item.LockTable()
		err = item.GetOrError(bomJnlLine.Item_no)
		if err != nil {
			return err
		}
		hundred := types.NewDecimalFromInt(100)
		one := types.NewDecimalFromInt(1)
		item.Last_direct_cost = system.Round(bomTotalCost.Div(itemLedgEntry.Quantity), unitPrecision)
		itemLedgEntry.Unit_cost = system.Round(
			bomTotalCost.Div(itemLedgEntry.Invoiced_quantity).Mul(one.Add(item.Indirect_cost_percent.Div(hundred))),
			unitPrecision)
		itemLedgEntry.Additional_currency_unit_cost, err = p.itemJnlPostLine.AddCurrencyUnitCost(bomJnlLine.Posting_date, itemLedgEntry.Unit_cost)
		if err != nil {
			return err
		}
		// adjust average cost
		item.CalcFields("net_invoiced_qty")
		if item.Net_invoiced_qty.GreaterThan(itemLedgEntry.Invoiced_quantity) {
			item.Average_cost = item.Net_invoiced_qty.Mul(item.Average_cost)
			item.Average_cost = item.Average_cost.Sub(itemLedgEntry.Adjusted_cost_invoiced_qty).Add(
				itemLedgEntry.Unit_cost.Mul(itemLedgEntry.Invoiced_quantity))
			item.Average_cost = item.Average_cost.Div(item.Net_invoiced_qty)

			item.Add_curr_average_cost = item.Net_invoiced_qty.Mul(item.Add_curr_average_cost)
			item.Add_curr_average_cost = item.Add_curr_average_cost.Sub(itemLedgEntry.Add_curr_adj_cost_inv_qty).Add(
				itemLedgEntry.Additional_currency_unit_cost.Mul(itemLedgEntry.Invoiced_quantity))
			item.Add_curr_average_cost = item.Add_curr_average_cost.Div(item.Net_invoiced_qty)
			addCurrAverageCost := p.itemJnlPostLine.RoundAddCurrUnitAmount(item.Add_curr_average_cost)

			err = item.Validate("add_curr_average_cost", addCurrAverageCost)
			if err != nil {
				return err
			}
			err = item.Validate("average_cost", system.Round(item.Average_cost, unitPrecision))
		} else {
			err = item.Validate("add_curr_average_cost", itemLedgEntry.Additional_currency_unit_cost)
			if err != nil {
				return err
			}
			err = item.Validate("average_cost", itemLedgEntry.Unit_cost)
		}
		if err != nil {
			return err
		}
		err = item.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	itemLedgEntry.Unit_amount = system.Round(itemLedgEntry.Unit_cost)
	itemLedgEntry.Adjusted_cost_invoiced_qty = system.Round(itemLedgEntry.Unit_cost.Mul(itemLedgEntry.Invoiced_quantity))
	addCurrUnitCost, err := p.itemJnlPostLine.AddCurrencyUnitCost(bomJnlLine.Posting_date, itemLedgEntry.Unit_cost)
	if err != nil {
		return err
	}
	itemLedgEntry.Add_curr_adj_cost_inv_qty = p.itemJnlPostLine.RoundAddCurrAmount(addCurrUnitCost.Mul(itemLedgEntry.Invoiced_quantity))
	itemLedgEntry.Adjusted_cost_qty = itemLedgEntry.Adjusted_cost_invoiced_qty
	itemLedgEntry.Add_currency_adj_cost_qty = itemLedgEntry.Add_curr_adj_cost_inv_qty
	itemLedgEntry.Amount = itemLedgEntry.Adjusted_cost_invoiced_qty
	// The changed cost reaches the G/L at once with Automatic Cost Posting (as codeunit 22 does
	// when it adjusts an entry); otherwise Post Inventory Cost to G/L posts it
	if p.itemJnlPostLine.AutomaticCostPosting() && !itemLedgEntry.Cost_posted_to_gl.Equal(itemLedgEntry.Adjusted_cost_invoiced_qty) {
		err = p.itemJnlPostLine.PostInventoryToGL(itemLedgEntry)
		if err != nil {
			return err
		}
	}
	err = itemLedgEntry.ModifyOrError(false)
	if err != nil {
		return err
	}
	if item.Costing_method == gtables.Item_Costing_method.Average {
		return p.itemJnlPostLine.AdjustSalesInv(itemLedgEntry)
	}
	return nil
}

// postItemComponent posts an item component as a negative adjustment; it returns the
// component's cost
func (p *BOMJnlPostLine) postItemComponent(bomComp *gtables.BOMComponent, compQty types.Decimal) (types.Decimal, error) {
	bomJnlLine := p.bomJnlLine
	itemJnlLine := gtables.NewItemJournalLine(p.src)
	itemJnlLine.Init()
	err := itemJnlLine.Validate("posting_date", bomJnlLine.Posting_date)
	if err != nil {
		return types.Decimal{}, err
	}
	itemJnlLine.Document_no = bomJnlLine.Document_no
	itemJnlLine.External_document_no = bomJnlLine.External_document_no
	itemJnlLine.Entry_type = gtables.ItemJournalLine_Entry_type.NegativeAdjmt
	err = itemJnlLine.Validate("item_no", bomComp.No)
	if err != nil {
		return types.Decimal{}, err
	}
	err = itemJnlLine.Validate("variant_code", bomComp.Variant_code)
	if err != nil {
		return types.Decimal{}, err
	}
	itemJnlLine.Description = bomJnlLine.Description
	itemJnlLine.Department_code = bomJnlLine.Department_code
	itemJnlLine.Project_code = bomJnlLine.Project_code
	itemJnlLine.Location_code = bomJnlLine.Location_code
	itemJnlLine.Source_code = bomJnlLine.Source_code
	itemJnlLine.Journal_batch_name = bomJnlLine.Journal_batch_name
	itemJnlLine.Reason_code = bomJnlLine.Reason_code
	itemJnlLine.Source_type = gtables.ItemJournalLine_Source_type.Item
	itemJnlLine.Source_no = bomJnlLine.Item_no
	itemJnlLine.Source_posting_group = bomJnlLine.Inventory_posting_group
	err = itemJnlLine.Validate("unit_of_measure_code", bomComp.Unit_of_measure_code)
	if err != nil {
		return types.Decimal{}, err
	}
	err = itemJnlLine.Validate("quantity", compQty)
	if err != nil {
		return types.Decimal{}, err
	}
	itemJnlLine.Posting_no_series = bomJnlLine.Posting_no_series
	err = p.itemJnlPostLine.Run(itemJnlLine)
	if err != nil {
		return types.Decimal{}, err
	}
	return itemJnlLine.Amount, nil
}

// postResComponent posts a resource component as usage; it returns the component's cost
func (p *BOMJnlPostLine) postResComponent(bomComp *gtables.BOMComponent, compQty types.Decimal) (types.Decimal, error) {
	bomJnlLine := p.bomJnlLine
	resJnlLine := gtables.NewResJournalLine(p.src)
	resJnlLine.Init()
	err := resJnlLine.Validate("posting_date", bomJnlLine.Posting_date)
	if err != nil {
		return types.Decimal{}, err
	}
	resJnlLine.Document_no = bomJnlLine.Document_no
	resJnlLine.External_document_no = bomJnlLine.External_document_no
	resJnlLine.Entry_type = gtables.ResJournalLine_Entry_type.Usage
	err = resJnlLine.Validate("resource_no", bomComp.No)
	if err != nil {
		return types.Decimal{}, err
	}
	resJnlLine.Description = bomJnlLine.Description
	resJnlLine.Department_code = bomJnlLine.Department_code
	resJnlLine.Project_code = bomJnlLine.Project_code
	resJnlLine.Source_code = bomJnlLine.Source_code
	resJnlLine.Journal_batch_name = bomJnlLine.Journal_batch_name
	resJnlLine.Reason_code = bomJnlLine.Reason_code
	resJnlLine.Unit_of_measure_code = bomComp.Unit_of_measure_code
	err = resJnlLine.Validate("quantity", compQty)
	if err != nil {
		return types.Decimal{}, err
	}
	resJnlLine.Posting_no_series = bomJnlLine.Posting_no_series
	err = p.resJnlPostLine.Run(resJnlLine)
	if err != nil {
		return types.Decimal{}, err
	}
	return resJnlLine.Total_cost, nil
}
