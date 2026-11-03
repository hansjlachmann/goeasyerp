package tables

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/noseries"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// itemJournalLineLogic is the business logic of table 83 Item Journal Line.
//
// Not migrated yet: reservations (the line's reserved quantities, the reservation checks of
// OnModify, OnDelete, OnRename and the fields), the availability warning (codeunit 311 Item-Check
// Avail.), item tracking (serial and lot numbers) and bins.
type itemJournalLineLogic struct{}

func init() { gtables.RegisterItemJournalLineLogic(itemJournalLineLogic{}) }

var unitPrecision = types.MustDecimal("0.00001")

// OnInsert: the line's template and batch exist
func (itemJournalLineLogic) OnInsert(rec *gtables.ItemJournalLine) error {
	rec.LockTable()
	itemJnlTemplate := gtables.NewItemJournalTemplate(rec)
	err := itemJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return err
	}
	itemJnlBatch := gtables.NewItemJournalBatch(rec)
	return itemJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "name": rec.Journal_batch_name})
}

// OnValidate_Item_no: the item's description, posting groups, unit cost and the unit amount and
// unit of measure of the entry type
func (itemJournalLineLogic) OnValidate_Item_no(rec *gtables.ItemJournalLine) error {
	xRec := rec.XRec()
	if rec.Item_no != xRec.Item_no {
		rec.Variant_code = ""
	}
	if rec.Item_no == "" {
		return nil
	}
	item := gtables.NewItem(rec)
	err := item.GetOrError(rec.Item_no)
	if err != nil {
		return err
	}
	err = item.TestField("blocked", false)
	if err != nil {
		return err
	}
	rec.Description = item.Description
	rec.Inventory_posting_group = item.Inventory_posting_group
	rec.Unit_cost = item.Unit_cost
	rec.Indirect_cost_percent = item.Indirect_cost_percent
	rec.Gen_prod_posting_group = item.Gen_prod_posting_group
	rec.Reserve = gtables.ItemJournalLineReserve(item.Reserve)
	err = GetItemJnlUnitAmount(rec, item)
	if err != nil {
		return err
	}
	err = rec.Validate("unit_amount", rec.Unit_amount)
	if err != nil {
		return err
	}
	switch rec.Entry_type {
	case gtables.ItemJournalLine_Entry_type.Purchase:
		return rec.Validate("unit_of_measure_code", item.Purch_unit_of_measure)
	case gtables.ItemJournalLine_Entry_type.Sale:
		return rec.Validate("unit_of_measure_code", item.Sales_unit_of_measure)
	default:
		return rec.Validate("unit_of_measure_code", item.Base_unit_of_measure)
	}
}

// GetItemJnlUnitAmount (GetUnitAmount): the unit amount of the entry type — a purchase and a
// positive adjustment the last direct cost, a sale the unit price (without the tax of a price
// including tax), a negative adjustment the unit cost, a transfer none
func GetItemJnlUnitAmount(rec *gtables.ItemJournalLine, item *gtables.Item) error {
	switch rec.Entry_type {
	case gtables.ItemJournalLine_Entry_type.Purchase:
		rec.Unit_amount = item.Last_direct_cost
	case gtables.ItemJournalLine_Entry_type.Sale:
		if item.Price_includes_tax {
			taxPostingSetup := gtables.NewTaxPostingSetup(rec)
			err := taxPostingSetup.GetOrError(map[string]interface{}{"tax_bus_posting_group": item.Tax_bus_posting_gr_price, "tax_prod_posting_group": item.Tax_prod_posting_group})
			if err != nil {
				return err
			}
			switch taxPostingSetup.Tax_calculation_type {
			case gtables.TaxPostingSetup_Tax_calculation_type.ReverseChargeVAT:
				taxPostingSetup.Vat_percent = types.Decimal{}
			case gtables.TaxPostingSetup_Tax_calculation_type.SalesTax:
				return apperrors.ItemPriceInclTaxSalesTax(gtables.TaxPostingSetupTableName, "tax_calculation_type", taxPostingSetup.Tax_calculation_type)
			}
			rec.Unit_amount = system.Round(item.Unit_price.Div(types.NewDecimalFromInt(1).Add(taxPostingSetup.Vat_percent.Div(types.NewDecimalFromInt(100)))), unitPrecision)
		} else {
			rec.Unit_amount = item.Unit_price
		}
	case gtables.ItemJournalLine_Entry_type.PositiveAdjmt:
		rec.Unit_amount = item.Last_direct_cost
	case gtables.ItemJournalLine_Entry_type.NegativeAdjmt:
		rec.Unit_amount = item.Unit_cost
	case gtables.ItemJournalLine_Entry_type.Transfer:
		rec.Unit_amount = types.Decimal{}
		rec.Unit_cost = types.Decimal{}
		rec.Amount = types.Decimal{}
	}
	return nil
}

