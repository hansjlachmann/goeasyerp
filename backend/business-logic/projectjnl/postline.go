package projectjnl

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/resjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

var unitPrecision = types.MustDecimal("0.00001")

// ProjectJnlPostLine is codeunit 202 Project Jnl.-Post Line: it posts a project journal line as
// a project ledger entry (a sale with the opposite sign) and keeps the Project Register. Unless
// the line posts the project entry only (sales and purchase invoices post their own amounts), a
// resource line is posted as resource usage (codeunit 212), an item line as a negative
// adjustment (codeunit 22) and a G/L account line against its balancing account (codeunit 12).
// The new entry is applied to the project's open entries of the other sign: by Applies-to ID,
// Applies-to Entry or, with the project's Application Method Apply to Oldest, the oldest one
// of the same type and number. Open it on the posting's transaction; one instance per posting
// keeps the entry numbers and the register.
type ProjectJnlPostLine struct {
	src ftables.Source

	projectJnlLine      *gtables.ProjectJournalLine
	projectLedgEntry    *gtables.ProjectLedgerEntry
	project             *gtables.Project
	projectReg          *gtables.ProjectRegister
	genPostingSetup     *gtables.GeneralPostingSetup
	projectJnlCheckLine ProjectJnlCheckLine
	resJnlPostLine      *resjnl.ResJnlPostLine
	itemJnlPostLine     *itemjnl.ItemJnlPostLine
	genJnlPostLine      *genjnl.GenJnlPostLine
	nextEntryNo         int
}

// NewProjectJnlPostLine opens the posting codeunit in the posting's database (its transaction)
func NewProjectJnlPostLine(src ftables.Source) *ProjectJnlPostLine {
	return &ProjectJnlPostLine{src: src}
}

// Run checks and posts a journal line (OnRun)
func (p *ProjectJnlPostLine) Run(projectJnlLine *gtables.ProjectJournalLine) error {
	p.projectJnlLine = gtables.NewProjectJournalLine(p.src)
	p.projectJnlLine.Copy(projectJnlLine)
	return p.code()
}

// GetProjectReg returns the Project Register of the posting (No. 0 before the first entry)
func (p *ProjectJnlPostLine) GetProjectReg() *gtables.ProjectRegister {
	if p.projectReg == nil {
		return gtables.NewProjectRegister(p.src)
	}
	return p.projectReg
}

