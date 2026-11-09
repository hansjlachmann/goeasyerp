package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/noseries"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// resourceLogic is the business logic of table 156 Resource.
//
// Not migrated yet (their tables come later): OnDelete's check of outstanding sales order
// lines and its deletion of extended texts.
// Resource Group No. asks no confirmation (no dialogs in table triggers yet).
type resourceLogic struct{}

func init() {
	gtables.RegisterResourceLogic(resourceLogic{})
	// the No. field's "…" (Resource Card): pick another number series
	functions.Register("table", gtables.ResourceTableName, "AssistEdit", functions.ForAssistEdit(AssistEditResource), functions.WithAssistEdit())
}

// OnInsert: a resource without No. gets the next number of the Resource Nos. series of the
// Resources Setup
func (resourceLogic) OnInsert(rec *gtables.Resource) error {
	if rec.No == "" {
		resSetup := gtables.GetResourcesSetup(rec)
		err := resSetup.TestField("resource_nos")
		if err != nil {
			return err
		}
		noSeriesMgt := noseries.New(rec)
		xRec := rec.XRec()
		err = noSeriesMgt.InitSeries(resSetup.Resource_nos, xRec.No_series, types.Date{}, &rec.No, &rec.No_series)
		if err != nil {
			return err
		}
	}
	return nil
}

// OnModify sets Last Date Modified
func (resourceLogic) OnModify(rec *gtables.Resource) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnDelete deletes the resource's capacity entries, costs, prices and comment lines
func (resourceLogic) OnDelete(rec *gtables.Resource) error {
	err := MoveResEntries(rec)
	if err != nil {
		return err
	}

	resCapacityEntry := gtables.NewResCapacityEntry(rec)
	resCapacityEntry.SetCurrentKey("resource_no")
	resCapacityEntry.SetRange("resource_no", rec.No)
	resCapacityEntry.DeleteAll()

	resCost := gtables.NewResourceCost(rec)
	resCost.SetRange("type", gtables.ResourceCost_Type.Resource)
	resCost.SetRange("code", rec.No)
	resCost.DeleteAll()

	resPrice := gtables.NewResourcePrice(rec)
	resPrice.SetRange("type", gtables.ResourcePrice_Type.Resource)
	resPrice.SetRange("code", rec.No)
	resPrice.DeleteAll()

	DeleteComments(rec, gtables.CommentLine_Table_name.Resource, rec.No)
	return DeleteExtendedTexts(rec, gtables.ExtendedTextHeader_Table_name.Resource, rec.No)
}