// OnValidate_Posting_date: the document date follows
func (itemJournalLineLogic) OnValidate_Posting_date(rec *gtables.ItemJournalLine) error {
	return rec.Validate("document_date", rec.Posting_date)
}

// OnValidate_Entry_type: only adjustments on a phys. inventory line; the item's amounts again;
// the transfer fields only on transfers
func (itemJournalLineLogic) OnValidate_Entry_type(rec *gtables.ItemJournalLine) error {
	if rec.Entry_type != gtables.ItemJournalLine_Entry_type.PositiveAdjmt && rec.Entry_type != gtables.ItemJournalLine_Entry_type.NegativeAdjmt {
		err := rec.TestField("phys_inventory", false)
		if err != nil {
			return err
		}
	}
	err := rec.Validate("item_no", rec.Item_no)
	if err != nil {
		return err
	}
	if rec.Entry_type != gtables.ItemJournalLine_Entry_type.Transfer {
		rec.New_location_code = ""
		rec.New_dimension_1_code = ""
		rec.New_dimension_2_code = ""
		rec.New_serial_no = ""
	}
	return nil
}

// OnValidate_Quantity: a phys. inventory line takes its quantity from the counted quantity
func (itemJournalLineLogic) OnValidate_Quantity(rec *gtables.ItemJournalLine) error {
	err := rec.TestField("phys_inventory", false)
	if err != nil {
		return err
	}
	return validateItemJnlQuantity(rec)
}

// validateItemJnlQuantity: the base quantity, invoiced quantities and amount follow the quantity
func validateItemJnlQuantity(rec *gtables.ItemJournalLine) error {
	quantityBase, err := calcItemJnlBaseQty(rec, rec.Quantity)
	if err != nil {
		return err
	}
	rec.Quantity_base = quantityBase
	err = checkReservedNotAbove(gtables.ItemJournalLineTableName, "positive_reserved_qty_base", rec.Quantity_base, rec.Positive_reserved_qty_base)
	if err == nil {
		err = checkReservedNotAbove(gtables.ItemJournalLineTableName, "negative_reserved_qty_base", rec.Quantity_base, rec.Negative_reserved_qty_base)
	}
	if err != nil {
		return err
	}
	rec.Invoiced_quantity = rec.Quantity
	rec.Invoiced_qty_base = rec.Quantity_base
	rec.Amount = system.Round(rec.Quantity.Mul(rec.Unit_amount))
	if rec.Entry_type == gtables.ItemJournalLine_Entry_type.Transfer {
		rec.Qty_calculated = types.Decimal{}
		rec.Qty_phys_inventory = types.Decimal{}
		rec.Last_item_ledger_entry_no = 0
	}
	return nil
}

// calcItemJnlBaseQty (CalcBaseQty): a quantity in the base unit of measure
func calcItemJnlBaseQty(rec *gtables.ItemJournalLine, qty types.Decimal) (types.Decimal, error) {
	err := rec.TestField("qty_per_unit_of_measure")
	if err != nil {
		return types.Decimal{}, err
	}
	return system.Round(qty.Mul(rec.Qty_per_unit_of_measure), unitPrecision), nil
}

