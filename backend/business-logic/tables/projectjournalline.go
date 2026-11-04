package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/itemprice"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/noseries"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/resprice"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// projectJournalLineLogic is the business logic of table 210 Project Journal Line: a project's
// usage of a resource, an item or a G/L account before posting.
//
// Not migrated yet: the item availability warning (codeunit 311 ItemCheckAvail).
type projectJournalLineLogic struct{}

func init() {
	gtables.RegisterProjectJournalLineLogic(projectJournalLineLogic{})
	// the lookups of Appl.-to Item Entry, Serial No. and Lot No. (SelectItemEntry)
	functions.Register("table", gtables.ProjectJournalLineTableName, "SelectItemEntry", functions.ForAssistEdit(ProjectJnlLineSelectItemEntry), functions.WithAssistEdit())
}

// OnInsert: the line's template and batch exist
func (projectJournalLineLogic) OnInsert(rec *gtables.ProjectJournalLine) error {
	projectJnlTemplate := gtables.NewProjectJournalTemplate(rec)
	err := projectJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return err
	}
	projectJnlBatch := gtables.NewProjectJournalBatch(rec)
	return projectJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "name": rec.Journal_batch_name})
}

// OnValidate_Project_no: the project is not blocked and on Order; a resource line takes the
// project's price
func (projectJournalLineLogic) OnValidate_Project_no(rec *gtables.ProjectJournalLine) error {
	if rec.Project_no == "" {
		return nil
	}
	project := gtables.NewProject(rec)
	err := project.GetOrError(rec.Project_no)
	if err != nil {
		return err
	}
	err = project.TestField("blocked", false)
	if err != nil {
		return err
	}
	err = project.TestField("status", gtables.Project_Status.Order)
	if err != nil {
		return err
	}
	if (rec.Type == gtables.ProjectJournalLine_Type.Resource) && (rec.No != "") {
		return findProjectJnlResPrice(rec)
	}
	return nil
}

// OnValidate_Posting_date: the document date follows
func (projectJournalLineLogic) OnValidate_Posting_date(rec *gtables.ProjectJournalLine) error {
	return rec.Validate("document_date", rec.Posting_date)
}

// OnValidate_Type: a new type clears the number and the balancing account
func (projectJournalLineLogic) OnValidate_Type(rec *gtables.ProjectJournalLine) error {
	rec.No = ""
	rec.Balancing_acc_no = ""
	return nil
}

// OnValidate_No: the resource's, item's or G/L account's description, costs, price,
// dimensions, unit of measure and posting groups
func (projectJournalLineLogic) OnValidate_No(rec *gtables.ProjectJournalLine) error {
	rec.Variant_code = ""
	if rec.No == "" {
		return nil
	}
	switch rec.Type {
	case gtables.ProjectJournalLine_Type.Resource:
		res := gtables.NewResource(rec)
		err := res.GetOrError(rec.No)
		if err != nil {
			return err
		}
		err = res.TestField("blocked", false)
		if err != nil {
			return err
		}
		rec.Description = res.Name
		rec.Direct_unit_cost = res.Direct_unit_cost
		rec.Unit_cost = res.Unit_cost
		rec.Unit_price = res.Unit_price
		rec.Resource_group_no = res.Resource_group_no
		rec.Dimension_1_code = res.Dimension_1_code
		rec.Dimension_2_code = res.Dimension_2_code
		rec.Unit_of_measure_code = res.Unit_of_measure_code
		rec.Work_type_code = ""
		rec.Gen_prod_posting_group = res.Gen_prod_posting_group
		err = findProjectJnlResUnitCost(rec)
		if err != nil {
			return err
		}
		err = findProjectJnlResPrice(rec)
		if err != nil {
			return err
		}
	case gtables.ProjectJournalLine_Type.Item:
		item := gtables.NewItem(rec)
		err := item.GetOrError(rec.No)
		if err != nil {
			return err
		}
		rec.Description = item.Description
		rec.Unit_cost = item.Unit_cost
		rec.Unit_price = item.Unit_price
		rec.Posting_group = item.Inventory_posting_group
		rec.Direct_unit_cost = types.Decimal{}
		rec.Dimension_1_code = ""
		rec.Dimension_2_code = ""
		rec.Gen_prod_posting_group = item.Gen_prod_posting_group
		err = rec.Validate("unit_of_measure_code", item.Base_unit_of_measure)
		if err != nil {
			return err
		}
		err = findProjectJnlItemPrice(rec)
		if err != nil {
			return err
		}
	case gtables.ProjectJournalLine_Type.AccountGL:
		glAcc := gtables.NewGLAccount(rec)
		err := glAcc.GetOrError(rec.No)
		if err != nil {
			return err
		}
		err = CheckGLAcc(glAcc)
		if err != nil {
			return err
		}
		err = glAcc.TestField("direct_posting", true)
		if err != nil {
			return err
		}
		rec.Description = glAcc.Name
		rec.Dimension_1_code = glAcc.Dimension_1_code
		rec.Dimension_2_code = glAcc.Dimension_2_code
		rec.Gen_bus_posting_group = glAcc.Gen_bus_posting_group
		rec.Gen_prod_posting_group = glAcc.Gen_prod_posting_group
		rec.Unit_of_measure_code = ""
		rec.Direct_unit_cost = types.Decimal{}
		rec.Unit_cost = types.Decimal{}
		rec.Unit_price = types.Decimal{}
	}
	return rec.Validate("quantity", rec.Quantity)
}

