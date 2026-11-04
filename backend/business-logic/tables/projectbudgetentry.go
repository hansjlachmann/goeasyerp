package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// projectBudgetEntryLogic is the business logic of table 211 Project Budget Entry: a budget
// line's quantity on a date, with its total cost and price at the line's unit cost and price
type projectBudgetEntryLogic struct{}

func init() { gtables.RegisterProjectBudgetEntryLogic(projectBudgetEntryLogic{}) }

// OnInsert: the user who entered it
func (projectBudgetEntryLogic) OnInsert(rec *gtables.ProjectBudgetEntry) error {
	rec.User_id = types.NewCode(system.UserId())
	return nil
}

// OnValidate_Project_no: the entry takes the project's status
func (projectBudgetEntryLogic) OnValidate_Project_no(rec *gtables.ProjectBudgetEntry) error {
	project := gtables.NewProject(rec)
	err := project.GetOrError(rec.Project_no)
	if err != nil {
		return err
	}
	rec.Project_status = gtables.ProjectBudgetEntryProject_status(project.Status)
	return nil
}

// OnValidate_No: a resource's group, or the group itself
func (projectBudgetEntryLogic) OnValidate_No(rec *gtables.ProjectBudgetEntry) error {
	if rec.No == "" {
		return nil
	}
	if rec.Type == gtables.ProjectBudgetEntry_Type.Resource {
		res := gtables.NewResource(rec)
		err := res.GetOrError(rec.No)
		if err != nil {
			return err
		}
		rec.Resource_group_no = res.Resource_group_no
	} else {
		if rec.Type == gtables.ProjectBudgetEntry_Type.GroupResource {
			rec.Resource_group_no = rec.No
		}
	}
	return nil
}

// OnValidate_Resource_group_no: an entry of a resource group
func (projectBudgetEntryLogic) OnValidate_Resource_group_no(rec *gtables.ProjectBudgetEntry) error {
	rec.Type = gtables.ProjectBudgetEntry_Type.GroupResource
	rec.No = rec.Resource_group_no
	return nil
}

// OnValidate_Quantity: the entry's budget line — made from the resource or resource group
// when there is none — gives its date (blank: the line's Starting Date), total cost and price
func (projectBudgetEntryLogic) OnValidate_Quantity(rec *gtables.ProjectBudgetEntry) error {
	projectBudgetLine := gtables.NewProjectBudgetLine(rec)
	if !projectBudgetLine.Get(map[string]interface{}{"project_no": rec.Project_no, "phase_code": rec.Phase_code, "task_code": rec.Task_code,
		"step_code": rec.Step_code, "type": gtables.ProjectBudgetLineType(rec.Type), "no": rec.No, "variant_code": rec.Variant_code}) {
		projectBudgetLine.Init()
		projectBudgetLine.Project_no = rec.Project_no
		projectBudgetLine.Phase_code = rec.Phase_code
		projectBudgetLine.Task_code = rec.Task_code
		projectBudgetLine.Step_code = rec.Step_code
		projectBudgetLine.Type = gtables.ProjectBudgetLineType(rec.Type)
		projectBudgetLine.No = rec.No
		projectBudgetLine.Variant_code = rec.Variant_code
		switch rec.Type {
		case gtables.ProjectBudgetEntry_Type.Resource:
			res := gtables.NewResource(rec)
			err := res.GetOrError(rec.No)
			if err != nil {
				return err
			}
			projectBudgetLine.Description = res.Name
			projectBudgetLine.Direct_unit_cost = res.Direct_unit_cost
			projectBudgetLine.Unit_cost = res.Unit_cost
			projectBudgetLine.Unit_price = res.Unit_price
			projectBudgetLine.Dimension_1_code = res.Dimension_1_code
			projectBudgetLine.Dimension_2_code = res.Dimension_2_code
			err = res.TestField("blocked", false)
			if err != nil {
				return err
			}
		case gtables.ProjectBudgetEntry_Type.GroupResource:
			resGr := gtables.NewResourceGroup(rec)
			err := resGr.GetOrError(rec.No)
			if err != nil {
				return err
			}
			projectBudgetLine.Description = resGr.Name
		}
		err := projectBudgetLine.InsertOrError(true)
		if err != nil {
			return err
		}
	}
	if rec.Date.IsZero() {
		rec.Date = projectBudgetLine.Starting_date
	}
	if rec.Date.IsZero() {
		return apperrors.ProjectBudgetEntryNoDate(gtables.ProjectBudgetEntryTableName)
	}
	rec.Total_cost = system.Round(rec.Quantity.Mul(projectBudgetLine.Unit_cost))
	rec.Total_price = system.Round(rec.Quantity.Mul(projectBudgetLine.Unit_price))
	return nil
}