// OnValidate_Unit_amount: the amount; a purchase or positive adjustment's unit cost with the
// indirect cost
func (itemJournalLineLogic) OnValidate_Unit_amount(rec *gtables.ItemJournalLine) error {
	rec.Amount = system.Round(rec.Quantity.Mul(rec.Unit_amount))
	if rec.Item_no != "" && (rec.Entry_type == gtables.ItemJournalLine_Entry_type.Purchase || rec.Entry_type == gtables.ItemJournalLine_Entry_type.PositiveAdjmt) {
		rec.Unit_cost = system.Round(rec.Unit_amount.Mul(types.NewDecimalFromInt(1).Add(rec.Indirect_cost_percent.Div(types.NewDecimalFromInt(100)))), unitPrecision)
		return validateItemJnlUnitCost(rec, false)
	}
	return nil
}

// OnValidate_Unit_cost: entered by the user — not for a standard cost item
func (itemJournalLineLogic) OnValidate_Unit_cost(rec *gtables.ItemJournalLine) error {
	return validateItemJnlUnitCost(rec, true)
}

// validateItemJnlUnitCost: a purchase or positive adjustment of a standard cost item costs the
// standard cost (the user cannot change it: byUser); otherwise the indirect cost % follows
func validateItemJnlUnitCost(rec *gtables.ItemJournalLine, byUser bool) error {
	err := rec.TestField("item_no")
	if err != nil {
		return err
	}
	if rec.Entry_type == gtables.ItemJournalLine_Entry_type.Purchase || rec.Entry_type == gtables.ItemJournalLine_Entry_type.PositiveAdjmt {
		item := gtables.NewItem(rec)
		err = item.GetOrError(rec.Item_no)
		if err != nil {
			return err
		}
		if item.Costing_method == gtables.Item_Costing_method.Standard {
			if byUser {
				return apperrors.ItemCannotChangeWhen(gtables.ItemTableName, "unit_cost", "costing_method", item.Costing_method)
			}
			rec.Unit_cost = system.Round(item.Unit_cost.Mul(rec.Qty_per_unit_of_measure), unitPrecision)
			rec.Indirect_cost_percent = types.Decimal{}
		} else {
			if !rec.Unit_amount.IsZero() {
				rec.Indirect_cost_percent = system.Round(rec.Unit_cost.Sub(rec.Unit_amount).Div(rec.Unit_amount).Mul(types.NewDecimalFromInt(100)), unitPrecision)
			} else {
				rec.Indirect_cost_percent = types.Decimal{}
			}
		}
	}
	return nil
}

// OnValidate_Amount: the unit amount from the amount and quantity
func (itemJournalLineLogic) OnValidate_Amount(rec *gtables.ItemJournalLine) error {
	err := rec.TestField("quantity")
	if err != nil {
		return err
	}
	rec.Unit_amount = rec.Amount.Div(rec.Quantity)
	err = rec.Validate("unit_amount", rec.Unit_amount)
	if err != nil {
		return err
	}
	rec.Unit_amount = system.Round(rec.Unit_amount, unitPrecision)
	return nil
}

// OnValidate_Indirect_cost_percent: not on sales and negative adjustments nor standard cost
// items; the unit cost follows
func (itemJournalLineLogic) OnValidate_Indirect_cost_percent(rec *gtables.ItemJournalLine) error {
	err := rec.TestField("item_no")
	if err != nil {
		return err
	}
	if rec.Entry_type == gtables.ItemJournalLine_Entry_type.Sale || rec.Entry_type == gtables.ItemJournalLine_Entry_type.NegativeAdjmt {
		return apperrors.ItemCannotChangeWhen(gtables.ItemJournalLineTableName, "indirect_cost_percent", "entry_type", rec.Entry_type)
	}
	item := gtables.NewItem(rec)
	err = item.GetOrError(rec.Item_no)
	if err != nil {
		return err
	}
	if item.Costing_method == gtables.Item_Costing_method.Standard {
		return apperrors.ItemCannotChangeWhen(gtables.ItemTableName, "indirect_cost_percent", "costing_method", item.Costing_method)
	}
	rec.Unit_cost = system.Round(rec.Unit_amount.Mul(types.NewDecimalFromInt(1).Add(rec.Indirect_cost_percent.Div(types.NewDecimalFromInt(100)))), unitPrecision)
	return nil
}

