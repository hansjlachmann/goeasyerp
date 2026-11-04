package reports

import (
	"strconv"

	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 294 Get Project Usage (Get Project Usage on a sales invoice): the open usage of the
// invoice's project (or the projects filtered) billed to the invoice's customer becomes invoice
// lines — per project, phase, task, step, type and number the remaining amounts summed by unit of
// measure, work type and dimensions (the classic Job Posting Buffer); each line's Project
// Applies-to ID is the first entry's number with the sign of its amount, and the entries it
// invoices are marked with it, so posting the invoice applies the sale to them.
const GetProjectUsageID = 294

func init() {
	Register(GetProjectUsageID, "get-project-usage", func() Report { return &getProjectUsage{} })
}

// projectUsageKey: what the usage is summed by (Job Posting Buffer's key)
type projectUsageKey struct {
	projectNo               types.Code
	lineType                gtables.ProjectLedgerEntryType
	no                      types.Code
	unitOfMeasure, workType types.Code
	dimension1, dimension2  types.Code
}

// projectUsageBuffer: a buffer line — the usage summed, the Applies-to ID (the first entry's
// number) and the phase, task and step of its group
type projectUsageBuffer struct {
	key                             projectUsageKey
	appliesToID                     string
	quantity, totalCost, totalPrice types.Decimal
	phaseCode, taskCode, stepCode   types.Code
}

type getProjectUsage struct {
	salesHeader *gtables.SalesHeader
	lastLineNo  int
	buffer      []*projectUsageBuffer
	group       string // the entries' group (project, phase, task, step, type, no — GroupTotalFields)
	project     *gtables.Project
	inserted    int
}

func (r *getProjectUsage) Definition() *Definition {
	return &Definition{
		ContextTable: gtables.SalesHeaderTableName,
		Transaction:  true,
		OnPreReport:  r.onPreReport,
		DataItems: []*DataItem{{
			Name: "Project_Ledger_Entry", Table: gtables.ProjectLedgerEntryTableName,
			Key: []string{"related_to_budget", "project_no", "entry_type", "phase_code", "task_code", "step_code", "type", "no", "posting_date"},
			TableView: []ftables.FieldFilter{
				{Field: "entry_type", Expression: "0"}, // Usage
				{Field: "open", Expression: "true"},
			},
			RequestFilterFields: []string{"project_no", "posting_date"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem: Trigger(func(projectLedgEntry *gtables.ProjectLedgerEntry, run *Run) error {
				run.Refresh() // the invoice shows its new lines
				return r.finalize(run)
			}),
		}},
	}
}

// onPreReport: the invoice it runs on (SetCurrentSalesLine) and its last line
func (r *getProjectUsage) onPreReport(run *Run) error {
	salesHeader, ok := run.Context().(*gtables.SalesHeader)
	if !ok || salesHeader.No == "" {
		return system.Error(i18n.GetInstance().Message("GET_PROJECT_USAGE_NO_DOCUMENT", fcodeunits.CurrentLanguage()))
	}
	r.salesHeader = gtables.NewSalesHeader(run)
	err := r.salesHeader.GetOrError(map[string]interface{}{"document_type": salesHeader.Document_type, "no": salesHeader.No})
	if err != nil {
		return err
	}
	salesLine := gtables.NewSalesLine(run)
	ftables.LockTable(run.DB, run.DBType, run.Company, gtables.SalesLineTableName)
	salesLine.SetRange("document_type", r.salesHeader.Document_type)
	salesLine.SetRange("document_no", r.salesHeader.No)
	if salesLine.FindLast() {
		r.lastLineNo = salesLine.Line_no
	}
	return nil
}

// onPreDataItem: the invoice's project when no project is filtered
func (r *getProjectUsage) onPreDataItem(projectLedgEntry *gtables.ProjectLedgerEntry, run *Run) error {
	if r.salesHeader.Project_no != "" && projectLedgEntry.GetFilter("project_no") == "" {
		projectLedgEntry.SetRange("project_no", r.salesHeader.Project_no)
	}
	return nil
}

// onAfterGetRecord: an entry of a project billed to the invoice's customer adds its remaining
// amount (and the quantity and cost in proportion) to the buffer and is marked with the buffer
// line's Applies-to ID and its sign
func (r *getProjectUsage) onAfterGetRecord(projectLedgEntry *gtables.ProjectLedgerEntry, run *Run) error {
	group := projectLedgEntry.Project_no.String() + "\x00" + projectLedgEntry.Phase_code.String() + "\x00" + projectLedgEntry.Task_code.String() + "\x00" +
		projectLedgEntry.Step_code.String() + "\x00" + strconv.Itoa(int(projectLedgEntry.Type)) + "\x00" + projectLedgEntry.No.String()
	if group != r.group {
		// the group before ends (its group footer: FinalizeJobPostingBuffer)
		err := r.finalize(run)
		if err != nil {
			return err
		}
		r.group = group
	}
	if r.project == nil || r.project.No != projectLedgEntry.Project_no {
		r.project = gtables.NewProject(run)
		err := r.project.GetOrError(projectLedgEntry.Project_no)
		if err != nil {
			return err
		}
	}
	if r.project.Bill_to_customer_no != r.salesHeader.Bill_to_customer_no {
		return nil
	}
	key := projectUsageKey{projectLedgEntry.Project_no, projectLedgEntry.Type, projectLedgEntry.No, projectLedgEntry.Unit_of_measure_code,
		projectLedgEntry.Work_type_code, projectLedgEntry.Dimension_1_code, projectLedgEntry.Dimension_2_code}
	var buffer *projectUsageBuffer
	for _, b := range r.buffer {
		if b.key == key {
			buffer = b
		}
	}
	if buffer == nil {
		// a new buffer line: its Applies-to ID is this entry's number — earlier marks with it go
		buffer = &projectUsageBuffer{key: key, appliesToID: strconv.Itoa(projectLedgEntry.Entry_no)}
		r.buffer = append(r.buffer, buffer)
		for _, sign := range []string{"+", "-"} {
			err := clearProjectAppliesToID(run, buffer.appliesToID+sign)
			if err != nil {
				return err
			}
		}
	}
	buffer.phaseCode, buffer.taskCode, buffer.stepCode = projectLedgEntry.Phase_code, projectLedgEntry.Task_code, projectLedgEntry.Step_code
	if !projectLedgEntry.Total_price.IsZero() {
		share := projectLedgEntry.Remaining_amount.Div(projectLedgEntry.Total_price)
		buffer.quantity = buffer.quantity.Add(system.Round(projectLedgEntry.Quantity.Mul(share), types.MustDecimal("0.00001")))
		buffer.totalCost = buffer.totalCost.Add(system.Round(projectLedgEntry.Total_cost.Mul(share), types.MustDecimal("0.00001")))
		buffer.totalPrice = buffer.totalPrice.Add(projectLedgEntry.Remaining_amount)
	}
	sign := "-"
	if projectLedgEntry.Total_price.IsPositive() {
		sign = "+"
	}
	entry := gtables.NewProjectLedgerEntry(run)
	err := entry.GetOrError(projectLedgEntry.Entry_no)
	if err != nil {
		return err
	}
	entry.Applies_to_id = types.Code(buffer.appliesToID + sign)
	return entry.ModifyOrError(false)
}

// clearProjectAppliesToID: the entries marked with an Applies-to ID lose it
func clearProjectAppliesToID(run *Run, appliesToID string) error {
	applEntry := gtables.NewProjectLedgerEntry(run)
	applEntry.SetCurrentKey("applies_to_id")
	applEntry.SetRange("applies_to_id", types.Code(appliesToID))
	for found := applEntry.FindSetBuffered(); found; found = applEntry.Next() {
		applEntry.Applies_to_id = ""
		err := applEntry.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}

// finalize (FinalizeJobPostingBuffer): an invoice line per buffer line with a quantity — the
// resource, item or (for G/L account usage) the project posting group's G/L Exp. Sales Acc., its
// unit of measure and work type, the unit price (in the invoice's currency) and cost from the
// totals, the project, its phase, task and step, and the Project Applies-to ID with the sign of
// the amount (the entries of the other sign lose their mark)
func (r *getProjectUsage) finalize(run *Run) error {
	buffer := r.buffer
	r.buffer = nil
	for _, b := range buffer {
		if b.quantity.IsZero() {
			continue
		}
		salesLine := gtables.NewSalesLine(run)
		salesLine.Init()
		salesLine.Document_type = gtables.SalesLineDocument_type(r.salesHeader.Document_type)
		salesLine.Document_no = r.salesHeader.No
		r.lastLineNo += 10000
		salesLine.Line_no = r.lastLineNo
		no := b.key.no
		switch b.key.lineType {
		case gtables.ProjectLedgerEntry_Type.Resource:
			salesLine.Type = gtables.SalesLine_Type.Resource
		case gtables.ProjectLedgerEntry_Type.Item:
			salesLine.Type = gtables.SalesLine_Type.Item
		default:
			salesLine.Type = gtables.SalesLine_Type.GLAccount
			project := gtables.NewProject(run)
			err := project.GetOrError(b.key.projectNo)
			if err == nil {
				err = project.TestField("project_posting_group")
			}
			if err != nil {
				return err
			}
			postingGroup := gtables.NewProjectPostingGroup(run)
			err = postingGroup.GetOrError(project.Project_posting_group)
			if err == nil {
				err = postingGroup.TestField("g_l_exp_sales_acc")
			}
			if err != nil {
				return err
			}
			no = postingGroup.G_l_exp_sales_acc
		}
		err := salesLine.Validate("no", no)
		if err != nil {
			return err
		}
		if b.key.lineType == gtables.ProjectLedgerEntry_Type.AccountGL {
			glAcc := gtables.NewGLAccount(run)
			if glAcc.Get(b.key.no) {
				salesLine.Description = glAcc.Name
			}
		}
		salesLine.Work_type_code = b.key.workType
		if b.key.unitOfMeasure != "" {
			err = salesLine.Validate("unit_of_measure_code", b.key.unitOfMeasure)
			if err != nil {
				return err
			}
		}
		unitPrice := system.Round(b.totalPrice.Div(b.quantity), types.MustDecimal("0.00001"))
		if r.salesHeader.Currency_code != "" {
			err = r.salesHeader.TestField("currency_factor")
			if err != nil {
				return err
			}
			unitPrice, err = tables.ExchangeRates(run).ExchangeAmtLCYToFCY(system.WorkDate(), r.salesHeader.Currency_code, unitPrice, r.salesHeader.Currency_factor)
			if err != nil {
				return err
			}
			unitPrice = system.Round(unitPrice, types.MustDecimal("0.01"))
		}
		salesLine.Unit_price = unitPrice
		err = salesLine.Validate("unit_cost_lcy", system.Round(b.totalCost.Div(b.quantity), types.MustDecimal("0.00001")))
		if err == nil {
			err = salesLine.Validate("quantity", b.quantity)
		}
		if err != nil {
			return err
		}
		salesLine.Project_no = b.key.projectNo
		salesLine.Dimension_1_code = b.key.dimension1
		salesLine.Dimension_2_code = b.key.dimension2
		salesLine.Phase_code, salesLine.Task_code, salesLine.Step_code = b.phaseCode, b.taskCode, b.stepCode
		sign, otherSign := "+", "-"
		if !b.totalPrice.IsPositive() {
			sign, otherSign = "-", "+"
		}
		err = clearProjectAppliesToID(run, b.appliesToID+otherSign)
		if err != nil {
			return err
		}
		salesLine.Project_applies_to_id = types.Code(b.appliesToID + sign)
		err = salesLine.InsertOrError(true)
		if err != nil {
			return err
		}
		r.inserted++
	}
	return nil
}
