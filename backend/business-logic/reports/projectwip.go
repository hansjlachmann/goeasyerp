package reports

import (
	"sort"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The project WIP and recognition batch jobs (Projects → Activities): 1295 Calculate Project WIP
// Value and 1294 Calculate Project Recognition fill the project ledger entries' Amt. to Post to
// G/L / Amt. to Recognize; 1214 Post Project WIP Value to G/L and 1213 Post Project Recognition
// to G/L post what changed since — per project ledger entry or per project — as pairs of G/L
// lines (codeunit 12): 1214 between the General Posting Setup's Project Cost/Sales Adjmt.
// Account (a G/L account entry without general product posting group: its own account) and the
// project posting group's WIP Cost/Sales Account, 1213 between the WIP and the Cost/Sales
// Account. Their classic printouts (the buffer's lines and totals) are not migrated: the posting
// reports tell the totals in a message.
const (
	CalculateProjectRecognitionID   = 1294
	CalculateProjectWIPValueID      = 1295
	PostProjectRecognitionToGLID    = 1213
	PostProjectWIPValueToGLID       = 1214
	postingMethodPerProjectLedgEntr = 1 // per project ledger entry (the option's 1-based number)
)

func init() {
	Register(CalculateProjectWIPValueID, "calculate-project-wip-value", func() Report { return &calculateProjectAmounts{recognition: false} })
	Register(CalculateProjectRecognitionID, "calculate-project-recognition", func() Report { return &calculateProjectAmounts{recognition: true} })
	Register(PostProjectWIPValueToGLID, "post-project-wip-value-to-gl", func() Report { return &postProjectAmounts{recognition: false} })
	Register(PostProjectRecognitionToGLID, "post-project-recognition-to-gl", func() Report { return &postProjectAmounts{recognition: true} })
}

// calculateProjectAmounts is report 1295 (WIP value: the entry's cost or price by the project's
// Project Usage Posting, 0 for entries not chargeable or projects without usage posting) or 1294
// (recognition: what is posted to the G/L is to be recognized)
type calculateProjectAmounts struct {
	recognition bool
	project     *gtables.Project
	lastEntryNo int
	window      *system.Window
}

func (r *calculateProjectAmounts) Definition() *Definition {
	return &Definition{
		Transaction: true,
		DataItems: []*DataItem{{
			Name: "Project", Table: gtables.ProjectTableName, Key: []string{"no"},
			RequestFilterFields: []string{"no"},
			OnPreDataItem: Trigger(func(project *gtables.Project, run *Run) error {
				projectLedgEntry := gtables.NewProjectLedgerEntry(run)
				if projectLedgEntry.FindLast() {
					r.lastEntryNo = projectLedgEntry.Entry_no
				}
				r.window = system.OpenWindow(run.Text("WINDOW"))
				return nil
			}),
			OnAfterGetRecord: Trigger(func(project *gtables.Project, run *Run) error {
				r.project = project
				return nil
			}),
			OnPostDataItem: Trigger(func(project *gtables.Project, run *Run) error {
				r.window.Close()
				return nil
			}),
			DataItems: []*DataItem{{
				Name: "Project_Ledger_Entry", Table: gtables.ProjectLedgerEntryTableName, Key: []string{"project_no", "posting_date"},
				RequestFilterFields: []string{"entry_type", "posting_date"},
				Link:                map[string]string{"project_no": "no"},
				OnAfterGetRecord:    Trigger(r.entryOnAfterGetRecord),
			}},
		}},
	}
}

func (r *calculateProjectAmounts) entryOnAfterGetRecord(projectLedgEntry *gtables.ProjectLedgerEntry, run *Run) error {
	r.window.Update(1, projectLedgEntry.Entry_no)
	if r.lastEntryNo != 0 {
		r.window.Update(2, projectLedgEntry.Entry_no*10000/r.lastEntryNo)
	}
	if r.recognition {
		tempAmtToRecognize := projectLedgEntry.Amt_to_recognize
		projectLedgEntry.Amt_to_recognize = projectLedgEntry.Amt_posted_to_g_l
		tempAmtToRecognizeAddCurr := projectLedgEntry.Add_curr_amount_to_recognize
		projectLedgEntry.Add_curr_amount_to_recognize = projectLedgEntry.Add_curr_amt_posted_to_g_l
		if !projectLedgEntry.Amt_to_recognize.Equal(tempAmtToRecognize) || !projectLedgEntry.Add_curr_amount_to_recognize.Equal(tempAmtToRecognizeAddCurr) {
			return projectLedgEntry.ModifyOrError(false)
		}
		return nil
	}
	// the classic report compares with what is posted to the G/L
	tempAmtPostedToGL := projectLedgEntry.Amt_posted_to_g_l
	tempAmtPostedToGLAddCurr := projectLedgEntry.Add_curr_amt_posted_to_g_l
	if r.project.Project_usage_posting == gtables.Project_Project_usage_posting.None {
		projectLedgEntry.Amt_to_post_to_g_l = types.Decimal{}
		projectLedgEntry.Add_curr_amt_to_post_to_g_l = types.Decimal{}
	} else {
		if (r.project.Project_usage_posting == gtables.Project_Project_usage_posting.Costs) &&
			(projectLedgEntry.Entry_type == gtables.ProjectLedgerEntry_Entry_type.Usage) {
			projectLedgEntry.Amt_to_post_to_g_l = projectLedgEntry.Total_cost
			projectLedgEntry.Add_curr_amt_to_post_to_g_l = projectLedgEntry.Additional_currency_total_cost
		} else {
			projectLedgEntry.Amt_to_post_to_g_l = projectLedgEntry.Total_price
			projectLedgEntry.Add_curr_amt_to_post_to_g_l = projectLedgEntry.Add_currency_total_price
		}
	}
	if !projectLedgEntry.Chargeable {
		projectLedgEntry.Amt_to_post_to_g_l = types.Decimal{}
		projectLedgEntry.Add_curr_amt_to_post_to_g_l = types.Decimal{}
	}
	if !projectLedgEntry.Amt_to_post_to_g_l.Equal(tempAmtPostedToGL) || !projectLedgEntry.Add_curr_amt_to_post_to_g_l.Equal(tempAmtPostedToGLAddCurr) {
		return projectLedgEntry.ModifyOrError(false)
	}
	return nil
}

// projectPostingBuffer is table 212 Project Posting Buffer (temporary): the amounts to post by
// project, entry type, posting group type, posting group, general posting groups and dimensions
type projectPostingBuffer struct {
	projectNo         types.Code
	entryType         gtables.ProjectLedgerEntryEntry_type
	postingGroupType  gtables.ProjectLedgerEntryType
	postingGroup      types.Code
	genBusPostingGrp  types.Code
	genProdPostingGrp types.Code
	dimension1Code    types.Code
	dimension2Code    types.Code
	amount            types.Decimal
	addCurrAmount     types.Decimal
}

func (b *projectPostingBuffer) key() string {
	return strings.Join([]string{b.projectNo.String(), string(rune('0' + int(b.entryType))), string(rune('0' + int(b.postingGroupType))),
		b.postingGroup.String(), b.genBusPostingGrp.String(), b.genProdPostingGrp.String(), b.dimension1Code.String(), b.dimension2Code.String()}, "\x00")
}

// postProjectAmounts is report 1214 (WIP value) or 1213 (recognition)
type postProjectAmounts struct {
	recognition        bool
	postingDate        types.Date
	postingDocNo       types.Code
	postingDescription string
	postingMethod      int
	sourceCode         types.Code
	glSetup            *gtables.GeneralLedgerSetup
	genJnlPostLine     *genjnl.GenJnlPostLine
	project            *gtables.Project
	buffer             map[string]*projectPostingBuffer
	totalUsage         types.Decimal
	totalPrice         types.Decimal
	window             *system.Window
}

func (r *postProjectAmounts) Definition() *Definition {
	return &Definition{
		Options: []Option{
			{Name: "posting_method", Type: "option", Default: "1", Options: []string{"PER_PROJECT_LEDGER_ENTRY", "PER_PROJECT"}},
			{Name: "posting_description", Type: "text"},
			{Name: "posting_date", Type: "date"},
			{Name: "document_no", Type: "text", Required: true},
		},
		Transaction:  true,
		OnInitReport: r.onInitReport,
		OnPreReport:  r.onPreReport,
		DataItems: []*DataItem{{
			Name: "Project", Table: gtables.ProjectTableName, Key: []string{"no"},
			RequestFilterFields: []string{"no"},
			OnPreDataItem:       Trigger(r.projectOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.projectOnAfterGetRecord),
			OnPostDataItem:      Trigger(r.projectOnPostDataItem),
			DataItems: []*DataItem{{
				Name: "Project_Ledger_Entry", Table: gtables.ProjectLedgerEntryTableName, Key: []string{"project_no", "posting_date"},
				RequestFilterFields: []string{"entry_type", "posting_date"},
				Link:                map[string]string{"project_no": "no"},
				OnAfterGetRecord:    Trigger(r.entryOnAfterGetRecord),
			}},
		}},
	}
}

// onInitReport (the request form's OnOpenForm): the work date and the classic description
func (r *postProjectAmounts) onInitReport(run *Run) error {
	run.SetOption("posting_date", system.WorkDate().String())
	if r.recognition {
		run.SetOption("posting_description", run.Text("RECOGNITION_DESCRIPTION"))
	} else {
		run.SetOption("posting_description", run.Text("WIP_DESCRIPTION"))
	}
	return nil
}

func (r *postProjectAmounts) onPreReport(run *Run) error {
	r.postingDocNo = types.Code(system.UpperCase(run.Option("document_no")))
	if r.postingDocNo == "" {
		return system.Error(run.Text("ENTER_DOCUMENT_NO"))
	}
	r.postingDate = run.OptionDate("posting_date")
	r.postingDescription = run.Option("posting_description")
	r.postingMethod = run.OptionInt("posting_method")
	sourceCodeSetup := gtables.GetSourceCodeSetup(run)
	r.sourceCode = sourceCodeSetup.Post_value
	if r.recognition {
		r.sourceCode = sourceCodeSetup.Post_recognition
	}
	return nil
}

func (r *postProjectAmounts) projectOnPreDataItem(project *gtables.Project, run *Run) error {
	projectLedgEntry := gtables.NewProjectLedgerEntry(run)
	projectLedgEntry.LockTable()
	r.buffer = map[string]*projectPostingBuffer{}
	r.window = system.OpenWindow(run.Text("WINDOW"))
	r.glSetup = gtables.GetGeneralLedgerSetup(run)
	r.genJnlPostLine = genjnl.NewGenJnlPostLine(run)
	return nil
}

// projectOnAfterGetRecord: the project's Project Posting Date (1214) or Recognition Date (1213);
// a Completed Contract project is recognized only when completed
func (r *postProjectAmounts) projectOnAfterGetRecord(project *gtables.Project, run *Run) error {
	r.window.Update(1, project.No)
	r.window.Update(2, "")
	r.project = project
	if r.recognition {
		if (project.Recognition_method == gtables.Project_Recognition_method.CompletedContract) && (project.Status != gtables.Project_Status.Completed) {
			return Skip()
		}
		project.Recognition_date = r.postingDate
	} else {
		project.Project_posting_date = r.postingDate
	}
	return project.ModifyOrError(false)
}

func (r *postProjectAmounts) entryOnAfterGetRecord(projectLedgEntry *gtables.ProjectLedgerEntry, run *Run) error {
	r.window.Update(2, projectLedgEntry.Posting_date)
	var adjAmount, adjAmountAddCurr types.Decimal
	if r.recognition {
		adjAmount = projectLedgEntry.Amt_to_recognize.Sub(projectLedgEntry.Amt_recognized)
		adjAmountAddCurr = projectLedgEntry.Add_curr_amount_to_recognize.Sub(projectLedgEntry.Add_curr_amount_recognized)
	} else {
		adjAmount = projectLedgEntry.Amt_to_post_to_g_l.Sub(projectLedgEntry.Amt_posted_to_g_l)
		adjAmountAddCurr = projectLedgEntry.Add_curr_amt_to_post_to_g_l.Sub(projectLedgEntry.Add_curr_amt_posted_to_g_l)
	}
	if !adjAmount.IsZero() || !adjAmountAddCurr.IsZero() {
		projectPostingBuffer := &projectPostingBuffer{
			projectNo:      projectLedgEntry.Project_no,
			entryType:      projectLedgEntry.Entry_type,
			dimension1Code: projectLedgEntry.Dimension_1_code,
			dimension2Code: projectLedgEntry.Dimension_2_code,
		}
		if r.recognition {
			projectPostingBuffer.postingGroup = r.project.Project_posting_group
		} else {
			projectPostingBuffer.postingGroupType = projectLedgEntry.Type
			projectPostingBuffer.postingGroup = projectLedgEntry.Project_posting_group
			projectPostingBuffer.genBusPostingGrp = projectLedgEntry.Gen_bus_posting_group
			projectPostingBuffer.genProdPostingGrp = projectLedgEntry.Gen_prod_posting_group
		}
		if projectPostingBuffer.entryType == gtables.ProjectLedgerEntry_Entry_type.Usage {
			r.totalUsage = r.totalUsage.Add(adjAmount)
		} else {
			r.totalPrice = r.totalPrice.Add(adjAmount)
		}
		r.updateProjectPostingBuffer(adjAmount, adjAmountAddCurr, projectPostingBuffer)
		if r.recognition {
			projectLedgEntry.Amt_recognized = projectLedgEntry.Amt_to_recognize
			projectLedgEntry.Add_curr_amount_recognized = projectLedgEntry.Add_curr_amount_to_recognize
		} else {
			projectLedgEntry.Amt_posted_to_g_l = projectLedgEntry.Amt_to_post_to_g_l
			projectLedgEntry.Add_curr_amt_posted_to_g_l = projectLedgEntry.Add_curr_amt_to_post_to_g_l
		}
		err := projectLedgEntry.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	if r.postingMethod == postingMethodPerProjectLedgEntr {
		err := r.postProjectPostingBuffer(run)
		if err != nil {
			return err
		}
		r.buffer = map[string]*projectPostingBuffer{}
	}
	return nil
}

// projectOnPostDataItem: per project, the buffer is posted at the end (the classic Job2 loop);
// the user is told the totals
func (r *postProjectAmounts) projectOnPostDataItem(project *gtables.Project, run *Run) error {
	r.window.Close()
	err := r.postProjectPostingBuffer(run)
	if err != nil {
		return err
	}
	r.buffer = map[string]*projectPostingBuffer{}
	run.Message(run.Text("POSTED", r.totalUsage, r.totalPrice))
	return nil
}

// updateProjectPostingBuffer (UpdateJobPostingBuffer): the amount added to the buffer line of
// its key
func (r *postProjectAmounts) updateProjectPostingBuffer(adjAmount, adjAmountAddCurr types.Decimal, projectPostingBuffer2 *projectPostingBuffer) {
	key := projectPostingBuffer2.key()
	projectPostingBuffer, found := r.buffer[key]
	if found {
		projectPostingBuffer.amount = projectPostingBuffer.amount.Add(adjAmount)
		projectPostingBuffer.addCurrAmount = projectPostingBuffer.addCurrAmount.Add(adjAmountAddCurr)
	} else {
		projectPostingBuffer2.amount = adjAmount
		projectPostingBuffer2.addCurrAmount = adjAmountAddCurr
		r.buffer[key] = projectPostingBuffer2
	}
}

// postProjectPostingBuffer (PostJobPostingBuffer): each buffer line as two G/L lines — the
// amount off the adjustment (1214) or WIP account (1213) and onto the WIP (1214) or
// cost/sales account (1213)
func (r *postProjectAmounts) postProjectPostingBuffer(run *Run) error {
	keys := make([]string, 0, len(r.buffer))
	for key := range r.buffer {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		projectPostingBuffer := r.buffer[key]
		project := gtables.NewProject(run)
		err := project.GetOrError(projectPostingBuffer.projectNo)
		if err != nil {
			return err
		}
		var fromAccNo, toAccNo types.Code
		if r.recognition {
			projectPostingGr := gtables.NewProjectPostingGroup(run)
			err = projectPostingGr.GetOrError(projectPostingBuffer.postingGroup)
			if err != nil {
				return err
			}
			fromAccNo, toAccNo, err = recognitionAccounts(projectPostingGr, projectPostingBuffer.entryType)
		} else {
			fromAccNo, err = r.adjustmentAccount(run, projectPostingBuffer)
			if err == nil {
				projectPostingGr := gtables.NewProjectPostingGroup(run)
				err = projectPostingGr.GetOrError(project.Project_posting_group)
				if err == nil {
					toAccNo, err = wipAccount(projectPostingGr, projectPostingBuffer.entryType)
				}
			}
		}
		if err != nil {
			return err
		}
		err = r.postGenJnlLine(run, projectPostingBuffer, fromAccNo, projectPostingBuffer.postingGroup, true)
		if err != nil {
			return err
		}
		err = r.postGenJnlLine(run, projectPostingBuffer, toAccNo, project.Project_posting_group, false)
		if err != nil {
			return err
		}
	}
	return nil
}

// wipAccount: the project posting group's WIP Cost (usage) or WIP Sales (sale) Account
func wipAccount(projectPostingGr *gtables.ProjectPostingGroup, entryType gtables.ProjectLedgerEntryEntry_type) (types.Code, error) {
	if entryType == gtables.ProjectLedgerEntry_Entry_type.Usage {
		return projectPostingGr.Wip_cost_account, projectPostingGr.TestField("wip_cost_account")
	}
	return projectPostingGr.Wip_sales_account, projectPostingGr.TestField("wip_sales_account")
}

// recognitionAccounts: the project posting group's WIP account and cost (usage) or sales (sale)
// account
func recognitionAccounts(projectPostingGr *gtables.ProjectPostingGroup, entryType gtables.ProjectLedgerEntryEntry_type) (types.Code, types.Code, error) {
	if entryType == gtables.ProjectLedgerEntry_Entry_type.Usage {
		err := projectPostingGr.TestField("wip_cost_account")
		if err == nil {
			err = projectPostingGr.TestField("cost_account")
		}
		return projectPostingGr.Wip_cost_account, projectPostingGr.Cost_account, err
	}
	err := projectPostingGr.TestField("wip_sales_account")
	if err == nil {
		err = projectPostingGr.TestField("sales_account")
	}
	return projectPostingGr.Wip_sales_account, projectPostingGr.Sales_account, err
}

// adjustmentAccount: the General Posting Setup's Project Cost (usage) or Sales (sale) Adjmt.
// Account — a G/L account entry without general product posting group posts to its own account
func (r *postProjectAmounts) adjustmentAccount(run *Run, projectPostingBuffer *projectPostingBuffer) (types.Code, error) {
	if (projectPostingBuffer.postingGroupType == gtables.ProjectLedgerEntry_Type.AccountGL) && (projectPostingBuffer.genProdPostingGrp == "") {
		return projectPostingBuffer.postingGroup, nil
	}
	genPostingSetup := gtables.NewGeneralPostingSetup(run)
	err := genPostingSetup.GetOrError(map[string]interface{}{"gen_bus_posting_group": projectPostingBuffer.genBusPostingGrp, "gen_prod_posting_group": projectPostingBuffer.genProdPostingGrp})
	if err != nil {
		return "", err
	}
	if projectPostingBuffer.entryType == gtables.ProjectLedgerEntry_Entry_type.Usage {
		err = genPostingSetup.TestField("project_cost_adjmt_account")
		return genPostingSetup.Project_cost_adjmt_account, err
	}
	err = genPostingSetup.TestField("project_sales_adjmt_account")
	return genPostingSetup.Project_sales_adjmt_account, err
}

// postGenJnlLine: one G/L line of the buffer line — the first takes the amount off (negative),
// the second puts it on; an amount only in the additional reporting currency posts it alone
func (r *postProjectAmounts) postGenJnlLine(run *Run, projectPostingBuffer *projectPostingBuffer, accountNo, postingGroup types.Code, off bool) error {
	amount, addCurrAmount := projectPostingBuffer.amount, projectPostingBuffer.addCurrAmount
	if off {
		amount, addCurrAmount = amount.Neg(), addCurrAmount.Neg()
	}
	genJnlLine := gtables.NewGenJournalLine(run)
	genJnlLine.Init()
	genJnlLine.Posting_date = r.postingDate
	genJnlLine.Document_no = r.postingDocNo
	genJnlLine.Source_code = r.sourceCode
	genJnlLine.System_created_entry = true
	genJnlLine.Account_type = gtables.GenJournalLine_Account_type.GLAccount
	genJnlLine.Account_no = accountNo
	genJnlLine.Dimension_1_code = projectPostingBuffer.dimension1Code
	genJnlLine.Dimension_2_code = projectPostingBuffer.dimension2Code
	genJnlLine.Project_no = projectPostingBuffer.projectNo
	entryType := run.Text("USAGE")
	if projectPostingBuffer.entryType == gtables.ProjectLedgerEntry_Entry_type.Sale {
		entryType = run.Text("SALE")
	}
	genJnlLine.Description = types.Text(system.CopyStr(system.StrSubstNo(r.postingDescription, entryType, postingGroup), 1, genJnlLine.MaxStrLen("description")))
	genJnlLine.Additional_currency_posting = gtables.GenJournalLine_Additional_currency_posting.None
	err := genJnlLine.Validate("amount", amount)
	if err != nil {
		return err
	}
	if r.glSetup.Additional_reporting_currency != "" {
		genJnlLine.Source_currency_code = r.glSetup.Additional_reporting_currency
		genJnlLine.Source_currency_amount = addCurrAmount
	}
	if genJnlLine.Amount.IsZero() && !genJnlLine.Source_currency_amount.IsZero() {
		genJnlLine.Additional_currency_posting = gtables.GenJournalLine_Additional_currency_posting.AdditionalCurrencyAmountOnly
		genJnlLine.Amount = genJnlLine.Source_currency_amount
		genJnlLine.Source_currency_amount = types.Decimal{}
	}
	return r.genJnlPostLine.Run(genJnlLine)
}
