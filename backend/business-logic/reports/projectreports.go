package reports

import (
	"sort"
	"strings"

	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The project reports (main menu Projects → Reports; the classic local project reports
// 10210–10220 renumbered in the base range):
//   - 1201 Project List: the projects with their customer and budget;
//   - 1203 Project Register: per register its project ledger entries;
//   - 1212 Completed Projects: the completed projects' budget, invoicing, cost and profit;
//   - 1215/1216 Customer Projects (Cost)/(Price): the projects on Order per customer, their
//     budget and usage (and invoicing), completion %, with customer and report totals;
//   - 1219 Project Cost Suggested Billing: per customer the usage not invoiced yet;
//   - 1217 Project Cost Budget: per project its budget lines by phase, task and step with
//     group totals;
//   - 1210/1211 Project Actual to Budget (Cost)/(Price): per project the budget lines and the
//     usage by phase, task, step, type and number, the variance and its percentage;
//   - 1220 Project Cost Transaction Detail: per project its ledger entries, total usage and
//     sales.
//
// Group headers and footers (the classic GroupHeader/GroupFooter sections with
// TotalsCausedBy) are rows printed through an Integer data item: each report fills reportRows,
// a section per row kind prints them.
const (
	ActualToBudgetCostID          = 1210
	ActualToBudgetPriceID         = 1211
	ProjectListID                 = 1201
	ProjectRegisterID             = 1203
	CompletedProjectsID           = 1212
	CustomerProjectsCostID        = 1215
	CustomerProjectsPriceID       = 1216
	ProjectCostBudgetID           = 1217
	ProjectCostSuggestedBillingID = 1219
	ProjectCostTransactionDetail  = 1220
)

func init() {
	Register(ProjectListID, "project-list", func() Report { return &projectList{} })
	Register(ProjectRegisterID, "project-register", func() Report { return &projectRegister{} })
	Register(CompletedProjectsID, "completed-projects", func() Report { return &completedProjects{} })
	Register(CustomerProjectsCostID, "customer-projects-cost", func() Report { return &customerProjects{kind: customerProjectsCost} })
	Register(CustomerProjectsPriceID, "customer-projects-price", func() Report { return &customerProjects{kind: customerProjectsPrice} })
	Register(ProjectCostSuggestedBillingID, "project-cost-suggested-billing", func() Report { return &customerProjects{kind: suggestedBilling} })
	Register(ProjectCostBudgetID, "project-cost-budget", func() Report { return &projectCostBudget{} })
	Register(ActualToBudgetCostID, "project-actual-to-budget-cost", func() Report { return &actualToBudget{price: false} })
	Register(ActualToBudgetPriceID, "project-actual-to-budget-price", func() Report { return &actualToBudget{price: true} })
	Register(ProjectCostTransactionDetail, "project-cost-transaction-detail", func() Report { return &projectTransactionDetail{} })
}

// percentOf: part as a percentage of base (0 without a base)
func percentOf(part, base types.Decimal) types.Decimal {
	if base.IsZero() {
		return types.Decimal{}
	}
	return types.NewDecimalFromInt(100).Mul(part).Div(base)
}

// reportRow is a printed row of a grouped report: its kind ("phase_head", "line", "total" …),
// the values its section prints
type reportRow struct {
	kind   string
	values map[string]any
}

// reportRows prints rows through an Integer data item: each row sets its values and its kind
// (the value named after the kind is true, the other kinds' false — the sections' print_if)
type reportRows struct {
	rows  []reportRow
	kinds []string
}

func (rr *reportRows) add(kind string, values map[string]any) {
	rr.rows = append(rr.rows, reportRow{kind: kind, values: values})
}

func (rr *reportRows) dataItem(name string) *DataItem {
	return &DataItem{
		Name:  name,
		Table: IntegerTable,
		OnPreDataItem: Trigger(func(rec *Integer, run *Run) error {
			rec.SetRange("number", 1, len(rr.rows))
			return nil
		}),
		OnAfterGetRecord: Trigger(func(rec *Integer, run *Run) error {
			row := rr.rows[rec.Number-1]
			for _, kind := range rr.kinds {
				run.SetValue(kind, kind == row.kind)
			}
			for name, value := range row.values {
				run.SetValue(name, value)
			}
			return nil
		}),
	}
}

// projectHead: the head of a report on projects (company, today, user, the projects' filters)
func projectHead(run *Run) error {
	setReportHeadValues(run)
	return nil
}

// Report 1201 Project List
type projectList struct{}

func (r *projectList) Definition() *Definition {
	return &Definition{
		OnPreReport: projectHead,
		DataItems: []*DataItem{{
			Name: "Project", Table: gtables.ProjectTableName, Key: []string{"no"},
			RequestFilterFields: []string{"no", "search_description", "bill_to_customer_no", "status", "date_filter"},
			OnPreDataItem:       headWithFilters[*gtables.Project](nil),
			OnAfterGetRecord: Trigger(func(project *gtables.Project, run *Run) error {
				project.CalcFields("budgeted_cost", "budgeted_price")
				customer := gtables.NewCustomer(run)
				if !customer.Get(project.Bill_to_customer_no) {
					customer.Init()
				}
				run.SetValue("customer_name", customer.Name.String())
				run.SetValue("description_2", project.Description_2.String())
				return nil
			}),
		}},
	}
}

// Report 1203 Project Register
type projectRegister struct {
	projectReg *gtables.ProjectRegister
}

func (r *projectRegister) Definition() *Definition {
	return &Definition{
		Options:     []Option{{Name: "print_descriptions", Type: "bool", Default: "false"}},
		OnPreReport: projectHead,
		DataItems: []*DataItem{{
			Name: "Project_Register", Table: gtables.ProjectRegisterTableName, Key: []string{"no"},
			RequestFilterFields: []string{"no", "creation_date", "source_code", "journal_batch_name"},
			OnPreDataItem:       headWithFilters[*gtables.ProjectRegister](nil),
			OnAfterGetRecord: Trigger(func(projectReg *gtables.ProjectRegister, run *Run) error {
				r.projectReg = projectReg
				sourceCode := gtables.NewSourceCode(run)
				sourceCodeText := ""
				if projectReg.Source_code != "" {
					sourceCodeText = run.Text("SOURCE_CODE", projectReg.Source_code)
					if !sourceCode.Get(projectReg.Source_code) {
						sourceCode.Init()
					}
				}
				run.SetValue("register_text", run.Text("REGISTER_NO", projectReg.No))
				run.SetValue("source_code_text", sourceCodeText)
				run.SetValue("source_code_description", sourceCode.Description.String())
				run.SetValue("no_of_entries", projectReg.To_entry_no-projectReg.From_entry_no+1)
				run.SetValue("entries_text", run.Text("NO_OF_ENTRIES", projectReg.No))
				return nil
			}),
			DataItems: []*DataItem{{
				Name: "Project_Ledger_Entry", Table: gtables.ProjectLedgerEntryTableName, Key: []string{"entry_no"},
				RequestFilterFields: []string{"project_no", "posting_date", "document_no"},
				OnPreDataItem: Trigger(func(projectLedgEntry *gtables.ProjectLedgerEntry, run *Run) error {
					projectLedgEntry.SetRange("entry_no", r.projectReg.From_entry_no, r.projectReg.To_entry_no)
					return nil
				}),
				OnAfterGetRecord: Trigger(func(projectLedgEntry *gtables.ProjectLedgerEntry, run *Run) error {
					projectDescription := ""
					if run.OptionBool("print_descriptions") {
						projectDescription = projectLedgEntry.Description.String()
						if projectDescription == "" {
							project := gtables.NewProject(run)
							if !project.Get(projectLedgEntry.Project_no) {
								project.Init()
							}
							projectDescription = project.Description.String()
						}
					}
					run.SetValue("project_description", projectDescription)
					return nil
				}),
			}},
		}},
	}
}

// Report 1212 Completed Projects
type completedProjects struct {
	totalProfit, totalInvoiced types.Decimal
}

func (r *completedProjects) Definition() *Definition {
	return &Definition{
		OnPreReport: projectHead,
		DataItems: []*DataItem{{
			Name: "Project", Table: gtables.ProjectTableName, Key: []string{"no"},
			TableView:           []ftables.FieldFilter{{Field: "status", Expression: "3"}}, // Completed
			RequestFilterFields: []string{"no", "bill_to_customer_no"},
			OnPreDataItem:       headWithFilters[*gtables.Project](nil),
			OnAfterGetRecord: Trigger(func(project *gtables.Project, run *Run) error {
				project.CalcFields("budgeted_price", "invoiced_price", "usage_cost")
				profit := project.Invoiced_price.Sub(project.Usage_cost)
				run.SetValue("profit", profit)
				run.SetValue("percent_profit", percentOf(profit, project.Invoiced_price))
				r.totalProfit = r.totalProfit.Add(profit)
				r.totalInvoiced = r.totalInvoiced.Add(project.Invoiced_price)
				run.SetValue("total_profit", r.totalProfit)
				run.SetValue("total_percent_profit", percentOf(r.totalProfit, r.totalInvoiced))
				return nil
			}),
		}},
	}
}

// customerProjectsKind: which of the customer reports (1215, 1216, 1219)
type customerProjectsKind int

const (
	customerProjectsCost customerProjectsKind = iota
	customerProjectsPrice
	suggestedBilling
)

// customerProjects is reports 1215, 1216 and 1219: the projects on Order of customers grouped
// by the bill-to customer, a heading per customer, its totals and the report's
type customerProjects struct {
	kind     customerProjectsKind
	rows     reportRows
	group    [4]types.Decimal
	total    [4]types.Decimal
	customer types.Code
}

func (r *customerProjects) Definition() *Definition {
	r.rows.kinds = []string{"customer_head", "line", "customer_total"}
	return &Definition{
		OnPreReport: projectHead,
		DataItems: []*DataItem{{
			Name: "Project", Table: gtables.ProjectTableName, Key: []string{"bill_to_customer_no", "no"},
			TableView: []ftables.FieldFilter{
				{Field: "status", Expression: "2"}, // Order
				{Field: "bill_to_customer_no", Expression: "<>''"},
			},
			RequestFilterFields: []string{"bill_to_customer_no", "date_filter"},
			OnPreDataItem:       headWithFilters[*gtables.Project](nil),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem: Trigger(func(project *gtables.Project, run *Run) error {
				r.closeCustomer(run)
				for i, v := range r.total {
					run.SetValue(totalName(i), v)
				}
				run.SetValue("total_percent", r.percent(r.total))
				run.SetValue("total_percent_invoiced", percentOf(r.total[2], r.total[1]))
				return nil
			}),
		}, r.rows.dataItem("Rows")},
	}
}

func totalName(i int) string { return "total_" + string(rune('0'+i)) }

// the amounts of a project: budget, usage, invoiced, suggested billing
func (r *customerProjects) amounts(project *gtables.Project) [4]types.Decimal {
	if r.kind == customerProjectsCost {
		project.CalcFields("budgeted_cost", "usage_cost")
		return [4]types.Decimal{project.Budgeted_cost, project.Usage_cost}
	}
	project.CalcFields("budgeted_price", "usage_price", "invoiced_price")
	var suggested types.Decimal
	if project.Usage_price.GreaterThan(project.Invoiced_price) {
		suggested = project.Usage_price.Sub(project.Invoiced_price)
	}
	return [4]types.Decimal{project.Budgeted_price, project.Usage_price, project.Invoiced_price, suggested}
}

// percent: the completion — usage of the budget
func (r *customerProjects) percent(a [4]types.Decimal) types.Decimal {
	return percentOf(a[1], a[0])
}

func (r *customerProjects) onAfterGetRecord(project *gtables.Project, run *Run) error {
	if project.Bill_to_customer_no != r.customer {
		r.closeCustomer(run)
		r.customer = project.Bill_to_customer_no
		customer := gtables.NewCustomer(run)
		if !customer.Get(project.Bill_to_customer_no) {
			customer.Init()
		}
		r.rows.add("customer_head", map[string]any{
			"customer_text": customer.TableCaption(fcodeunits.CurrentLanguage()) + " " + customer.FieldCaption("no", fcodeunits.CurrentLanguage()) + " " + project.Bill_to_customer_no.String(),
			"customer_name": customer.Name.String(),
		})
	}
	a := r.amounts(project)
	for i := range a {
		r.group[i] = r.group[i].Add(a[i])
		r.total[i] = r.total[i].Add(a[i])
	}
	r.rows.add("line", map[string]any{
		"project_no": project.No.String(), "description": project.Description.String(),
		"starting_date": project.Starting_date, "ending_date": project.Ending_date,
		"amount_0": a[0], "amount_1": a[1], "amount_2": a[2], "amount_3": a[3],
		"percent": r.percent(a), "percent_invoiced": percentOf(a[2], a[1]),
	})
	return Skip() // printed by the rows
}

// closeCustomer: the totals of the customer before (the classic group footer)
func (r *customerProjects) closeCustomer(run *Run) {
	if r.customer == "" {
		return
	}
	customer := gtables.NewCustomer(run)
	language := fcodeunits.CurrentLanguage()
	r.rows.add("customer_total", map[string]any{
		"total_text": run.Text("TOTAL_FOR", customer.TableCaption(language)+" "+customer.FieldCaption("no", language)+" "+r.customer.String()),
		"group_0":    r.group[0], "group_1": r.group[1], "group_2": r.group[2], "group_3": r.group[3],
		"group_percent": r.percent(r.group), "group_percent_invoiced": percentOf(r.group[2], r.group[1]),
	})
	r.group = [4]types.Decimal{}
	r.customer = ""
}

// phaseTaskStep groups budget rows by phase, task and step: a heading where a (non-blank) code
// starts, its totals where it ends (classic ChangeLevel with TotalsCausedBy)
type phaseTaskStep struct {
	rows    *reportRows
	codes   [3]types.Code // the open phase, task and step
	open    [3]bool
	totals  [3][]types.Decimal
	columns int
	src     ftables.Source
	run     *Run
}

var levelNames = [3]string{"phase", "task", "step"}

// level: the indentation of the lines (the open groups)
func (g *phaseTaskStep) level() int {
	n := 0
	for _, o := range g.open {
		if o {
			n++
		}
	}
	return n
}

func (g *phaseTaskStep) description(level int, code types.Code) string {
	switch level {
	case 0:
		phase := gtables.NewPhase(g.src)
		if phase.Get(code) {
			return phase.Description.String()
		}
	case 1:
		task := gtables.NewTask(g.src)
		if task.Get(code) {
			return task.Description.String()
		}
	case 2:
		step := gtables.NewStep(g.src)
		if step.Get(code) {
			return step.Description.String()
		}
	}
	return ""
}

// close: the totals of the groups from level on (the innermost first)
func (g *phaseTaskStep) close(from int, blankLine bool) {
	for level := 2; level >= from; level-- {
		if !g.open[level] {
			continue
		}
		values := map[string]any{"code": g.codes[level].String(), "text": g.run.Text("TOTAL", g.description(level, g.codes[level])), "indent": strings.Repeat("  ", level)}
		for i, v := range g.totals[level] {
			values["total_"+string(rune('0'+i))] = v
		}
		g.rows.add(levelNames[level]+"_total", values)
		if blankLine {
			g.rows.add("blank", nil)
		}
		g.open[level] = false
		g.codes[level] = ""
	}
}

// next: a line of the given codes — the groups that end are closed, the groups that start get
// their heading; its amounts add to the open groups
func (g *phaseTaskStep) next(codes [3]types.Code, amounts []types.Decimal, blankLines bool) {
	for level := 0; level < 3; level++ {
		if codes[level] != g.codes[level] || (!g.open[level] && codes[level] != "") {
			g.close(level, blankLines)
			for l := level; l < 3; l++ {
				if codes[l] != "" {
					g.open[l] = true
					g.codes[l] = codes[l]
					g.totals[l] = make([]types.Decimal, g.columns)
					g.rows.add(levelNames[l]+"_head", map[string]any{"code": codes[l].String(), "text": g.description(l, codes[l]), "indent": strings.Repeat("  ", l)})
				}
			}
			break
		}
	}
	for level := 0; level < 3; level++ {
		if g.open[level] {
			for i := range amounts {
				g.totals[level][i] = g.totals[level][i].Add(amounts[i])
			}
		}
	}
}

// groupKinds: the row kinds of the phase/task/step reports
var groupKinds = []string{"phase_head", "task_head", "step_head", "line", "step_total", "task_total", "phase_total", "blank"}

// Report 1217 Project Cost Budget: a page per project, its budget lines in the project's
// FlowFilters
type projectCostBudget struct {
	rows reportRows
}

func (r *projectCostBudget) Definition() *Definition {
	r.rows.kinds = groupKinds
	return &Definition{
		Options:     []Option{{Name: "print_blank_lines", Type: "bool", Default: "false"}},
		OnPreReport: projectHead,
		DataItems: []*DataItem{{
			Name: "Project", Table: gtables.ProjectTableName, Key: []string{"no"},
			RequestFilterFields: []string{"no", "bill_to_customer_no", "date_filter", "status"},
			OnPreDataItem:       headWithFilters[*gtables.Project](nil),
			OnAfterGetRecord: Trigger(func(project *gtables.Project, run *Run) error {
				run.SetValue("title", run.Text("TITLE_FOR", project.No))
				projectDates(project, run)
				r.rows.rows = nil
				g := &phaseTaskStep{rows: &r.rows, columns: 2, src: run, run: run}
				blankLines := run.OptionBool("print_blank_lines")
				projectBudgetLine := gtables.NewProjectBudgetLine(run)
				linkProjectBudgetLines(projectBudgetLine, project)
				var totalCost, totalPrice types.Decimal
				for found := projectBudgetLine.FindSetBuffered(); found; found = projectBudgetLine.Next() {
					projectBudgetLine.CalcFields("quantity", "total_cost", "total_price")
					g.next([3]types.Code{projectBudgetLine.Phase_code, projectBudgetLine.Task_code, projectBudgetLine.Step_code},
						[]types.Decimal{projectBudgetLine.Total_cost, projectBudgetLine.Total_price}, blankLines)
					r.rows.add("line", map[string]any{
						"description": strings.Repeat("    ", g.level()) + projectBudgetLine.Description.String(),
						"type":        budgetLineTypeCaption(run, projectBudgetLine.Type),
						"no":          projectBudgetLine.No.String(), "quantity": projectBudgetLine.Quantity,
						"unit_cost": projectBudgetLine.Unit_cost, "total_cost": projectBudgetLine.Total_cost,
						"unit_price": projectBudgetLine.Unit_price, "total_price": projectBudgetLine.Total_price,
					})
					totalCost = totalCost.Add(projectBudgetLine.Total_cost)
					totalPrice = totalPrice.Add(projectBudgetLine.Total_price)
				}
				g.close(0, blankLines)
				run.SetValue("project_total_text", run.Text("TOTAL_FOR_PROJECT", project.No))
				run.SetValue("project_total_cost", totalCost)
				run.SetValue("project_total_price", totalPrice)
				return nil
			}),
			DataItems: []*DataItem{r.rows.dataItem("Rows")},
		}},
	}
}

// projectDates: the project's descriptions and dates in the head of a project page
func projectDates(project *gtables.Project, run *Run) {
	language := fcodeunits.CurrentLanguage()
	run.SetValue("project_description", project.Description.String())
	run.SetValue("project_description_2", project.Description_2.String())
	run.SetValue("starting_date_caption", project.FieldCaption("starting_date", language))
	run.SetValue("starting_date", project.Starting_date)
	run.SetValue("ending_date_caption", project.FieldCaption("ending_date", language))
	run.SetValue("ending_date", project.Ending_date)
}

// linkProjectBudgetLines (DataItemLink): the project's budget lines in its FlowFilters
func linkProjectBudgetLines(projectBudgetLine *gtables.ProjectBudgetLine, project *gtables.Project) {
	projectBudgetLine.SetRange("project_no", project.No)
	if project.Date_filter != "" {
		_ = projectBudgetLine.SetFlowFilter("date_filter", project.Date_filter)
	}
	for field, filter := range map[string]string{"phase_code": project.Phase_filter, "task_code": project.Task_filter, "step_code": project.Step_filter, "type": project.Type_filter} {
		if filter != "" {
			projectBudgetLine.SetFilter(field, filter)
		}
	}
}

// actualToBudget is reports 1210 (cost) and 1211 (price): per project the budget lines and the
// usage entries (Related to Budget) merged by phase, task, step, type and number
type actualToBudget struct {
	price bool
	rows  reportRows
}

// budgetActual: a budget line's amounts with the usage of its key
type budgetActual struct {
	codes       [3]types.Code
	lineType    gtables.ProjectBudgetLineType
	no          types.Code
	description string
	budget      types.Decimal
	actual      types.Decimal
}

func (r *actualToBudget) Definition() *Definition {
	r.rows.kinds = groupKinds
	return &Definition{
		OnPreReport: projectHead,
		DataItems: []*DataItem{{
			Name: "Project", Table: gtables.ProjectTableName, Key: []string{"no"},
			RequestFilterFields: []string{"no", "bill_to_customer_no", "date_filter", "status"},
			OnPreDataItem:       headWithFilters[*gtables.Project](nil),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			DataItems:           []*DataItem{r.rows.dataItem("Rows")},
		}},
	}
}

func (r *actualToBudget) onAfterGetRecord(project *gtables.Project, run *Run) error {
	if r.price {
		run.SetValue("title", run.Text("TITLE_FOR_PRICE", project.No))
	} else {
		run.SetValue("title", run.Text("TITLE_FOR_COST", project.No))
	}
	projectDates(project, run)
	lines := map[string]*budgetActual{}
	key := func(codes [3]types.Code, lineType int, no types.Code) string {
		return codes[0].String() + "\x00" + codes[1].String() + "\x00" + codes[2].String() + "\x00" + string(rune('0'+lineType)) + "\x00" + no.String()
	}
	projectBudgetLine := gtables.NewProjectBudgetLine(run)
	linkProjectBudgetLines(projectBudgetLine, project)
	for found := projectBudgetLine.FindSetBuffered(); found; found = projectBudgetLine.Next() {
		projectBudgetLine.CalcFields("total_cost", "total_price")
		codes := [3]types.Code{projectBudgetLine.Phase_code, projectBudgetLine.Task_code, projectBudgetLine.Step_code}
		k := key(codes, int(projectBudgetLine.Type), projectBudgetLine.No)
		l, ok := lines[k]
		if !ok {
			l = &budgetActual{codes: codes, lineType: projectBudgetLine.Type, no: projectBudgetLine.No, description: projectBudgetLine.Description.String()}
			lines[k] = l
		}
		if r.price {
			l.budget = l.budget.Add(projectBudgetLine.Total_price)
		} else {
			l.budget = l.budget.Add(projectBudgetLine.Total_cost)
		}
	}
	projectLedgEntry := gtables.NewProjectLedgerEntry(run)
	projectLedgEntry.SetRange("project_no", project.No)
	projectLedgEntry.SetRange("entry_type", gtables.ProjectLedgerEntry_Entry_type.Usage)
	if project.Date_filter != "" {
		projectLedgEntry.SetFilter("posting_date", project.Date_filter)
	}
	for field, filter := range map[string]string{"phase_code": project.Phase_filter, "task_code": project.Task_filter, "step_code": project.Step_filter, "type": project.Type_filter} {
		if filter != "" {
			projectLedgEntry.SetFilter(field, filter)
		}
	}
	for found := projectLedgEntry.FindSetBuffered(); found; found = projectLedgEntry.Next() {
		codes := [3]types.Code{projectLedgEntry.Phase_code, projectLedgEntry.Task_code, projectLedgEntry.Step_code}
		k := key(codes, int(projectLedgEntry.Type), projectLedgEntry.No)
		l, ok := lines[k]
		if !ok {
			l = &budgetActual{codes: codes, lineType: gtables.ProjectBudgetLineType(projectLedgEntry.Type), no: projectLedgEntry.No, description: projectLedgEntry.Description.String()}
			lines[k] = l
		}
		if r.price {
			l.actual = l.actual.Add(projectLedgEntry.Total_price)
		} else {
			l.actual = l.actual.Add(projectLedgEntry.Total_cost)
		}
	}
	keys := make([]string, 0, len(lines))
	for k := range lines {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	r.rows.rows = nil
	g := &phaseTaskStep{rows: &r.rows, columns: 2, src: run, run: run}
	var totalActual, totalBudget types.Decimal
	for _, k := range keys {
		l := lines[k]
		g.next(l.codes, []types.Decimal{l.actual, l.budget}, false)
		variance := l.actual.Sub(l.budget)
		r.rows.add("line", map[string]any{
			"description": strings.Repeat("    ", g.level()) + l.description,
			"type":        budgetLineTypeCaption(run, l.lineType),
			"no":          l.no.String(), "actual": l.actual, "budget": l.budget, "variance": variance, "percent_variance": percentOf(variance, l.budget),
		})
		totalActual = totalActual.Add(l.actual)
		totalBudget = totalBudget.Add(l.budget)
	}
	g.close(0, false)
	// the groups' totals: their variance
	for i := range r.rows.rows {
		row := &r.rows.rows[i]
		if strings.HasSuffix(row.kind, "_total") {
			actual, _ := row.values["total_0"].(types.Decimal)
			budget, _ := row.values["total_1"].(types.Decimal)
			row.values["total_2"] = actual.Sub(budget)
			row.values["total_3"] = percentOf(actual.Sub(budget), budget)
		}
	}
	run.SetValue("project_total_text", run.Text("TOTAL_FOR_PROJECT", project.No))
	run.SetValue("project_actual", totalActual)
	run.SetValue("project_budget", totalBudget)
	run.SetValue("project_variance", totalActual.Sub(totalBudget))
	run.SetValue("project_percent_variance", percentOf(totalActual.Sub(totalBudget), totalBudget))
	return nil
}

// Report 1220 Project Cost Transaction Detail
type projectTransactionDetail struct {
	totalCost, totalPrice, amtPostedToGL, amtRecognized [2]types.Decimal
}

func (r *projectTransactionDetail) Definition() *Definition {
	return &Definition{
		OnPreReport: projectHead,
		DataItems: []*DataItem{{
			Name: "Project", Table: gtables.ProjectTableName, Key: []string{"no"},
			RequestFilterFields: []string{"no", "status", "date_filter"},
			OnPreDataItem:       headWithFilters[*gtables.Project](nil),
			OnAfterGetRecord: Trigger(func(project *gtables.Project, run *Run) error {
				language := fcodeunits.CurrentLanguage()
				run.SetValue("project_text", project.TableCaption(language)+" "+project.FieldCaption("no", language)+" "+project.No.String())
				projectLedgEntry := gtables.NewProjectLedgerEntry(run)
				run.SetValue("usage_text", run.Text("TOTAL_USAGE_FOR", projectLedgEntry.FieldCaption("project_no", language), project.No))
				run.SetValue("sales_text", run.Text("TOTAL_SALES_FOR", projectLedgEntry.FieldCaption("project_no", language), project.No))
				return nil
			}),
			DataItems: []*DataItem{{
				Name: "Project_Ledger_Entry", Table: gtables.ProjectLedgerEntryTableName, Key: []string{"project_no", "posting_date"},
				Link: map[string]string{"project_no": "no"},
				OnPreDataItem: Trigger(func(projectLedgEntry *gtables.ProjectLedgerEntry, run *Run) error {
					r.totalCost, r.totalPrice, r.amtPostedToGL, r.amtRecognized = [2]types.Decimal{}, [2]types.Decimal{}, [2]types.Decimal{}, [2]types.Decimal{}
					r.setTotals(run)
					return nil
				}),
				OnAfterGetRecord: Trigger(func(projectLedgEntry *gtables.ProjectLedgerEntry, run *Run) error {
					i := int(projectLedgEntry.Entry_type)
					r.totalCost[i] = r.totalCost[i].Add(projectLedgEntry.Total_cost)
					r.totalPrice[i] = r.totalPrice[i].Add(projectLedgEntry.Total_price)
					r.amtPostedToGL[i] = r.amtPostedToGL[i].Add(projectLedgEntry.Amt_posted_to_g_l)
					r.amtRecognized[i] = r.amtRecognized[i].Add(projectLedgEntry.Amt_recognized)
					r.setTotals(run)
					return nil
				}),
			}},
		}},
	}
}

// setTotals: the footer's usage and sales totals (set before the last entry ends)
func (r *projectTransactionDetail) setTotals(run *Run) {
	for i, name := range []string{"usage", "sales"} {
		run.SetValue(name+"_total_cost", r.totalCost[i])
		run.SetValue(name+"_total_price", r.totalPrice[i])
		run.SetValue(name+"_amt_posted_to_g_l", r.amtPostedToGL[i])
		run.SetValue(name+"_amt_recognized", r.amtRecognized[i])
	}
}

// budgetLineTypeCaption: a budget line type in the user's language
func budgetLineTypeCaption(run *Run, lineType gtables.ProjectBudgetLineType) string {
	options := gtables.NewProjectBudgetLine(run).GetOptionFields()["type"]
	if int(lineType) >= len(options) {
		return ""
	}
	return optionCaption(gtables.ProjectBudgetLineTableName, "type", options[lineType], fcodeunits.CurrentLanguage())
}