func (p *ProjectJnlPostLine) code() error {
	projectJnlLine := p.projectJnlLine
	if tables.ProjectJnlLineEmptyLine(projectJnlLine) {
		return nil
	}

	err := p.projectJnlCheckLine.RunCheck(projectJnlLine)
	if err != nil {
		return err
	}

	if p.nextEntryNo == 0 {
		projectLedgEntry := gtables.NewProjectLedgerEntry(p.src)
		projectLedgEntry.LockTable()
		if projectLedgEntry.FindLast() {
			p.nextEntryNo = projectLedgEntry.Entry_no
		}
		p.nextEntryNo = p.nextEntryNo + 1
	}

	if projectJnlLine.Document_date.IsZero() {
		projectJnlLine.Document_date = projectJnlLine.Posting_date
	}

	if p.projectReg == nil || p.projectReg.No == 0 {
		p.projectReg = gtables.NewProjectRegister(p.src)
		p.projectReg.LockTable()
		if !p.projectReg.FindLast() || p.projectReg.To_entry_no != 0 {
			no := p.projectReg.No
			p.projectReg.Init()
			p.projectReg.No = no + 1
			p.projectReg.From_entry_no = p.nextEntryNo
			p.projectReg.To_entry_no = p.nextEntryNo
			p.projectReg.Creation_date = system.Today()
			p.projectReg.Source_code = projectJnlLine.Source_code
			p.projectReg.Journal_batch_name = projectJnlLine.Journal_batch_name
			p.projectReg.User_id = types.Code(system.UserId())
			err = p.projectReg.InsertOrError(false)
			if err != nil {
				return err
			}
		}
	}
	p.projectReg.To_entry_no = p.nextEntryNo
	err = p.projectReg.ModifyOrError(false)
	if err != nil {
		return err
	}

	p.project = gtables.NewProject(p.src)
	err = p.project.GetOrError(projectJnlLine.Project_no)
	if err != nil {
		return err
	}
	err = p.project.TestField("blocked", false)
	if err != nil {
		return err
	}
	err = p.project.TestField("status", gtables.Project_Status.Order)
	if err != nil {
		return err
	}
	err = p.project.TestField("project_posting_group")
	if err != nil {
		return err
	}
	if projectJnlLine.Type == gtables.ProjectJournalLine_Type.AccountGL {
		projectJnlLine.Posting_group = projectJnlLine.No
	}

	projectJnlLine2 := gtables.NewProjectJournalLine(p.src)
	projectJnlLine2.Copy(projectJnlLine)
	err = p.addCurrencyTotals(projectJnlLine2)
	if err != nil {
		return err
	}

	if !projectJnlLine.Post_project_entry_only {
		switch projectJnlLine.Type {
		case gtables.ProjectJournalLine_Type.Resource:
			err = p.postResource(projectJnlLine2)
		case gtables.ProjectJournalLine_Type.Item:
			err = p.postItem(projectJnlLine2)
		case gtables.ProjectJournalLine_Type.AccountGL:
			err = p.postGLAccount(projectJnlLine2)
		}
		if err != nil {
			return err
		}
	}

	if projectJnlLine.Post_project_entry_only {
		if p.genPostingSetup == nil ||
			p.genPostingSetup.Gen_bus_posting_group != projectJnlLine.Gen_bus_posting_group ||
			p.genPostingSetup.Gen_prod_posting_group != projectJnlLine.Gen_prod_posting_group {
			p.genPostingSetup = gtables.NewGeneralPostingSetup(p.src)
			err = p.genPostingSetup.GetOrError(map[string]interface{}{"gen_bus_posting_group": projectJnlLine.Gen_bus_posting_group, "gen_prod_posting_group": projectJnlLine.Gen_prod_posting_group})
			if err != nil {
				return err
			}
		}
	}

	projectLedgEntry := gtables.NewProjectLedgerEntry(p.src)
	projectLedgEntry.Init()
	projectLedgEntry.Project_no = projectJnlLine2.Project_no
	projectLedgEntry.Posting_date = projectJnlLine2.Posting_date
	projectLedgEntry.Document_date = projectJnlLine2.Document_date
	projectLedgEntry.Document_no = projectJnlLine2.Document_no
	projectLedgEntry.External_document_no = projectJnlLine2.External_document_no
	projectLedgEntry.Type = gtables.ProjectLedgerEntryType(projectJnlLine2.Type)
	projectLedgEntry.No = projectJnlLine2.No
	projectLedgEntry.Description = projectJnlLine2.Description
	projectLedgEntry.Quantity = projectJnlLine2.Quantity
	projectLedgEntry.Direct_unit_cost = projectJnlLine2.Direct_unit_cost
	projectLedgEntry.Unit_cost = projectJnlLine2.Unit_cost
	projectLedgEntry.Total_cost = projectJnlLine2.Total_cost
	projectLedgEntry.Unit_price = projectJnlLine2.Unit_price
	projectLedgEntry.Total_price = projectJnlLine2.Total_price
	projectLedgEntry.Additional_currency_total_cost = projectJnlLine2.Source_currency_total_cost
	projectLedgEntry.Add_currency_total_price = projectJnlLine2.Source_currency_total_price
	projectLedgEntry.Resource_group_no = projectJnlLine2.Resource_group_no
	projectLedgEntry.Unit_of_measure_code = projectJnlLine2.Unit_of_measure_code
	projectLedgEntry.Location_code = projectJnlLine2.Location_code
	projectLedgEntry.Chargeable = projectJnlLine2.Chargeable
	projectLedgEntry.Project_posting_group = projectJnlLine2.Posting_group
	projectLedgEntry.Dimension_1_code = projectJnlLine2.Dimension_1_code
	projectLedgEntry.Dimension_2_code = projectJnlLine2.Dimension_2_code
	projectLedgEntry.Work_type_code = projectJnlLine2.Work_type_code
	projectLedgEntry.Price_group_code = projectJnlLine2.Price_group_code
	projectLedgEntry.Source_code = projectJnlLine2.Source_code
	projectLedgEntry.Phase_code = projectJnlLine2.Phase_code
	projectLedgEntry.Task_code = projectJnlLine2.Task_code
	projectLedgEntry.Step_code = projectJnlLine2.Step_code
	projectLedgEntry.Entry_type = gtables.ProjectLedgerEntryEntry_type(projectJnlLine2.Entry_type)
	projectLedgEntry.Gen_bus_posting_group = projectJnlLine2.Gen_bus_posting_group
	projectLedgEntry.Gen_prod_posting_group = projectJnlLine2.Gen_prod_posting_group
	projectLedgEntry.Transaction_type = projectJnlLine2.Transaction_type
	projectLedgEntry.Transport_method = projectJnlLine2.Transport_method
	projectLedgEntry.Entry_exit_point = projectJnlLine2.Entry_exit_point
	projectLedgEntry.Area = projectJnlLine2.Area
	projectLedgEntry.Transaction_specification = projectJnlLine2.Transaction_specification
	projectLedgEntry.Country_code = projectJnlLine2.Country_code
	projectLedgEntry.Journal_batch_name = projectJnlLine2.Journal_batch_name
	projectLedgEntry.Reason_code = projectJnlLine2.Reason_code
	projectLedgEntry.Lot_no = projectJnlLine2.Lot_no
	projectLedgEntry.Variant_code = projectJnlLine2.Variant_code
	projectLedgEntry.Bin_code = projectJnlLine2.Bin_code
	projectLedgEntry.Qty_per_unit_of_measure = projectJnlLine2.Qty_per_unit_of_measure
	projectLedgEntry.Remaining_amount = projectLedgEntry.Total_price
	if projectLedgEntry.Entry_type == gtables.ProjectLedgerEntry_Entry_type.Sale {
		projectLedgEntry.Chargeable = true
		projectLedgEntry.Quantity = projectLedgEntry.Quantity.Neg()
		projectLedgEntry.Total_cost = projectLedgEntry.Total_cost.Neg()
		projectLedgEntry.Total_price = projectLedgEntry.Total_price.Neg()
		projectLedgEntry.Remaining_amount = projectLedgEntry.Remaining_amount.Neg()
		projectLedgEntry.Additional_currency_total_cost = projectLedgEntry.Additional_currency_total_cost.Neg()
		projectLedgEntry.Add_currency_total_price = projectLedgEntry.Add_currency_total_price.Neg()
	}
	if !projectLedgEntry.Quantity.IsZero() {
		projectLedgEntry.Unit_cost = system.Round(projectLedgEntry.Total_cost.Div(projectLedgEntry.Quantity), unitPrecision)
		projectLedgEntry.Unit_price = system.Round(projectLedgEntry.Total_price.Div(projectLedgEntry.Quantity), unitPrecision)
	}
	if !projectLedgEntry.Chargeable {
		projectLedgEntry.Remaining_amount = types.Decimal{}
	}
	projectLedgEntry.User_id = types.Code(system.UserId())
	projectLedgEntry.Positive = projectLedgEntry.Total_price.IsPositive()
	projectLedgEntry.Open = !projectLedgEntry.Remaining_amount.IsZero()
	projectLedgEntry.Entry_no = p.nextEntryNo
	projectLedgEntry.No_series = projectJnlLine2.Posting_no_series
	p.projectLedgEntry = projectLedgEntry
	err = p.applyProjectEntry(projectJnlLine.Apply_and_close_project)
	if err != nil {
		return err
	}
	if projectJnlLine.Apply_and_close_project {
		projectLedgEntry.Remaining_amount = types.Decimal{}
		projectLedgEntry.Open = false
	}
	err = projectLedgEntry.InsertOrError(false)
	if err != nil {
		return err
	}
	p.nextEntryNo = p.nextEntryNo + 1
	return nil
}

