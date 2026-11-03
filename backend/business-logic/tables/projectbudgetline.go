package tables

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/itemprice"
	"github.com/hansjlachmann/openerp/backend/business-logic/resprice"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// projectBudgetLineLogic is the business logic of table 168 Project Budget Line: what a project
// plans to use (a resource, an item, a G/L account or a resource group, by phase, task and
// step) with its costs and price; its quantity, total cost and total price are the sums of its
// budget entries (SetProjectBudgetLineQuantity enters them).
type projectBudgetLineLogic struct{}

func init() { gtables.RegisterProjectBudgetLineLogic(projectBudgetLineLogic{}) }

// OnInsert: the line starts on the project's Starting Date
func (projectBudgetLineLogic) OnInsert(rec *gtables.ProjectBudgetLine) error {
	project := gtables.NewProject(rec)
	err := project.GetOrError(rec.Project_no)
	if err != nil {
		return err
	}
	rec.Starting_date = project.Starting_date
	return nil
}

// OnDelete deletes the line's budget entries
func (projectBudgetLineLogic) OnDelete(rec *gtables.ProjectBudgetLine) error {
	projectBudgetEntry := gtables.NewProjectBudgetEntry(rec)
	setBudgetLineEntryFilters(projectBudgetEntry, rec)
	projectBudgetEntry.DeleteAll()
	return nil
}

// OnRename: a budget line keeps its key — delete it and enter a new line
func (projectBudgetLineLogic) OnRename(rec *gtables.ProjectBudgetLine) error {
	return apperrors.ProjectBudgetLineRename(gtables.ProjectBudgetLineTableName)
}

// OnValidate_Project_no: the project must not be blocked
func (projectBudgetLineLogic) OnValidate_Project_no(rec *gtables.ProjectBudgetLine) error {
	if rec.Project_no == "" {
		return nil
	}
	project := gtables.NewProject(rec)
	err := project.GetOrError(rec.Project_no)
	if err != nil {
		return err
	}
	return project.TestField("blocked", false)
}

// OnValidate_Phase_code: the line takes the phase's description
func (projectBudgetLineLogic) OnValidate_Phase_code(rec *gtables.ProjectBudgetLine) error {
	if rec.Phase_code != "" {
		phase := gtables.NewPhase(rec)
		err := phase.GetOrError(rec.Phase_code)
		if err != nil {
			return err
		}
		rec.Description = phase.Description
	}
	return nil
}

// OnValidate_Task_code: the line takes the task's description
func (projectBudgetLineLogic) OnValidate_Task_code(rec *gtables.ProjectBudgetLine) error {
	if rec.Task_code != "" {
		task := gtables.NewTask(rec)
		err := task.GetOrError(rec.Task_code)
		if err != nil {
			return err
		}
		rec.Description = task.Description
	}
	return nil
}

// OnValidate_Step_code: the line takes the step's description
func (projectBudgetLineLogic) OnValidate_Step_code(rec *gtables.ProjectBudgetLine) error {
	if rec.Step_code != "" {
		step := gtables.NewStep(rec)
		err := step.GetOrError(rec.Step_code)
		if err != nil {
			return err
		}
		rec.Description = step.Description
	}
	return nil
}

// OnValidate_No: the line takes the resource's, item's, account's or resource group's
// description, costs, price and dimensions
func (projectBudgetLineLogic) OnValidate_No(rec *gtables.ProjectBudgetLine) error {
	if rec.No == "" {
		return nil
	}
	switch rec.Type {
	case gtables.ProjectBudgetLine_Type.Resource:
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
		rec.Dimension_1_code = res.Dimension_1_code
		rec.Dimension_2_code = res.Dimension_2_code
		err = findBudgetLineResUnitCost(rec)
		if err != nil {
			return err
		}
		err = findBudgetLineResPrice(rec)
		if err != nil {
			return err
		}
	case gtables.ProjectBudgetLine_Type.Item:
		item := gtables.NewItem(rec)
		err := item.GetOrError(rec.No)
		if err != nil {
			return err
		}
		rec.Description = item.Description
		rec.Direct_unit_cost = item.Last_direct_cost
		rec.Unit_cost = item.Unit_cost
		rec.Unit_price = item.Unit_price
		rec.Dimension_2_code = ""
		rec.Dimension_1_code = ""
		err = findBudgetLineItemPrice(rec)
		if err != nil {
			return err
		}
	case gtables.ProjectBudgetLine_Type.AccountGL:
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
	case gtables.ProjectBudgetLine_Type.GroupResource:
		resGr := gtables.NewResourceGroup(rec)
		err := resGr.GetOrError(rec.No)
		if err != nil {
			return err
		}
		rec.Description = resGr.Name
		resCost := gtables.NewResourceCost(rec)
		if resCost.Get(map[string]interface{}{"type": gtables.ResourceCost_Type.GroupResource, "code": rec.No, "work_type_code": ""}) &&
			(resCost.Cost_type == gtables.ResourceCost_Cost_type.Fixed) {
			rec.Direct_unit_cost = resCost.Direct_unit_cost
			rec.Unit_cost = resCost.Unit_cost
		}
		// the classic project's own price first: resource prices per project come with the
		// project ledger
		resPrice := gtables.NewResourcePrice(rec)
		if resPrice.Get(map[string]interface{}{"type": gtables.ResourcePrice_Type.GroupResource, "code": rec.No, "work_type_code": "", "currency_code": ""}) {
			rec.Unit_price = resPrice.Unit_price
		}
	}
	return nil
}