// OnValidate_Quantity: the base quantity and the totals
func (projectJournalLineLogic) OnValidate_Quantity(rec *gtables.ProjectJournalLine) error {
	qtyBase, err := calcProjectJnlBaseQty(rec, rec.Quantity)
	if err != nil {
		return err
	}
	rec.Quantity_base = qtyBase
	rec.Total_cost = system.Round(rec.Quantity.Mul(rec.Unit_cost))
	rec.Total_price = system.Round(rec.Quantity.Mul(rec.Unit_price))
	return nil
}

// OnValidate_Unit_cost: an item at standard cost keeps its cost; the total cost and profit %
func (projectJournalLineLogic) OnValidate_Unit_cost(rec *gtables.ProjectJournalLine) error {
	if rec.Type == gtables.ProjectJournalLine_Type.Item {
		item := gtables.NewItem(rec)
		err := item.GetOrError(rec.No)
		if err != nil {
			return err
		}
		if item.Costing_method == gtables.Item_Costing_method.Standard {
			return apperrors.ItemCannotChangeWhen(gtables.ItemTableName, "unit_cost", "costing_method", item.Costing_method)
		}
	}
	updateProjectJnlUnitCost(rec)
	return nil
}

// OnValidate_Unit_price: the total price and profit %
func (projectJournalLineLogic) OnValidate_Unit_price(rec *gtables.ProjectJournalLine) error {
	updateProjectJnlUnitPrice(rec)
	return nil
}

// OnValidate_Unit_of_measure_code: an item's quantity per unit of measure, its unit cost and
// price in that unit; the quantity again
func (projectJournalLineLogic) OnValidate_Unit_of_measure_code(rec *gtables.ProjectJournalLine) error {
	if rec.Type == gtables.ProjectJournalLine_Type.Item {
		item := gtables.NewItem(rec)
		err := item.GetOrError(rec.No)
		if err != nil {
			return err
		}
		rec.Qty_per_unit_of_measure, err = GetQtyPerUnitOfMeasure(item, rec.Unit_of_measure_code)
		if err != nil {
			return err
		}
		rec.Unit_cost = system.Round(item.Unit_cost.Mul(rec.Qty_per_unit_of_measure), types.MustDecimal("0.00001"))
		err = findProjectJnlItemPrice(rec)
		if err != nil {
			return err
		}
	} else {
		rec.Qty_per_unit_of_measure = types.NewDecimalFromInt(1)
	}
	return rec.Validate("quantity", rec.Quantity)
}

// OnValidate_Location_code: a new location clears the bin
func (projectJournalLineLogic) OnValidate_Location_code(rec *gtables.ProjectJournalLine) error {
	rec.Bin_code = ""
	return nil
}

// OnValidate_Work_type_code: a resource's unit of measure of the work type (else the
// resource's), its cost and price for the work type
func (projectJournalLineLogic) OnValidate_Work_type_code(rec *gtables.ProjectJournalLine) error {
	if (rec.Type == gtables.ProjectJournalLine_Type.Resource) && (rec.No != "") {
		workType := gtables.NewWorkType(rec)
		if workType.Get(rec.Work_type_code) {
			rec.Unit_of_measure_code = workType.Unit_of_measure_code
		} else {
			res := gtables.NewResource(rec)
			err := res.GetOrError(rec.No)
			if err != nil {
				return err
			}
			rec.Unit_of_measure_code = res.Unit_of_measure_code
		}
		err := findProjectJnlResUnitCost(rec)
		if err != nil {
			return err
		}
		return findProjectJnlResPrice(rec)
	}
	return nil
}

// OnValidate_Price_group_code: an item's price for the price group
func (projectJournalLineLogic) OnValidate_Price_group_code(rec *gtables.ProjectJournalLine) error {
	if (rec.Type == gtables.ProjectJournalLine_Type.Item) && (rec.No != "") {
		return findProjectJnlItemPrice(rec)
	}
	return nil
}