// addCurrencyTotals: the line's totals in the additional reporting currency at the posting
// date's rate (a line in that currency keeps its own); none without one
func (p *ProjectJnlPostLine) addCurrencyTotals(projectJnlLine2 *gtables.ProjectJournalLine) error {
	glSetup := gtables.GetGeneralLedgerSetup(p.src)
	if glSetup.Additional_reporting_currency != "" {
		if projectJnlLine2.Source_currency_code != glSetup.Additional_reporting_currency {
			currency := gtables.NewCurrency(p.src)
			err := currency.GetOrError(glSetup.Additional_reporting_currency)
			if err != nil {
				return err
			}
			err = currency.TestField("amount_rounding_precision")
			if err != nil {
				return err
			}
			currencyExchRate := tables.ExchangeRates(p.src)
			rate, err := currencyExchRate.ExchangeRate(projectJnlLine2.Posting_date, glSetup.Additional_reporting_currency)
			if err != nil {
				return err
			}
			totalCost, err := currencyExchRate.ExchangeAmtLCYToFCY(projectJnlLine2.Posting_date, glSetup.Additional_reporting_currency, projectJnlLine2.Total_cost, rate)
			if err != nil {
				return err
			}
			totalPrice, err := currencyExchRate.ExchangeAmtLCYToFCY(projectJnlLine2.Posting_date, glSetup.Additional_reporting_currency, projectJnlLine2.Total_price, rate)
			if err != nil {
				return err
			}
			projectJnlLine2.Source_currency_total_cost = system.Round(totalCost, currency.Amount_rounding_precision)
			projectJnlLine2.Source_currency_total_price = system.Round(totalPrice, currency.Amount_rounding_precision)
		}
	} else {
		projectJnlLine2.Source_currency_total_cost = types.Decimal{}
		projectJnlLine2.Source_currency_total_price = types.Decimal{}
	}
	return nil
}

