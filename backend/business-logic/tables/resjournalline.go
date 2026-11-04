package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/noseries"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/resprice"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// resJournalLineLogic is the business logic of table 207 Res. Journal Line.
type resJournalLineLogic struct{}

func init() { gtables.RegisterResJournalLineLogic(resJournalLineLogic{}) }

// OnInsert: the line's template and batch exist
func (resJournalLineLogic) OnInsert(rec *gtables.ResJournalLine) error {
	resJnlTemplate := gtables.NewResJournalTemplate(rec)
	err := resJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return err
	}
	resJnlBatch := gtables.NewResJournalBatch(rec)
	return resJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "name": rec.Journal_batch_name})
}

// OnValidate_Posting_date: the document date follows
func (resJournalLineLogic) OnValidate_Posting_date(rec *gtables.ResJournalLine) error {
	return rec.Validate("document_date", rec.Posting_date)
}

// OnValidate_Resource_no: the resource's name, unit of measure, group, dimension 1, dimension 2 and
// product posting group; its cost and price for no work type
func (resJournalLineLogic) OnValidate_Resource_no(rec *gtables.ResJournalLine) error {
	if rec.Resource_no == "" {
		return nil
	}
	res := gtables.NewResource(rec)
	err := res.GetOrError(rec.Resource_no)
	if err != nil {
		return err
	}
	err = res.TestField("blocked", false)
	if err != nil {
		return err
	}
	rec.Description = res.Name
	rec.Unit_of_measure_code = res.Unit_of_measure_code
	rec.Direct_unit_cost = res.Direct_unit_cost
	rec.Unit_cost = res.Unit_cost
	rec.Unit_price = res.Unit_price
	rec.Dimension_1_code = res.Dimension_1_code
	rec.Dimension_2_code = res.Dimension_2_code
	rec.Resource_group_no = res.Resource_group_no
	rec.Work_type_code = ""
	rec.Gen_prod_posting_group = res.Gen_prod_posting_group
	err = findResJnlUnitCost(rec)
	if err != nil {
		return err
	}
	err = findResJnlPrice(rec)
	if err != nil {
		return err
	}
	return rec.Validate("quantity", rec.Quantity)
}

// OnValidate_Project_no: the resource's price for the project
func (resJournalLineLogic) OnValidate_Project_no(rec *gtables.ResJournalLine) error {
	return findResJnlPrice(rec)
}

// OnValidate_Work_type_code: the work type's unit of measure (else the resource's), the cost
// and price of the work type
func (resJournalLineLogic) OnValidate_Work_type_code(rec *gtables.ResJournalLine) error {
	if rec.Resource_no != "" {
		workType := gtables.NewWorkType(rec)
		if workType.Get(rec.Work_type_code) {
			rec.Unit_of_measure_code = workType.Unit_of_measure_code
		} else {
			res := gtables.NewResource(rec)
			err := res.GetOrError(rec.Resource_no)
			if err != nil {
				return err
			}
			rec.Unit_of_measure_code = res.Unit_of_measure_code
		}
		err := findResJnlUnitCost(rec)
		if err != nil {
			return err
		}
		return findResJnlPrice(rec)
	}
	return nil
}

// OnValidate_Quantity: the totals again
func (resJournalLineLogic) OnValidate_Quantity(rec *gtables.ResJournalLine) error {
	err := rec.Validate("unit_cost", rec.Unit_cost)
	if err != nil {
		return err
	}
	return rec.Validate("unit_price", rec.Unit_price)
}

// OnValidate_Unit_cost: Total Cost = Quantity × Unit Cost
func (resJournalLineLogic) OnValidate_Unit_cost(rec *gtables.ResJournalLine) error {
	rec.Total_cost = rec.Quantity.Mul(rec.Unit_cost)
	return nil
}