// OnValidate_Profit_percent: the unit price at the profit % on the unit cost
func (projectJournalLineLogic) OnValidate_Profit_percent(rec *gtables.ProjectJournalLine) error {
	hundred := types.NewDecimalFromInt(100)
	if !rec.Profit_percent.Equal(hundred) {
		rec.Unit_price = system.Round(rec.Unit_cost.Mul(hundred).Div(hundred.Sub(rec.Profit_percent)), types.MustDecimal("0.00001"))
	} else {
		rec.Unit_price = types.Decimal{}
	}
	rec.Total_price = system.Round(rec.Quantity.Mul(rec.Unit_price))
	return nil
}

// OnValidate_Variant_code: an item's variant gives the line its description
func (projectJournalLineLogic) OnValidate_Variant_code(rec *gtables.ProjectJournalLine) error {
	if rec.Variant_code == "" {
		return nil
	}
	err := rec.TestField("type", gtables.ProjectJournalLine_Type.Item)
	if err != nil {
		return err
	}
	itemVariant := gtables.NewItemVariant(rec)
	err = itemVariant.GetOrError(map[string]interface{}{"item_no": rec.No, "code": rec.Variant_code})
	if err != nil {
		return err
	}
	rec.Description = itemVariant.Description
	return nil
}

// OnValidate_Bin_code: a bin needs the location
func (projectJournalLineLogic) OnValidate_Bin_code(rec *gtables.ProjectJournalLine) error {
	if rec.Bin_code == "" {
		return nil
	}
	return rec.TestField("location_code")
}

// OnValidate_Quantity_base: only in the base unit; the quantity follows
func (projectJournalLineLogic) OnValidate_Quantity_base(rec *gtables.ProjectJournalLine) error {
	err := rec.TestField("qty_per_unit_of_measure", types.NewDecimalFromInt(1))
	if err != nil {
		return err
	}
	return rec.Validate("quantity", rec.Quantity_base)
}

// OnValidate_Balancing_acc_no: only on G/L account lines
func (projectJournalLineLogic) OnValidate_Balancing_acc_no(rec *gtables.ProjectJournalLine) error {
	return rec.TestField("type", gtables.ProjectJournalLine_Type.AccountGL)
}

// updateProjectJnlUnitCost: Total Cost = Quantity × Unit Cost, the profit % again
func updateProjectJnlUnitCost(rec *gtables.ProjectJournalLine) {
	rec.Total_cost = system.Round(rec.Quantity.Mul(rec.Unit_cost))
	calcProjectJnlProfitPct(rec)
}

// updateProjectJnlUnitPrice: Total Price = Quantity × Unit Price, the profit % again
func updateProjectJnlUnitPrice(rec *gtables.ProjectJournalLine) {
	rec.Total_price = system.Round(rec.Quantity.Mul(rec.Unit_price))
	calcProjectJnlProfitPct(rec)
}

// calcProjectJnlBaseQty (CalcBaseQty): the quantity in the base unit
func calcProjectJnlBaseQty(rec *gtables.ProjectJournalLine, qty types.Decimal) (types.Decimal, error) {
	err := rec.TestField("qty_per_unit_of_measure")
	if err != nil {
		return types.Decimal{}, err
	}
	return system.Round(qty.Mul(rec.Qty_per_unit_of_measure), types.MustDecimal("0.00001")), nil
}

// calcProjectJnlProfitPct (CalcProfitPct): the profit of the unit price over the unit cost
func calcProjectJnlProfitPct(rec *gtables.ProjectJournalLine) {
	if !rec.Unit_price.IsZero() {
		rec.Profit_percent = system.Round(types.NewDecimalFromInt(100).Mul(rec.Unit_price.Sub(rec.Unit_cost)).Div(rec.Unit_price), types.MustDecimal("0.00001"))
	} else {
		rec.Profit_percent = types.Decimal{}
	}
}

// findProjectJnlResPrice (FindResPrice): the resource's price for the project and work type
// (codeunit 221)
func findProjectJnlResPrice(rec *gtables.ProjectJournalLine) error {
	resPrice := gtables.NewResourcePrice(rec)
	resPrice.Init()
	resPrice.Project_no = rec.Project_no
	resPrice.Code = rec.No
	resPrice.Work_type_code = rec.Work_type_code
	err := resprice.FindResPrice(resPrice)
	if err != nil {
		return err
	}
	rec.Unit_price = resPrice.Unit_price
	updateProjectJnlUnitPrice(rec)
	return nil
}

// findProjectJnlResUnitCost (FindResUnitCost): the resource's cost for the work type (codeunit
// 220)
func findProjectJnlResUnitCost(rec *gtables.ProjectJournalLine) error {
	resCost := gtables.NewResourceCost(rec)
	resCost.Init()
	resCost.Code = rec.No
	resCost.Work_type_code = rec.Work_type_code
	err := resprice.FindResCost(resCost)
	if err != nil {
		return err
	}
	rec.Direct_unit_cost = resCost.Direct_unit_cost
	rec.Unit_cost = resCost.Unit_cost
	updateProjectJnlUnitCost(rec)
	return nil
}

