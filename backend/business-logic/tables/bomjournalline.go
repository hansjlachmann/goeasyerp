package tables

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/noseries"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// bomJournalLineLogic is the business logic of table 89 BOM Journal Line: a line assembles a
// quantity of a bill of materials (its components are used when it is posted).
//
// Not migrated yet: the availability check of the components (codeunit 311), item tracking
// (Lot No.), bins and recurring journals.
type bomJournalLineLogic struct{}

func init() { gtables.RegisterBOMJournalLineLogic(bomJournalLineLogic{}) }

// OnInsert: the line's template and batch exist
func (bomJournalLineLogic) OnInsert(rec *gtables.BOMJournalLine) error {
	bomJnlTemplate := gtables.NewBOMJournalTemplate(rec)
	err := bomJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return err
	}
	bomJnlBatch := gtables.NewBOMJournalBatch(rec)
	return bomJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "name": rec.Journal_batch_name})
}

// OnValidate_Item_no: the item must be a bill of materials; its description, unit cost,
// inventory posting group and base unit of measure
func (bomJournalLineLogic) OnValidate_Item_no(rec *gtables.BOMJournalLine) error {
	rec.Variant_code = ""
	if rec.Item_no == "" {
		return nil
	}
	item := gtables.NewItem(rec)
	err := item.GetOrError(rec.Item_no)
	if err != nil {
		return err
	}
	item.CalcFields("bill_of_materials")
	err = item.TestField("bill_of_materials", true)
	if err != nil {
		return err
	}
	err = item.TestField("blocked", false)
	if err != nil {
		return err
	}
	rec.Description = item.Description
	rec.Unit_cost = item.Unit_cost
	rec.Inventory_posting_group = item.Inventory_posting_group
	rec.Reserve = gtables.BOMJournalLineReserve(item.Reserve)
	rec.Total_cost = system.Round(rec.Quantity.Mul(rec.Unit_cost))
	return rec.Validate("unit_of_measure_code", item.Base_unit_of_measure)
}

// OnValidate_Posting_date: the document date follows
func (bomJournalLineLogic) OnValidate_Posting_date(rec *gtables.BOMJournalLine) error {
	return rec.Validate("document_date", rec.Posting_date)
}

// OnValidate_Quantity: Quantity (Base) and Total Cost
func (bomJournalLineLogic) OnValidate_Quantity(rec *gtables.BOMJournalLine) error {
	qtyBase, err := calcBOMJnlBaseQty(rec, rec.Quantity)
	if err != nil {
		return err
	}
	rec.Quantity_base = qtyBase
	rec.Total_cost = system.Round(rec.Quantity.Mul(rec.Unit_cost))
	return checkReservedNotAbove(gtables.BOMJournalLineTableName, "reserved_qty_base", rec.Quantity_base, rec.Reserved_qty_base)
}

// calcBOMJnlBaseQty (CalcBaseQty): a quantity in the base unit of measure
func calcBOMJnlBaseQty(rec *gtables.BOMJournalLine, qty types.Decimal) (types.Decimal, error) {
	err := rec.TestField("qty_per_unit_of_measure")
	if err != nil {
		return types.Decimal{}, err
	}
	return system.Round(qty.Mul(rec.Qty_per_unit_of_measure), unitPrecision), nil
}

// OnValidate_Variant_code: the variant's description
func (bomJournalLineLogic) OnValidate_Variant_code(rec *gtables.BOMJournalLine) error {
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

// OnValidate_Unit_of_measure_code: the quantity per unit of measure and the unit cost of the
// unit; the quantity again
func (bomJournalLineLogic) OnValidate_Unit_of_measure_code(rec *gtables.BOMJournalLine) error {
	item := gtables.NewItem(rec)
	err := item.GetOrError(rec.Item_no)
	if err != nil {
		return err
	}
	rec.Qty_per_unit_of_measure, err = GetQtyPerUnitOfMeasure(item, rec.Unit_of_measure_code)
	if err != nil {
		return err
	}
	rec.Unit_cost = system.Round(item.Unit_cost.Mul(rec.Qty_per_unit_of_measure), unitPrecision)
	return rec.Validate("quantity", rec.Quantity)
}

// OnValidate_Quantity_base: only for a line in the base unit of measure
func (bomJournalLineLogic) OnValidate_Quantity_base(rec *gtables.BOMJournalLine) error {
	err := rec.TestField("qty_per_unit_of_measure", types.NewDecimalFromInt(1))
	if err != nil {
		return err
	}
	return rec.Validate("quantity", rec.Quantity_base)
}

// BOMJnlLineEmptyLine (EmptyLine): a line without item and quantity
func BOMJnlLineEmptyLine(rec *gtables.BOMJournalLine) bool {
	return rec.Item_no == "" && rec.Quantity.IsZero()
}

// SetUpNewBOMJnlLine (SetUpNewLine): a new line takes the posting date and document no. of the
// last line — the first line of a batch the work date and the next number of the batch's
// series —, the template's source code and the batch's reason code and posting no. series
func SetUpNewBOMJnlLine(rec, lastBOMJnlLine *gtables.BOMJournalLine) error {
	bomJnlTemplate := gtables.NewBOMJournalTemplate(rec)
	err := bomJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return err
	}
	bomJnlBatch := gtables.NewBOMJournalBatch(rec)
	err = bomJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "name": rec.Journal_batch_name})
	if err != nil {
		return err
	}
	bomJnlLine := gtables.NewBOMJournalLine(rec)
	bomJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	bomJnlLine.SetRange("journal_batch_name", rec.Journal_batch_name)
	if bomJnlLine.FindFirst() {
		rec.Posting_date = lastBOMJnlLine.Posting_date
		rec.Document_date = lastBOMJnlLine.Posting_date
		rec.Document_no = lastBOMJnlLine.Document_no
	} else {
		rec.Posting_date = system.WorkDate()
		rec.Document_date = system.WorkDate()
		if bomJnlBatch.No_series != "" {
			noSeriesMgt := noseries.New(rec)
			rec.Document_no = noSeriesMgt.TryGetNextNo(bomJnlBatch.No_series, rec.Posting_date)
		}
	}
	rec.Recurring_method = lastBOMJnlLine.Recurring_method
	rec.Source_code = bomJnlTemplate.Source_code
	rec.Reason_code = bomJnlBatch.Reason_code
	rec.Posting_no_series = bomJnlBatch.Posting_no_series
	return nil
}