// OnValidate_Total_cost: the unit cost from the total
func (resJournalLineLogic) OnValidate_Total_cost(rec *gtables.ResJournalLine) error {
	err := rec.TestField("quantity")
	if err != nil {
		return err
	}
	rec.Unit_cost = system.Round(rec.Total_cost.Div(rec.Quantity), types.MustDecimal("0.00001"))
	return nil
}

// OnValidate_Unit_price: Total Price = Quantity × Unit Price
func (resJournalLineLogic) OnValidate_Unit_price(rec *gtables.ResJournalLine) error {
	rec.Total_price = rec.Quantity.Mul(rec.Unit_price)
	return nil
}

// OnValidate_Total_price: the unit price from the total
func (resJournalLineLogic) OnValidate_Total_price(rec *gtables.ResJournalLine) error {
	err := rec.TestField("quantity")
	if err != nil {
		return err
	}
	rec.Unit_price = system.Round(rec.Total_price.Div(rec.Quantity), types.MustDecimal("0.00001"))
	return nil
}

// findResJnlUnitCost (FindResUnitCost): the resource's cost for the work type (codeunit 220)
func findResJnlUnitCost(rec *gtables.ResJournalLine) error {
	resCost := gtables.NewResourceCost(rec)
	resCost.Init()
	resCost.Code = rec.Resource_no
	resCost.Work_type_code = rec.Work_type_code
	err := resprice.FindResCost(resCost)
	if err != nil {
		return err
	}
	rec.Direct_unit_cost = resCost.Direct_unit_cost
	rec.Unit_cost = resCost.Unit_cost
	return rec.Validate("unit_cost", rec.Unit_cost)
}

// findResJnlPrice (FindResPrice): the resource's price for the project and work type (codeunit 221)
func findResJnlPrice(rec *gtables.ResJournalLine) error {
	resPrice := gtables.NewResourcePrice(rec)
	resPrice.Init()
	resPrice.Project_no = rec.Project_no
	resPrice.Code = rec.Resource_no
	resPrice.Work_type_code = rec.Work_type_code
	err := resprice.FindResPrice(resPrice)
	if err != nil {
		return err
	}
	rec.Unit_price = resPrice.Unit_price
	return rec.Validate("unit_price", rec.Unit_price)
}

// ResJnlLineEmptyLine (EmptyLine): a line without resource and quantity
func ResJnlLineEmptyLine(rec *gtables.ResJournalLine) bool {
	return rec.Resource_no == "" && rec.Quantity.IsZero()
}

// SetUpNewResJnlLine (SetUpNewLine): a new line takes the posting date and document no. of the
// last line — the first line of a batch the work date and the next number of the batch's
// series —, the template's source code and the batch's reason code and posting no. series
func SetUpNewResJnlLine(rec, lastResJnlLine *gtables.ResJournalLine) error {
	resJnlTemplate := gtables.NewResJournalTemplate(rec)
	err := resJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return err
	}
	resJnlBatch := gtables.NewResJournalBatch(rec)
	err = resJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "name": rec.Journal_batch_name})
	if err != nil {
		return err
	}
	resJnlLine := gtables.NewResJournalLine(rec)
	resJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	resJnlLine.SetRange("journal_batch_name", rec.Journal_batch_name)
	if resJnlLine.FindFirst() {
		rec.Posting_date = lastResJnlLine.Posting_date
		rec.Document_date = lastResJnlLine.Posting_date
		rec.Document_no = lastResJnlLine.Document_no
	} else {
		rec.Posting_date = system.WorkDate()
		rec.Document_date = system.WorkDate()
		if resJnlBatch.No_series != "" {
			noSeriesMgt := noseries.New(rec)
			rec.Document_no = noSeriesMgt.TryGetNextNo(resJnlBatch.No_series, rec.Posting_date)
		}
	}
	rec.Recurring_method = lastResJnlLine.Recurring_method
	rec.Source_code = resJnlTemplate.Source_code
	rec.Reason_code = resJnlBatch.Reason_code
	rec.Posting_no_series = resJnlBatch.Posting_no_series
	return nil
}