// OnValidate_New_location_code: only on transfers
func (itemJournalLineLogic) OnValidate_New_location_code(rec *gtables.ItemJournalLine) error {
	return rec.TestField("entry_type", gtables.ItemJournalLine_Entry_type.Transfer)
}

// OnValidate_New_dimension_1_code: only on transfers
func (itemJournalLineLogic) OnValidate_New_dimension_1_code(rec *gtables.ItemJournalLine) error {
	return rec.TestField("entry_type", gtables.ItemJournalLine_Entry_type.Transfer)
}

// OnValidate_New_dimension_2_code: only on transfers
func (itemJournalLineLogic) OnValidate_New_dimension_2_code(rec *gtables.ItemJournalLine) error {
	return rec.TestField("entry_type", gtables.ItemJournalLine_Entry_type.Transfer)
}

// OnValidate_Qty_calculated: the counted quantity again
func (itemJournalLineLogic) OnValidate_Qty_calculated(rec *gtables.ItemJournalLine) error {
	return rec.Validate("qty_phys_inventory", rec.Qty_phys_inventory)
}

// OnValidate_Qty_phys_inventory: the counted quantity of a phys. inventory line gives a positive
// or negative adjustment of the difference to the calculated quantity
func (itemJournalLineLogic) OnValidate_Qty_phys_inventory(rec *gtables.ItemJournalLine) error {
	err := rec.TestField("phys_inventory", true)
	if err != nil {
		return err
	}
	rec.Quantity = types.Decimal{}
	if !rec.Qty_phys_inventory.LessThan(rec.Qty_calculated) {
		rec.Entry_type = gtables.ItemJournalLine_Entry_type.PositiveAdjmt
		err = rec.Validate("entry_type", rec.Entry_type)
		if err != nil {
			return err
		}
		rec.Quantity = rec.Qty_phys_inventory.Sub(rec.Qty_calculated)
	} else {
		rec.Entry_type = gtables.ItemJournalLine_Entry_type.NegativeAdjmt
		err = rec.Validate("entry_type", rec.Entry_type)
		if err != nil {
			return err
		}
		rec.Quantity = rec.Qty_calculated.Sub(rec.Qty_phys_inventory)
	}
	return validateItemJnlQuantity(rec)
}

// OnValidate_Variant_code: the variant's description
func (itemJournalLineLogic) OnValidate_Variant_code(rec *gtables.ItemJournalLine) error {
	if rec.Variant_code == "" {
		return nil
	}
	itemVariant := gtables.NewItemVariant(rec)
	err := itemVariant.GetOrError(map[string]interface{}{"item_no": rec.Item_no, "code": rec.Variant_code})
	if err != nil {
		return err
	}
	rec.Description = itemVariant.Description
	return nil
}

// OnValidate_Unit_of_measure_code: the unit's quantity in the base unit; unit cost, unit amount
// and the quantities in the unit
func (itemJournalLineLogic) OnValidate_Unit_of_measure_code(rec *gtables.ItemJournalLine) error {
	item := gtables.NewItem(rec)
	err := item.GetOrError(rec.Item_no)
	if err != nil {
		return err
	}
	err = GetItemJnlUnitAmount(rec, item)
	if err != nil {
		return err
	}
	rec.Qty_per_unit_of_measure, err = GetQtyPerUnitOfMeasure(item, rec.Unit_of_measure_code)
	if err != nil {
		return err
	}
	rec.Unit_cost = system.Round(item.Unit_cost.Mul(rec.Qty_per_unit_of_measure), unitPrecision)
	rec.Unit_amount = system.Round(rec.Unit_amount.Mul(rec.Qty_per_unit_of_measure), unitPrecision)
	return validateItemJnlQuantity(rec)
}

// OnValidate_Quantity_base: only in the base unit of measure; the quantity follows
func (itemJournalLineLogic) OnValidate_Quantity_base(rec *gtables.ItemJournalLine) error {
	err := rec.TestField("qty_per_unit_of_measure", types.NewDecimalFromInt(1))
	if err != nil {
		return err
	}
	return rec.Validate("quantity", rec.Quantity_base)
}

