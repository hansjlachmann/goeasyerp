package bomjnl

import (
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/resprice"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The bill of materials functions of the pages: Explode BOM on page 36 Bill of Materials
// (codeunit 51), Calc. Standard Cost on the Item card (codeunit 54), Explode BOM in the item
// journal (codeunit 246) and the BOM Register's Item Ledger / Resource Ledger (codeunits 255,
// 256)
func init() {
	functions.Register("table", gtables.BOMComponentTableName, "ExplodeBOM", functions.For(ExplodeBOM))
	functions.Register("table", gtables.ItemTableName, "CalcStandardCost", functions.For(CalcStandardCost), functions.WithDialog())
	functions.Register("table", gtables.ItemJournalLineTableName, "ExplodeBOM", functions.For(ItemJnlExplodeBOM), functions.WithDialog())
	functions.Register("table", gtables.BOMRegisterTableName, "ShowItemLedger", functions.For(BOMRegShowItemLedger), functions.WithDialog())
	functions.Register("table", gtables.BOMRegisterTableName, "ShowResLedger", functions.For(BOMRegShowResLedger), functions.WithDialog())
}

// maxBOMLevels is how many levels of bills of materials the cost calculation follows (classic
// ARRAYLEN(BOMComp))
const maxBOMLevels = 10

// ExplodeBOM (codeunit 51 BOM-Explode BOM): an item component that is a bill of materials
// itself is replaced by its own components, their quantities multiplied by the line's
func ExplodeBOM(rec *gtables.BOMComponent) (fcodeunits.Result, error) {
	err := rec.TestField("type", gtables.BOMComponent_Type.Item)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if rec.No == rec.Parent_item_no {
		return fcodeunits.Result{}, apperrors.BOMComponentOfItself()
	}
	fromBOMComp := gtables.NewBOMComponent(rec)
	fromBOMComp.SetRange("parent_item_no", rec.No)
	toBOMComp := gtables.NewBOMComponent(rec)
	toBOMComp.SetRange("parent_item_no", rec.Parent_item_no)
	noOfBOMComp := fromBOMComp.Count()
	if noOfBOMComp == 0 {
		return fcodeunits.Result{}, apperrors.BOMNotBillOfMaterials(rec.No.String())
	}
	var lineSpacing int
	toBOMComp.Parent_item_no = rec.Parent_item_no
	toBOMComp.Line_no = rec.Line_no
	if toBOMComp.Find(">") {
		lineSpacing = (toBOMComp.Line_no - rec.Line_no) / (1 + noOfBOMComp)
		if lineSpacing == 0 {
			return fcodeunits.Result{}, apperrors.BOMNoSpaceToExplode()
		}
	} else {
		lineSpacing = 10000
	}
	nextLineNo := rec.Line_no
	for found := fromBOMComp.FindSetBuffered(); found; found = fromBOMComp.Next() {
		nextLineNo = nextLineNo + lineSpacing
		toBOMComp = gtables.NewBOMComponent(rec)
		toBOMComp.TransferFields(fromBOMComp)
		toBOMComp.Parent_item_no = rec.Parent_item_no
		toBOMComp.Line_no = nextLineNo
		toBOMComp.Quantity_per = system.Round(rec.Quantity_per.Mul(fromBOMComp.Quantity_per), unitPrecision)
		toBOMComp.Position = types.Code(system.StrSubstNo(rec.Position.String(), fromBOMComp.Position))
		err = toBOMComp.InsertOrError(false)
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}
	err = rec.DeleteOrError(false)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// CalcStandardCost (codeunit 54 BOM-Calculate Cost): the standard cost of a bill of materials
// from its components — this level (the components' unit costs) or all levels (the
// sub-assemblies' standard costs calculated first) —, resources at their cost, plus the item's
// indirect cost
func CalcStandardCost(rec *gtables.Item) (fcodeunits.Result, error) {
	item := gtables.NewItem(rec)
	err := item.GetOrError(rec.No)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	item.CalcFields("bill_of_materials")
	err = item.TestField("bill_of_materials", true)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	switch system.StrMenu(i18n.GetInstance().Message("BOM_CALC_COST_LEVELS", fcodeunits.CurrentLanguage()), 1) {
	case 0:
		return fcodeunits.Result{Success: true}, nil
	case 1:
		err = CalcItemStandardCost(item, false)
	case 2:
		err = CalcItemStandardCost(item, true)
	}
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// CalcItemStandardCost is the calculation of codeunit 54 without the question: the standard
// cost of a bill of materials from this level of components or from all levels
func CalcItemStandardCost(item *gtables.Item, allLevels bool) error {
	maxLevel := 1
	if allLevels {
		maxLevel = maxBOMLevels + 1
	}
	calc := &bomCalculateCost{item: item, maxLevel: maxLevel}
	_, err := calc.updateUnitCost(item.No, 1)
	return err
}

// bomCalculateCost is the state of codeunit 54: the item it runs on and the deepest level
type bomCalculateCost struct {
	item     *gtables.Item
	maxLevel int
}

// updateUnitCost (UpdateUnitCost): a bill of materials' standard cost from its components; it
// returns the item's unit cost afterwards
func (c *bomCalculateCost) updateUnitCost(itemNo types.Code, level int) (types.Decimal, error) {
	if level > maxBOMLevels {
		return types.Decimal{}, apperrors.BOMTooManyLevels(maxBOMLevels)
	}
	unitCost := types.Decimal{}
	bomComp := gtables.NewBOMComponent(c.item)
	bomComp.SetRange("parent_item_no", itemNo)
	bomComp.SetFilter("type", "<>0")
	item2 := gtables.NewItem(c.item)
	if bomComp.FindSetBuffered() {
		for found := true; found; found = bomComp.Next() {
			switch bomComp.Type {
			case gtables.BOMComponent_Type.Item:
				if level < c.maxLevel {
					// the sub-assembly's standard cost first; its unit cost follows
					_, err := c.updateUnitCost(bomComp.No, level+1)
					if err != nil {
						return types.Decimal{}, err
					}
				}
				err := item2.GetOrError(bomComp.No)
				if err != nil {
					return types.Decimal{}, err
				}
				qtyPerUnitOfMeasure, err := tables.GetQtyPerUnitOfMeasure(item2, bomComp.Unit_of_measure_code)
				if err != nil {
					return types.Decimal{}, err
				}
				unitCost = unitCost.Add(bomComp.Quantity_per.Mul(qtyPerUnitOfMeasure).Mul(item2.Unit_cost))
			case gtables.BOMComponent_Type.Resource:
				resCost := gtables.NewResourceCost(c.item)
				resCost.Init()
				resCost.Code = bomComp.No
				resCost.Work_type_code = ""
				err := resprice.FindResCost(resCost)
				if err != nil {
					return types.Decimal{}, err
				}
				unitCost = unitCost.Add(bomComp.Quantity_per.Mul(resCost.Unit_cost))
			}
		}
		err := item2.GetOrError(itemNo)
		if err != nil {
			return types.Decimal{}, err
		}
		hundred := types.NewDecimalFromInt(100)
		one := types.NewDecimalFromInt(1)
		err = item2.Validate("standard_cost", system.Round(unitCost.Mul(one.Add(c.item.Indirect_cost_percent.Div(hundred))), unitPrecision))
		if err != nil {
			return types.Decimal{}, err
		}
		err = item2.ModifyOrError(true)
		if err != nil {
			return types.Decimal{}, err
		}
	} else {
		err := item2.GetOrError(itemNo)
		if err != nil {
			return types.Decimal{}, err
		}
	}
	return item2.Unit_cost, nil
}

// ItemJnlExplodeBOM (codeunit 246 Item Jnl.-Explode BOM): an item journal line of a bill of
// materials becomes a line per item component (the quantities multiplied by the line's); the
// line itself keeps only its description
func ItemJnlExplodeBOM(rec *gtables.ItemJournalLine) (fcodeunits.Result, error) {
	err := rec.TestField("item_no")
	if err != nil {
		return fcodeunits.Result{}, err
	}
	fromBOMComp := gtables.NewBOMComponent(rec)
	fromBOMComp.SetRange("parent_item_no", rec.Item_no)
	fromBOMComp.SetRange("type", gtables.BOMComponent_Type.Item)
	noOfBOMComp := fromBOMComp.Count()
	if noOfBOMComp == 0 {
		return fcodeunits.Result{}, apperrors.BOMNotBillOfMaterials(rec.Item_no.String())
	}
	if !system.Confirm(i18n.GetInstance().Message("BOM_EXPLODE_CONFIRM", fcodeunits.CurrentLanguage())) {
		return fcodeunits.Result{Success: true}, nil
	}

	toItemJnlLine := gtables.NewItemJournalLine(rec)
	toItemJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	toItemJnlLine.SetRange("journal_batch_name", rec.Journal_batch_name)
	toItemJnlLine.Journal_template_name = rec.Journal_template_name
	toItemJnlLine.Journal_batch_name = rec.Journal_batch_name
	toItemJnlLine.Line_no = rec.Line_no
	var lineSpacing int
	if toItemJnlLine.Find(">") {
		lineSpacing = (toItemJnlLine.Line_no - rec.Line_no) / (1 + noOfBOMComp)
		if lineSpacing == 0 {
			return fcodeunits.Result{}, apperrors.BOMNoSpaceToExplode()
		}
	} else {
		lineSpacing = 10000
	}

	// The line keeps its description only
	description := rec.Description
	toItemJnlLine = gtables.NewItemJournalLine(rec)
	err = toItemJnlLine.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "journal_batch_name": rec.Journal_batch_name, "line_no": rec.Line_no})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	toItemJnlLine.Init()
	toItemJnlLine.Description = description
	err = toItemJnlLine.ModifyOrError(false)
	if err != nil {
		return fcodeunits.Result{}, err
	}

	nextLineNo := rec.Line_no
	for found := fromBOMComp.FindSetBuffered(); found; found = fromBOMComp.Next() {
		toItemJnlLine = gtables.NewItemJournalLine(rec)
		toItemJnlLine.Init()
		toItemJnlLine.Journal_template_name = rec.Journal_template_name
		toItemJnlLine.Journal_batch_name = rec.Journal_batch_name
		toItemJnlLine.Document_no = rec.Document_no
		toItemJnlLine.Posting_date = rec.Posting_date
		toItemJnlLine.Entry_type = rec.Entry_type
		toItemJnlLine.Location_code = rec.Location_code
		toItemJnlLine.Dimension_1_code = rec.Dimension_1_code
		toItemJnlLine.Dimension_2_code = rec.Dimension_2_code
		nextLineNo = nextLineNo + lineSpacing
		toItemJnlLine.Line_no = nextLineNo
		toItemJnlLine.Drop_shipment = rec.Drop_shipment
		toItemJnlLine.Source_code = rec.Source_code
		for _, step := range []struct {
			field string
			value interface{}
		}{
			{"item_no", fromBOMComp.No},
			{"variant_code", fromBOMComp.Variant_code},
			{"unit_of_measure_code", fromBOMComp.Unit_of_measure_code},
			{"quantity", system.Round(rec.Quantity_base.Mul(fromBOMComp.Quantity_per), unitPrecision)},
		} {
			err = toItemJnlLine.Validate(step.field, step.value)
			if err != nil {
				return fcodeunits.Result{}, err
			}
		}
		toItemJnlLine.Description = fromBOMComp.Description
		err = toItemJnlLine.InsertOrError(false)
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// BOMRegShowItemLedger (codeunit 255 BOM Reg.-Show Item Ledger): the item ledger entries the
// register's BOM ledger entries point at
func BOMRegShowItemLedger(rec *gtables.BOMRegister) (fcodeunits.Result, error) {
	itemLedgEntry := gtables.NewItemLedgerEntry(rec)
	itemLedgEntry.SetFilter("entry_no", bomRegEntryFilter(rec, gtables.BOMLedgerEntry_Reference_type.Item))
	system.RunModal(38, itemLedgEntry, false)
	return fcodeunits.Result{Success: true}, nil
}

// BOMRegShowResLedger (codeunit 256 BOM Reg.-Show Res. Ledger): the resource ledger entries the
// register's BOM ledger entries point at
func BOMRegShowResLedger(rec *gtables.BOMRegister) (fcodeunits.Result, error) {
	resLedgEntry := gtables.NewResLedgerEntry(rec)
	resLedgEntry.SetFilter("entry_no", bomRegEntryFilter(rec, gtables.BOMLedgerEntry_Reference_type.Resource))
	system.RunModal(202, resLedgEntry, false)
	return fcodeunits.Result{Success: true}, nil
}

// bomRegEntryFilter is a filter on the entry numbers the register's BOM ledger entries of a
// reference type point at (classic: the entries marked, MARKEDONLY); "0" when there are none
func bomRegEntryFilter(bomReg *gtables.BOMRegister, referenceType gtables.BOMLedgerEntryReference_type) string {
	bomLedgEntry := gtables.NewBOMLedgerEntry(bomReg)
	bomLedgEntry.SetRange("entry_no", bomReg.From_entry_no, bomReg.To_entry_no)
	bomLedgEntry.SetRange("reference_type", referenceType)
	var entryNos []string
	for found := bomLedgEntry.FindSetBuffered(); found; found = bomLedgEntry.Next() {
		entryNos = append(entryNos, system.Format(bomLedgEntry.Reference_entry_no))
	}
	if len(entryNos) == 0 {
		return "0"
	}
	return strings.Join(entryNos, "|")
}