// OnValidate_Starting_date: budget entries all on one date move to the new starting date
func (projectBudgetLineLogic) OnValidate_Starting_date(rec *gtables.ProjectBudgetLine) error {
	xRec := rec.XRec()
	if rec.Starting_date == xRec.Starting_date {
		return nil
	}
	projectBudgetEntry := gtables.NewProjectBudgetEntry(rec)
	setBudgetLineEntryFilters(projectBudgetEntry, rec)
	if projectBudgetEntry.FindLast() {
		lastBudgetEntryDate := projectBudgetEntry.Date
		projectBudgetEntry.FindFirst()
		if lastBudgetEntryDate == projectBudgetEntry.Date {
			for found := projectBudgetEntry.FindSetBuffered(); found; found = projectBudgetEntry.Next() { // modified in the loop: read all first
				projectBudgetEntry.Date = rec.Starting_date
				err := projectBudgetEntry.ModifyOrError(false)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// OnValidate_Unit_cost: the budget entries' total cost follows
func (projectBudgetLineLogic) OnValidate_Unit_cost(rec *gtables.ProjectBudgetLine) error {
	xRec := rec.XRec()
	if !rec.Unit_cost.Equal(xRec.Unit_cost) {
		return updateProjectBudgetEntry(rec)
	}
	return nil
}

// OnValidate_Unit_price: the budget entries' total price follows
func (projectBudgetLineLogic) OnValidate_Unit_price(rec *gtables.ProjectBudgetLine) error {
	xRec := rec.XRec()
	if !rec.Unit_price.Equal(xRec.Unit_price) {
		return updateProjectBudgetEntry(rec)
	}
	return nil
}

// OnValidate_Price_group_code: an item's price for the price group
func (projectBudgetLineLogic) OnValidate_Price_group_code(rec *gtables.ProjectBudgetLine) error {
	if (rec.Type == gtables.ProjectBudgetLine_Type.Item) && (rec.No != "") {
		return findBudgetLineItemPrice(rec)
	}
	return nil
}

// OnValidate_Variant_code: an item's variant gives the line its description
func (projectBudgetLineLogic) OnValidate_Variant_code(rec *gtables.ProjectBudgetLine) error {
	if rec.Variant_code == "" {
		return nil
	}
	err := rec.TestField("type", gtables.ProjectBudgetLine_Type.Item)
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

// setBudgetLineEntryFilters: the budget entries of a budget line
func setBudgetLineEntryFilters(projectBudgetEntry *gtables.ProjectBudgetEntry, rec *gtables.ProjectBudgetLine) {
	projectBudgetEntry.SetCurrentKey("project_no", "phase_code", "task_code", "step_code", "type", "no", "variant_code", "date")
	projectBudgetEntry.SetRange("project_no", rec.Project_no)
	projectBudgetEntry.SetRange("phase_code", rec.Phase_code)
	projectBudgetEntry.SetRange("task_code", rec.Task_code)
	projectBudgetEntry.SetRange("step_code", rec.Step_code)
	projectBudgetEntry.SetRange("type", rec.Type)
	projectBudgetEntry.SetRange("no", rec.No)
	projectBudgetEntry.SetRange("variant_code", rec.Variant_code)
}

// updateProjectBudgetEntry (UpdateJobBudgetEntry): the line's budget entries get their total
// cost and price at the line's unit cost and price
func updateProjectBudgetEntry(rec *gtables.ProjectBudgetLine) error {
	projectBudgetEntry := gtables.NewProjectBudgetEntry(rec)
	setBudgetLineEntryFilters(projectBudgetEntry, rec)
	for found := projectBudgetEntry.FindSetBuffered(); found; found = projectBudgetEntry.Next() { // modified in the loop: read all first
		projectBudgetEntry.Total_cost = system.Round(projectBudgetEntry.Quantity.Mul(rec.Unit_cost))
		projectBudgetEntry.Total_price = system.Round(projectBudgetEntry.Quantity.Mul(rec.Unit_price))
		err := projectBudgetEntry.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	rec.CalcFields("total_cost", "total_price")
	return nil
}

// findBudgetLineResUnitCost (FindResUnitCost): the resource's cost (codeunit 220)
func findBudgetLineResUnitCost(rec *gtables.ProjectBudgetLine) error {
	resCost := gtables.NewResourceCost(rec)
	resCost.Init()
	resCost.Code = rec.No
	err := resprice.FindResCost(resCost)
	if err != nil {
		return err
	}
	rec.Direct_unit_cost = resCost.Direct_unit_cost
	rec.Unit_cost = resCost.Unit_cost
	return rec.Validate("unit_cost", rec.Unit_cost)
}

// findBudgetLineResPrice (FindResPrice): the resource's price (codeunit 221)
func findBudgetLineResPrice(rec *gtables.ProjectBudgetLine) error {
	resPrice := gtables.NewResourcePrice(rec)
	resPrice.Init()
	resPrice.Code = rec.No
	resPrice.Work_type_code = ""
	err := resprice.FindResPrice(resPrice)
	if err != nil {
		return err
	}
	rec.Unit_price = resPrice.Unit_price
	return rec.Validate("unit_price", rec.Unit_price)
}

// findBudgetLineItemPrice (FindItemPrice): the item's price for the price group on the line's
// starting date (codeunit 52); a price including tax is refused
func findBudgetLineItemPrice(rec *gtables.ProjectBudgetLine) error {
	itemPrice := gtables.NewItemPrice(rec)
	itemPrice.Init()
	itemPrice.Item_no = rec.No
	itemPrice.Price_group_code = rec.Price_group_code
	itemPrice.Starting_date = rec.Starting_date
	err := itemprice.FindPrice(itemPrice)
	if err != nil {
		return err
	}
	err = itemPrice.TestField("price_includes_tax", false)
	if err != nil {
		return err
	}
	rec.Unit_price = itemPrice.Unit_price
	return rec.Validate("unit_price", rec.Unit_price)
}

// SetProjectBudgetLineQuantity enters the budget line's Quantity in its Date Filter (classic: a
// value typed into the FlowField): a budget entry with the difference to the quantity budgeted
// now, on the Date Filter's first date (blank: the line's Starting Date)
func SetProjectBudgetLineQuantity(rec *gtables.ProjectBudgetLine, quantity types.Decimal) error {
	date := types.Date{}
	if rec.Date_filter != "" {
		var ok bool
		date, ok = dateFilterMin(rec.Date_filter)
		if !ok {
			return apperrors.BudgetNotOneValue(gtables.ProjectBudgetLineTableName, "date_filter")
		}
	}
	rec.CalcFields("quantity")
	if quantity.Equal(rec.Quantity) {
		return nil
	}
	projectBudgetEntry := gtables.NewProjectBudgetEntry(rec)
	projectBudgetEntry.LockTable()
	entryNo := 1
	if projectBudgetEntry.FindLast() {
		entryNo = projectBudgetEntry.Entry_no + 1
	}
	projectBudgetEntry.Init()
	projectBudgetEntry.Entry_no = entryNo
	err := projectBudgetEntry.Validate("project_no", rec.Project_no)
	if err != nil {
		return err
	}
	projectBudgetEntry.Type = gtables.ProjectBudgetEntryType(rec.Type)
	err = projectBudgetEntry.Validate("no", rec.No)
	if err != nil {
		return err
	}
	projectBudgetEntry.Phase_code = rec.Phase_code
	projectBudgetEntry.Task_code = rec.Task_code
	projectBudgetEntry.Step_code = rec.Step_code
	projectBudgetEntry.Variant_code = rec.Variant_code
	projectBudgetEntry.Date = date
	err = projectBudgetEntry.Validate("quantity", quantity.Sub(rec.Quantity))
	if err != nil {
		return err
	}
	return projectBudgetEntry.InsertOrError(true)
}