// findProjectJnlItemPrice (FindItemPrice): the item's price for the price group, unit of measure
// and posting date (codeunit 52) — a price including tax is refused; a price for any unit is
// per base unit
func findProjectJnlItemPrice(rec *gtables.ProjectJournalLine) error {
	itemPrice := gtables.NewItemPrice(rec)
	itemPrice.Init()
	itemPrice.Item_no = rec.No
	itemPrice.Price_group_code = rec.Price_group_code
	itemPrice.Unit_of_measure_code = rec.Unit_of_measure_code
	itemPrice.Starting_date = rec.Posting_date
	err := itemprice.FindPrice(itemPrice)
	if err != nil {
		return err
	}
	err = itemPrice.TestField("price_includes_tax", false)
	if err != nil {
		return err
	}
	rec.Unit_price = itemPrice.Unit_price
	if itemPrice.Unit_of_measure_code == "" {
		rec.Unit_price = system.Round(rec.Unit_price.Mul(rec.Qty_per_unit_of_measure), types.MustDecimal("0.00001"))
	}
	updateProjectJnlUnitPrice(rec)
	return nil
}

// ProjectJnlLineSelectItemEntry (SelectItemEntry): the item's open entries of the sign the line
// takes; the picked one's entry, serial and lot number, location and bin
func ProjectJnlLineSelectItemEntry(rec *gtables.ProjectJournalLine) (bool, error) {
	err := rec.TestField("type", gtables.ProjectJournalLine_Type.Item)
	if err != nil {
		return false, err
	}
	itemLedgEntry := gtables.NewItemLedgerEntry(rec)
	itemLedgEntry.SetCurrentKey("item_no", "variant_code", "open")
	itemLedgEntry.SetRange("item_no", rec.No)
	itemLedgEntry.SetRange("open", true)
	if !rec.Quantity.IsZero() {
		itemLedgEntry.SetRange("positive", rec.Quantity.IsPositive())
	}
	if system.RunModal(itemLedgerEntriesPageID, itemLedgEntry, true) == system.ActionLookupOK {
		rec.Appl_to_item_entry = itemLedgEntry.Entry_no
		rec.Serial_no = itemLedgEntry.Serial_no
		rec.Location_code = itemLedgEntry.Location_code
		rec.Bin_code = itemLedgEntry.Bin_code
		rec.Lot_no = itemLedgEntry.Lot_no
		return true, nil
	}
	return false, nil
}

// ProjectJnlLineEmptyLine (EmptyLine): a line without number and quantity
func ProjectJnlLineEmptyLine(rec *gtables.ProjectJournalLine) bool {
	return rec.No == "" && rec.Quantity.IsZero()
}

// SetUpNewProjectJnlLine (SetUpNewLine): a new line takes the posting date and document no. of
// the last line — the first line of a batch the work date and the next number of the batch's
// series —, the last line's recurring method, entry type, type and project, the template's
// source code and the batch's reason code and posting no. series
func SetUpNewProjectJnlLine(rec, lastProjectJnlLine *gtables.ProjectJournalLine) error {
	projectJnlTemplate := gtables.NewProjectJournalTemplate(rec)
	err := projectJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return err
	}
	projectJnlBatch := gtables.NewProjectJournalBatch(rec)
	err = projectJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "name": rec.Journal_batch_name})
	if err != nil {
		return err
	}
	projectJnlLine := gtables.NewProjectJournalLine(rec)
	projectJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	projectJnlLine.SetRange("journal_batch_name", rec.Journal_batch_name)
	if projectJnlLine.FindFirst() {
		rec.Posting_date = lastProjectJnlLine.Posting_date
		rec.Document_date = lastProjectJnlLine.Posting_date
		rec.Document_no = lastProjectJnlLine.Document_no
	} else {
		rec.Posting_date = system.WorkDate()
		rec.Document_date = system.WorkDate()
		if projectJnlBatch.No_series != "" {
			noSeriesMgt := noseries.New(rec)
			rec.Document_no = noSeriesMgt.TryGetNextNo(projectJnlBatch.No_series, rec.Posting_date)
		}
	}
	rec.Recurring_method = lastProjectJnlLine.Recurring_method
	rec.Entry_type = lastProjectJnlLine.Entry_type
	rec.Type = lastProjectJnlLine.Type
	rec.Project_no = lastProjectJnlLine.Project_no
	rec.Source_code = projectJnlTemplate.Source_code
	rec.Reason_code = projectJnlBatch.Reason_code
	rec.Posting_no_series = projectJnlBatch.Posting_no_series
	return nil
}