// postResource: the resource's usage (or sale) through codeunit 212
func (p *ProjectJnlPostLine) postResource(projectJnlLine2 *gtables.ProjectJournalLine) error {
	resJnlLine := gtables.NewResJournalLine(p.src)
	resJnlLine.Init()
	resJnlLine.Entry_type = gtables.ResJournalLineEntry_type(projectJnlLine2.Entry_type)
	resJnlLine.Document_no = projectJnlLine2.Document_no
	resJnlLine.External_document_no = projectJnlLine2.External_document_no
	resJnlLine.Posting_date = projectJnlLine2.Posting_date
	resJnlLine.Document_date = projectJnlLine2.Document_date
	resJnlLine.Resource_no = projectJnlLine2.No
	resJnlLine.Description = projectJnlLine2.Description
	resJnlLine.Work_type_code = projectJnlLine2.Work_type_code
	resJnlLine.Project_no = projectJnlLine2.Project_no
	resJnlLine.Quantity = projectJnlLine2.Quantity
	resJnlLine.Direct_unit_cost = projectJnlLine2.Direct_unit_cost
	resJnlLine.Unit_cost = projectJnlLine2.Unit_cost
	resJnlLine.Total_cost = projectJnlLine2.Total_cost
	resJnlLine.Unit_price = projectJnlLine2.Unit_price
	resJnlLine.Total_price = projectJnlLine2.Total_price
	resJnlLine.Dimension_1_code = projectJnlLine2.Dimension_1_code
	resJnlLine.Dimension_2_code = projectJnlLine2.Dimension_2_code
	resJnlLine.Chargeable = projectJnlLine2.Chargeable
	resJnlLine.Unit_of_measure_code = projectJnlLine2.Unit_of_measure_code
	resJnlLine.Source_code = p.projectJnlLine.Source_code
	resJnlLine.Gen_bus_posting_group = p.projectJnlLine.Gen_bus_posting_group
	resJnlLine.Gen_prod_posting_group = p.projectJnlLine.Gen_prod_posting_group
	resJnlLine.Posting_no_series = p.projectJnlLine.Posting_no_series
	if p.resJnlPostLine == nil {
		p.resJnlPostLine = resjnl.NewResJnlPostLine(p.src)
	}
	err := p.resJnlPostLine.Run(resJnlLine)
	if err != nil {
		return err
	}
	res := gtables.NewResource(p.src)
	err = res.GetOrError(resJnlLine.Resource_no)
	if err != nil {
		return err
	}
	projectJnlLine2.Resource_group_no = res.Resource_group_no
	return nil
}