// GetQtyPerUnitOfMeasure (codeunit 5402 Unit of Measure Management): the quantity of a unit of
// measure of the item in its base unit — 1 for the base unit or no unit
func GetQtyPerUnitOfMeasure(item *gtables.Item, unitOfMeasureCode types.Code) (types.Decimal, error) {
	if unitOfMeasureCode == item.Base_unit_of_measure || unitOfMeasureCode == "" {
		return types.NewDecimalFromInt(1), nil
	}
	itemUnitOfMeasure := gtables.NewItemUnitOfMeasure(item)
	err := itemUnitOfMeasure.GetOrError(map[string]interface{}{"item_no": item.No, "code": unitOfMeasureCode})
	if err != nil {
		return types.Decimal{}, err
	}
	err = itemUnitOfMeasure.TestField("qty_per_unit_of_measure")
	if err != nil {
		return types.Decimal{}, err
	}
	return itemUnitOfMeasure.Qty_per_unit_of_measure, nil
}

// ItemJnlLineEmptyLine (EmptyLine): a line without item and quantity
func ItemJnlLineEmptyLine(rec *gtables.ItemJournalLine) bool {
	return rec.Item_no == "" && rec.Quantity.IsZero()
}

// ItemJnlLineSigned (Signed): a quantity with the sign of the entry type — negative for sales
// and negative adjustments
func ItemJnlLineSigned(rec *gtables.ItemJournalLine, value types.Decimal) types.Decimal {
	switch rec.Entry_type {
	case gtables.ItemJournalLine_Entry_type.Sale, gtables.ItemJournalLine_Entry_type.NegativeAdjmt:
		return value.Neg()
	}
	return value
}

// SetUpNewItemJnlLine (SetUpNewLine): a new line takes the posting date, document no. and entry
// type of the last line — the first line of a batch the work date and the next number of the
// batch's series —, the template's source code and the batch's reason code and posting no.
// series
func SetUpNewItemJnlLine(rec, lastItemJnlLine *gtables.ItemJournalLine) error {
	itemJnlTemplate := gtables.NewItemJournalTemplate(rec)
	err := itemJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return err
	}
	itemJnlBatch := gtables.NewItemJournalBatch(rec)
	err = itemJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "name": rec.Journal_batch_name})
	if err != nil {
		return err
	}
	itemJnlLine := gtables.NewItemJournalLine(rec)
	itemJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	itemJnlLine.SetRange("journal_batch_name", rec.Journal_batch_name)
	if itemJnlLine.FindFirst() {
		rec.Posting_date = lastItemJnlLine.Posting_date
		rec.Document_date = lastItemJnlLine.Posting_date
		rec.Document_no = lastItemJnlLine.Document_no
	} else {
		rec.Posting_date = system.WorkDate()
		rec.Document_date = system.WorkDate()
		if itemJnlBatch.No_series != "" {
			noSeriesMgt := noseries.New(rec)
			rec.Document_no = noSeriesMgt.TryGetNextNo(itemJnlBatch.No_series, rec.Posting_date)
		}
	}
	rec.Entry_type = lastItemJnlLine.Entry_type
	rec.Recurring_method = lastItemJnlLine.Recurring_method
	rec.Source_code = itemJnlTemplate.Source_code
	rec.Reason_code = itemJnlBatch.Reason_code
	rec.Posting_no_series = itemJnlBatch.Posting_no_series
	return nil
}

// OnValidate_New_serial_no / OnValidate_New_lot_no: only a transfer gives the item a new serial
// or lot number
func (itemJournalLineLogic) OnValidate_New_serial_no(rec *gtables.ItemJournalLine) error {
	return rec.TestField("entry_type", gtables.ItemJournalLine_Entry_type.Transfer)
}

func (itemJournalLineLogic) OnValidate_New_lot_no(rec *gtables.ItemJournalLine) error {
	return rec.TestField("entry_type", gtables.ItemJournalLine_Entry_type.Transfer)
}