// OnRename sets Last Date Modified
func (resourceLogic) OnRename(rec *gtables.Resource) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnValidate_No: a number typed by hand must be allowed by the Resource Nos. series (Manual
// Nos.) and takes the resource out of its series; a new resource saved with the number its
// series just issued (AssistEdit) keeps that series
func (resourceLogic) OnValidate_No(rec *gtables.Resource) error {
	xRec := rec.XRec()
	if rec.No != xRec.No {
		resSetup := gtables.GetResourcesSetup(rec)
		noSeriesMgt := noseries.New(rec)
		series, issued := noSeriesMgt.IssuedBy(resSetup.Resource_nos, rec.No)
		if issued && xRec.No == "" {
			rec.No_series = series
		} else {
			err := noSeriesMgt.TestManual(resSetup.Resource_nos)
			if err != nil {
				return err
			}
			rec.No_series = ""
		}
	}
	return nil
}

// OnValidate_Name: Search Name follows the name while it was blank or the old name
func (resourceLogic) OnValidate_Name(rec *gtables.Resource) error {
	xRec := rec.XRec()
	if string(rec.Search_name) == system.UpperCase(xRec.Name.String()) || rec.Search_name == "" {
		rec.Search_name = types.NewCode(system.CopyStr(rec.Name.String(), 1, 50))
	}
	return nil
}

// OnValidate_Resource_group_no: the resource's capacity entries and project budget entries move
// to the new group
func (resourceLogic) OnValidate_Resource_group_no(rec *gtables.Resource) error {
	xRec := rec.XRec()
	if rec.Resource_group_no == xRec.Resource_group_no {
		return nil
	}

	// Resource Capacity Entries
	resCapacityEntry := gtables.NewResCapacityEntry(rec)
	resCapacityEntry.SetCurrentKey("resource_no")
	resCapacityEntry.SetRange("resource_no", rec.No)
	resCapacityEntry.ModifyAll("resource_group_no", rec.Resource_group_no)

	// Project Budget Entries
	projectBudgetEntry := gtables.NewProjectBudgetEntry(rec)
	projectBudgetEntry.SetCurrentKey("project_status", "type", "no")
	projectBudgetEntry.SetRange("type", gtables.ProjectBudgetEntry_Type.Resource)
	projectBudgetEntry.SetRange("no", rec.No)
	projectBudgetEntry.ModifyAll("resource_group_no", rec.Resource_group_no)
	return nil
}

// OnValidate_Direct_unit_cost: the unit cost again
func (resourceLogic) OnValidate_Direct_unit_cost(rec *gtables.Resource) error {
	return rec.Validate("indirect_cost_percent", rec.Indirect_cost_percent)
}

// OnValidate_Indirect_cost_percent: Unit Cost = Direct Unit Cost plus the indirect cost
func (resourceLogic) OnValidate_Indirect_cost_percent(rec *gtables.Resource) error {
	hundred := types.NewDecimalFromInt(100)
	one := types.NewDecimalFromInt(1)
	return rec.Validate("unit_cost", system.Round(rec.Direct_unit_cost.Mul(one.Add(rec.Indirect_cost_percent.Div(hundred))), types.MustDecimal("0.01")))
}

// OnValidate_Unit_cost: the price/profit calculation again
func (resourceLogic) OnValidate_Unit_cost(rec *gtables.Resource) error {
	return validateResPriceProfit(rec)
}

// OnValidate_Profit_percent: the price/profit calculation again
func (resourceLogic) OnValidate_Profit_percent(rec *gtables.Resource) error {
	return validateResPriceProfit(rec)
}

// OnValidate_Price_profit_calculation: Profit % from the price and cost, or the price from the
// cost and Profit %
func (resourceLogic) OnValidate_Price_profit_calculation(rec *gtables.Resource) error {
	hundred := types.NewDecimalFromInt(100)
	one := types.NewDecimalFromInt(1)
	switch rec.Price_profit_calculation {
	case gtables.Resource_Price_profit_calculation.ProfitPriceCost:
		if !rec.Unit_price.IsZero() {
			rec.Profit_percent = system.Round(hundred.Mul(one.Sub(rec.Unit_cost.Div(rec.Unit_price))), types.MustDecimal("0.00001"))
		} else {
			rec.Profit_percent = types.Decimal{}
		}
	case gtables.Resource_Price_profit_calculation.PriceCostProfit:
		if rec.Profit_percent.LessThan(hundred) {
			rec.Unit_price = system.Round(rec.Unit_cost.Div(one.Sub(rec.Profit_percent.Div(hundred))), types.MustDecimal("0.00001"))
		}
	}
	return nil
}

// OnValidate_Unit_price: the price/profit calculation again
func (resourceLogic) OnValidate_Unit_price(rec *gtables.Resource) error {
	return validateResPriceProfit(rec)
}

func validateResPriceProfit(rec *gtables.Resource) error {
	return rec.Validate("price_profit_calculation", rec.Price_profit_calculation)
}

// OnValidate_Gen_prod_posting_group fills in the group's default Tax Prod. Posting Group
func (resourceLogic) OnValidate_Gen_prod_posting_group(rec *gtables.Resource) error {
	xRec := rec.XRec()
	if xRec.Gen_prod_posting_group != rec.Gen_prod_posting_group {
		genProdPostingGroup := gtables.NewGenProductPostingGroup(rec)
		autoInsert, err := ValidateTaxProdPostingGroup(genProdPostingGroup, rec.Gen_prod_posting_group)
		if err != nil {
			return err
		}
		if autoInsert {
			err = rec.Validate("tax_prod_posting_group", genProdPostingGroup.Def_tax_prod_posting_group)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// OnValidate_Post_code: a known post code gives the city
func (resourceLogic) OnValidate_Post_code(rec *gtables.Resource) error {
	postCode := gtables.NewPostCode(rec)
	if postCode.Get(rec.Post_code) {
		rec.City = postCode.City
	}
	return nil
}

// AssistEditResource lets the user pick another number series for the resource — the
// Resource Nos. series or one related to it — and gives the resource that series' next number
// (AssistEdit of the No. field); false when the user cancelled
func AssistEditResource(rec *gtables.Resource) (bool, error) {
	resSetup := gtables.GetResourcesSetup(rec)
	err := resSetup.TestField("resource_nos")
	if err != nil {
		return false, err
	}
	noSeriesMgt := noseries.New(rec)
	xRec := rec.XRec()
	noSeries := rec.No_series
	picked, err := noSeriesMgt.SelectSeries(resSetup.Resource_nos, xRec.No_series, &noSeries)
	if err != nil {
		return false, err
	}
	if picked {
		err = noSeriesMgt.SetSeries(&rec.No)
		if err != nil {
			return false, err
		}
		rec.No_series = noSeries
	}
	return picked, nil
}

// MoveResEntries (codeunit 361 MoveEntries): a resource with ledger entries in a fiscal year
// that is not closed cannot be deleted; its older entries lose the resource no.
func MoveResEntries(res *gtables.Resource) error {
	resLedgEntry := gtables.NewResLedgerEntry(res)
	resLedgEntry.Reset()
	resLedgEntry.SetCurrentKey("resource_no", "posting_date")
	resLedgEntry.SetRange("resource_no", res.No)
	accountingPeriod := gtables.NewAccountingPeriod(res)
	accountingPeriod.SetRange("closed", false)
	if accountingPeriod.FindFirst() {
		resLedgEntry.SetFilter("posting_date", ">="+accountingPeriod.Starting_date.String())
	}
	if resLedgEntry.FindFirst() {
		return apperrors.DeleteHasEntries(gtables.ResourceTableName, res.No.String())
	}

	resLedgEntry.Reset()
	resLedgEntry.SetCurrentKey("resource_no")
	resLedgEntry.SetRange("resource_no", res.No)
	resLedgEntry.ModifyAll("resource_no", types.Code(""))
	return nil
}