// postItem: the item used on the project, a negative adjustment through codeunit 22 (the
// project as its source)
func (p *ProjectJnlPostLine) postItem(projectJnlLine2 *gtables.ProjectJournalLine) error {
	itemJnlLine := gtables.NewItemJournalLine(p.src)
	itemJnlLine.Init()
	itemJnlLine.Item_no = projectJnlLine2.No
	itemJnlLine.Posting_date = projectJnlLine2.Posting_date
	itemJnlLine.Document_date = projectJnlLine2.Document_date
	itemJnlLine.Source_no = projectJnlLine2.Project_no
	itemJnlLine.Document_no = projectJnlLine2.Document_no
	itemJnlLine.External_document_no = projectJnlLine2.External_document_no
	itemJnlLine.Description = projectJnlLine2.Description
	itemJnlLine.Location_code = projectJnlLine2.Location_code
	itemJnlLine.Quantity = projectJnlLine2.Quantity
	itemJnlLine.Quantity_base = projectJnlLine2.Quantity_base
	itemJnlLine.Invoiced_quantity = projectJnlLine2.Quantity
	itemJnlLine.Invoiced_qty_base = projectJnlLine2.Quantity_base
	itemJnlLine.Unit_cost = projectJnlLine2.Unit_cost
	itemJnlLine.Applies_to_entry = projectJnlLine2.Appl_to_item_entry
	itemJnlLine.Dimension_1_code = projectJnlLine2.Dimension_1_code
	itemJnlLine.Dimension_2_code = projectJnlLine2.Dimension_2_code
	itemJnlLine.Transaction_type = projectJnlLine2.Transaction_type
	itemJnlLine.Transport_method = projectJnlLine2.Transport_method
	itemJnlLine.Entry_exit_point = projectJnlLine2.Entry_exit_point
	itemJnlLine.Area = projectJnlLine2.Area
	itemJnlLine.Transaction_specification = projectJnlLine2.Transaction_specification
	itemJnlLine.Country_code = projectJnlLine2.Country_code
	itemJnlLine.Unit_amount = projectJnlLine2.Unit_price
	itemJnlLine.Amount = projectJnlLine2.Total_price
	itemJnlLine.Entry_type = gtables.ItemJournalLine_Entry_type.NegativeAdjmt
	itemJnlLine.Source_code = p.projectJnlLine.Source_code
	itemJnlLine.Gen_bus_posting_group = p.projectJnlLine.Gen_bus_posting_group
	itemJnlLine.Gen_prod_posting_group = p.projectJnlLine.Gen_prod_posting_group
	itemJnlLine.Serial_no = p.projectJnlLine.Serial_no
	itemJnlLine.Posting_no_series = p.projectJnlLine.Posting_no_series
	itemJnlLine.Lot_no = projectJnlLine2.Lot_no
	itemJnlLine.Variant_code = projectJnlLine2.Variant_code
	itemJnlLine.Bin_code = projectJnlLine2.Bin_code
	itemJnlLine.Unit_of_measure_code = projectJnlLine2.Unit_of_measure_code
	itemJnlLine.Qty_per_unit_of_measure = projectJnlLine2.Qty_per_unit_of_measure
	if p.itemJnlPostLine == nil {
		p.itemJnlPostLine = itemjnl.NewItemJnlPostLine(p.src)
	}
	err := p.itemJnlPostLine.Run(itemJnlLine)
	if err != nil {
		return err
	}
	item := gtables.NewItem(p.src)
	err = item.GetOrError(itemJnlLine.Item_no)
	if err != nil {
		return err
	}
	projectJnlLine2.Posting_group = item.Inventory_posting_group
	return nil
}

// postGLAccount: a G/L account line with a balancing account posts the total cost between the
// two accounts through codeunit 12 (without one only the project entry is written)
func (p *ProjectJnlPostLine) postGLAccount(projectJnlLine2 *gtables.ProjectJournalLine) error {
	if projectJnlLine2.Balancing_acc_no == "" {
		return nil
	}
	glAcc := gtables.NewGLAccount(p.src)
	err := glAcc.GetOrError(projectJnlLine2.No)
	if err != nil {
		return err
	}
	err = glAcc.TestField("direct_posting", true)
	if err != nil {
		return err
	}
	err = glAcc.GetOrError(projectJnlLine2.Balancing_acc_no)
	if err != nil {
		return err
	}
	err = glAcc.TestField("direct_posting", true)
	if err != nil {
		return err
	}
	genJnlLine := gtables.NewGenJournalLine(p.src)
	genJnlLine.Init()
	genJnlLine.Posting_date = projectJnlLine2.Posting_date
	genJnlLine.Account_type = gtables.GenJournalLine_Account_type.GLAccount
	err = genJnlLine.Validate("account_no", projectJnlLine2.No)
	if err != nil {
		return err
	}
	genJnlLine.Bal_account_type = gtables.GenJournalLine_Bal_account_type.GLAccount
	err = genJnlLine.Validate("bal_account_no", projectJnlLine2.Balancing_acc_no)
	if err != nil {
		return err
	}
	genJnlLine.Document_no = projectJnlLine2.Document_no
	genJnlLine.Description = projectJnlLine2.Description
	genJnlLine.System_created_entry = true
	err = genJnlLine.Validate("amount", projectJnlLine2.Total_cost)
	if err != nil {
		return err
	}
	genJnlLine.Dimension_1_code = projectJnlLine2.Dimension_1_code
	genJnlLine.Dimension_2_code = projectJnlLine2.Dimension_2_code
	genJnlLine.Source_code = p.projectJnlLine.Source_code
	genJnlLine.Project_no = projectJnlLine2.Project_no
	genJnlLine.Quantity = projectJnlLine2.Quantity
	genJnlLine.Reason_code = projectJnlLine2.Reason_code
	genJnlLine.Document_date = projectJnlLine2.Document_date
	genJnlLine.External_document_no = projectJnlLine2.External_document_no
	genJnlLine.Posting_no_series = projectJnlLine2.Posting_no_series
	genJnlLine.Gen_bus_posting_group = projectJnlLine2.Gen_bus_posting_group
	genJnlLine.Gen_prod_posting_group = projectJnlLine2.Gen_prod_posting_group
	if p.genJnlPostLine == nil {
		p.genJnlPostLine = genjnl.NewGenJnlPostLine(p.src)
	}
	return p.genJnlPostLine.Run(genJnlLine)
}

// applyProjectEntry (ApplyJobEntry): the new entry closes the open entries of the other sign
// it applies to — by Applies-to ID, Applies-to Entry or, with Apply to Oldest, the project's
// oldest open entry of the same type and number; with Apply and Close (closeProjectEntries) the
// old entries are closed whatever the new entry's amount
func (p *ProjectJnlPostLine) applyProjectEntry(closeProjectEntries bool) error {
	projectJnlLine := p.projectJnlLine
	projectLedgEntry := p.projectLedgEntry
	oldProjectEntry := gtables.NewProjectLedgerEntry(p.src)
	if projectJnlLine.Applies_to_id != "" {
		oldProjectEntry.SetCurrentKey("applies_to_id")
		oldProjectEntry.SetRange("applies_to_id", projectJnlLine.Applies_to_id)
		if !oldProjectEntry.FindSetBuffered() {
			return nil
		}
	} else {
		if projectJnlLine.Applies_to_entry != 0 {
			if !oldProjectEntry.Get(projectJnlLine.Applies_to_entry) {
				return apperrors.ProjectApplyToEntryMissing(gtables.ProjectJournalLineTableName)
			}
		} else {
			if p.project.Application_method == gtables.Project_Application_method.ApplyToOldest {
				oldProjectEntry.SetCurrentKey("project_no", "open", "positive", "posting_date")
				oldProjectEntry.SetRange("project_no", projectLedgEntry.Project_no)
				oldProjectEntry.SetRange("open", true)
				oldProjectEntry.SetRange("positive", !projectLedgEntry.Positive)
				oldProjectEntry.SetRange("type", projectLedgEntry.Type)
				if projectLedgEntry.Type != gtables.ProjectLedgerEntry_Type.AccountGL {
					oldProjectEntry.SetRange("no", projectLedgEntry.No)
				}
				if !oldProjectEntry.FindFirst() {
					return nil
				}
			} else {
				return nil
			}
		}
	}

	for projectLedgEntry.Open {
		err := oldProjectEntry.TestField("project_no", projectLedgEntry.Project_no)
		if err != nil {
			return err
		}
		err = oldProjectEntry.TestField("type", projectLedgEntry.Type)
		if err != nil {
			return err
		}
		if projectLedgEntry.Type != gtables.ProjectLedgerEntry_Type.AccountGL {
			err = oldProjectEntry.TestField("no", projectLedgEntry.No)
			if err != nil {
				return err
			}
		}
		err = oldProjectEntry.TestField("open", true)
		if err != nil {
			return err
		}
		if oldProjectEntry.Positive == projectLedgEntry.Positive {
			return apperrors.ProjectApplyOppositeSign(gtables.ProjectLedgerEntryTableName)
		}
		var appliedAmount types.Decimal
		if closeProjectEntries {
			appliedAmount = oldProjectEntry.Remaining_amount.Neg()
		} else {
			if projectLedgEntry.Remaining_amount.Abs().LessThan(oldProjectEntry.Remaining_amount.Abs()) {
				appliedAmount = projectLedgEntry.Remaining_amount
			} else {
				appliedAmount = oldProjectEntry.Remaining_amount.Neg()
			}
		}
		oldProjectEntry.Remaining_amount = oldProjectEntry.Remaining_amount.Add(appliedAmount)
		oldProjectEntry.Open = !oldProjectEntry.Remaining_amount.IsZero()
		if !oldProjectEntry.Open {
			oldProjectEntry.Closed_by_entry_no = projectLedgEntry.Entry_no
			oldProjectEntry.Closed_at_date = projectLedgEntry.Posting_date
			oldProjectEntry.Closed_by_amount = appliedAmount.Neg()
			oldProjectEntry.Applies_to_id = ""
		}
		err = oldProjectEntry.ModifyOrError(false)
		if err != nil {
			return err
		}
		if !closeProjectEntries {
			projectLedgEntry.Remaining_amount = projectLedgEntry.Remaining_amount.Sub(appliedAmount)
			projectLedgEntry.Open = !projectLedgEntry.Remaining_amount.IsZero()
		}
		if !projectLedgEntry.Open && oldProjectEntry.Open {
			projectLedgEntry.Closed_by_entry_no = oldProjectEntry.Entry_no
			projectLedgEntry.Closed_at_date = projectLedgEntry.Posting_date
			projectLedgEntry.Closed_by_amount = appliedAmount
		}
		if projectJnlLine.Applies_to_id != "" {
			if !oldProjectEntry.Next() {
				return nil
			}
		} else {
			if projectJnlLine.Applies_to_entry != 0 {
				return nil
			}
			if !oldProjectEntry.FindFirst() {
				return nil
			}
		}
	}
	return nil
}
