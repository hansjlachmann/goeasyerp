package reports

import (
	"sort"
	"strings"

	"github.com/hansjlachmann/openerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 2 General Journal - Test (Test Report on the general journals — also the sales,
// purchase, cash receipt, payment, fixed asset G/L and recurring ones): per batch its lines
// (a recurring journal's with method, expiration date, frequency and their allocations) with
// what posting would refuse under each line — the accounts and balancing accounts by type (G/L
// account: blocked, posting account, direct posting, dimension posting rules, tax posting
// setup; customer/vendor: blocked, currency, the document already posted, external document
// no.; bank account: blocked, currency, checks; fixed asset: book, posting type, G/L
// integration, dates), the general posting type and posting groups, signs of the amounts, the
// recurring fields and allocations, the posting and document dates, the document no. and gaps
// in the number series, applications and payment discounts — and where documents, dates or
// the batch do not balance; then the reconciliation: the G/L accounts the batch posts to (also
// the receivables, payables and bank accounts of its customers, vendors and banks) with the
// net change in the journal and the balance after posting.
//
// Not migrated: the classic US "Payment Journal - Test" of payment templates (they print this
// report), Budgeted FA No. (budgeted assets are not migrated).
const GenJnlTestID = 2

func init() {
	Register(GenJnlTestID, "general-journal-test", func() Report { return &genJnlTest{} })
}

// glAccNetChange is a line of the reconciliation (temporary G/L Account Net Change)
type glAccNetChange struct {
	no                  types.Code
	name                types.Text
	netChangeInJnl      types.Decimal
	balanceAfterPosting types.Decimal
}

type genJnlTest struct {
	jnlTest
	batch       *gtables.GenJournalBatch
	template    *gtables.GenJournalTemplate
	salesSetup  *gtables.SalesReceivablesSetup
	purchSetup  *gtables.PurchasesPayablesSetup
	amountError bool

	// CheckBalance: the balances of the document, the date and the batch (and of their
	// reversing entries), the document and date they belong to
	docBalance, dateBalance, totalBalance                      types.Decimal
	docBalanceReverse, dateBalanceReverse, totalBalanceReverse types.Decimal
	lastDocType                                                gtables.GenJournalLineDocument_type
	lastDocNo                                                  types.Code
	lastDate                                                   types.Date
	currentCustomerVendors                                     int
	taxEntryCreated                                            bool
	custPosting, vendPosting                                   bool
	salesPostingType, purchPostingType                         bool

	allowFAPostingFrom, allowFAPostingTo types.Date

	// the reconciliation (ReconcileLoop): per G/L account, sorted by No. when printed
	netChanges map[types.Code]*glAccNetChange
	reconcile  []*glAccNetChange
}

func (r *genJnlTest) Definition() *Definition {
	return &Definition{
		ContextTable: gtables.GenJournalLineTableName,
		// run from a journal (ReportPrint.PrintGenJnlLine): its template and batch
		OnInitReport: func(run *Run) error {
			line, ok := run.Context().(*gtables.GenJournalLine)
			if ok && line.Journal_template_name != "" {
				run.SetRequestFilter("Gen_Journal_Batch", "journal_template_name", line.Journal_template_name.String())
				run.SetRequestFilter("Gen_Journal_Batch", "name", line.Journal_batch_name.String())
			}
			return nil
		},
		OnPreReport: func(run *Run) error {
			setReportHeadValues(run)
			r.salesSetup = gtables.GetSalesReceivablesSetup(run)
			r.purchSetup = gtables.GetPurchasesPayablesSetup(run)
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Gen_Journal_Batch", Table: gtables.GenJournalBatchTableName, Key: []string{"journal_template_name", "name"},
			RequestFilterFields: []string{"journal_template_name", "name"},
			OnAfterGetRecord: Trigger(func(batch *gtables.GenJournalBatch, run *Run) error {
				r.batch = batch
				r.template = gtables.NewGenJournalTemplate(run)
				if !r.template.Get(batch.Journal_template_name) {
					r.template.Init()
				}
				run.SetValue("recurring", r.template.Recurring)
				line := gtables.NewGenJournalLine(run)
				line.SetRange("journal_template_name", batch.Journal_template_name)
				line.SetRange("journal_batch_name", batch.Name)
				return jnlTestBatchHasLines(run, line.IsEmpty())
			}),
			DataItems: []*DataItem{{
				Name: "Gen_Journal_Line", Table: gtables.GenJournalLineTableName,
				Key:                 []string{"journal_template_name", "journal_batch_name", "line_no"},
				Link:                map[string]string{"journal_template_name": "journal_template_name", "journal_batch_name": "name"},
				RequestFilterFields: []string{"posting_date"},
				OnPreDataItem:       Trigger(r.lineOnPreDataItem),
				OnAfterGetRecord:    Trigger(r.lineOnAfterGetRecord),
				DataItems: []*DataItem{{
					Name: "Gen_Jnl_Allocation", Table: gtables.GenJnlAllocationTableName,
					Key:  []string{"journal_template_name", "journal_batch_name", "journal_line_no", "line_no"},
					Link: map[string]string{"journal_template_name": "journal_template_name", "journal_batch_name": "journal_batch_name", "journal_line_no": "line_no"},
					OnPreDataItem: Trigger(func(genJnlAlloc *gtables.GenJnlAllocation, run *Run) error {
						if !r.template.Recurring {
							return Break()
						}
						return nil
					}),
					OnAfterGetRecord: Trigger(func(genJnlAlloc *gtables.GenJnlAllocation, run *Run) error {
						genJnlAlloc.CalcFields("account_name")
						run.SetValue("alloc_account_name", genJnlAlloc.Account_name.String())
						return nil
					}),
				}, r.errorLoop()},
			}, {
				Name: "Reconcile_Loop", Table: IntegerTable,
				OnPreDataItem: Trigger(func(rec *Integer, run *Run) error {
					r.reconcile = r.reconcile[:0]
					for _, netChange := range r.netChanges {
						r.reconcile = append(r.reconcile, netChange)
					}
					sort.Slice(r.reconcile, func(i, j int) bool { return r.reconcile[i].no < r.reconcile[j].no })
					rec.SetRange("number", 1, len(r.reconcile))
					return nil
				}),
				OnAfterGetRecord: Trigger(func(rec *Integer, run *Run) error {
					netChange := r.reconcile[rec.Number-1]
					run.SetValue("gl_acc_no", netChange.no.String())
					run.SetValue("gl_acc_name", netChange.name.String())
					run.SetValue("net_change_in_jnl", netChange.netChangeInJnl)
					run.SetValue("balance_after_posting", netChange.balanceAfterPosting)
					return nil
				}),
			}},
		}},
	}
}

// lineOnPreDataItem: a recurring journal's lines up to the work date (not filtered on the
// posting or expiration date); the batch's number series; the balances and the reconciliation
// start afresh
func (r *genJnlTest) lineOnPreDataItem(line *gtables.GenJournalLine, run *Run) error {
	// the user's filters, before a recurring journal's own (GETFILTERS in OnPreReport)
	setFilterText(run, line)
	r.startBatch(run, line, r.template.Recurring, r.batch.No_series)
	r.currentCustomerVendors = 0
	r.taxEntryCreated = false
	r.netChanges = map[types.Code]*glAccNetChange{}
	return nil
}

func (r *genJnlTest) lineOnAfterGetRecord(line *gtables.GenJournalLine, run *Run) error {
	language := fcodeunits.CurrentLanguage()
	r.errors = nil
	caption := func(field string) string { return line.GetFieldCaption(field, language) }
	// Amount (LCY) and Balance (LCY) are kept by the line's triggers (UpdateLineBalance)
	if line.Currency_code == "" {
		line.Amount_lcy = line.Amount
	}

	if !tables.GenJnlLineEmptyLine(line) {
		r.makeRecurringText(run, line)
		r.amountError = false

		if line.Account_no == "" && line.Bal_account_no == "" {
			r.addError(run.Text("ACC_OR_BAL_ACC_SPECIFIED", caption("account_no"), caption("bal_account_no")))
		} else {
			if line.Account_type != gtables.GenJournalLine_Account_type.FixedAsset && line.Bal_account_type != gtables.GenJournalLine_Bal_account_type.FixedAsset {
				r.testFixedAssetFields(run, line)
			}
		}

		if line.Account_no != "" {
			r.checkAccountType(run, line, false)
		}
		if line.Bal_account_no != "" {
			r.checkAccountType(run, line, true)
		}

		if line.Account_no != "" && !line.System_created_entry && line.Gen_posting_type == gtables.GenJournalLine_Gen_posting_type.Blank &&
			line.Amount.IsZero() && line.Account_type != gtables.GenJournalLine_Account_type.FixedAsset {
			r.addError(run.Text("MUST_BE_SPECIFIED", caption("amount")))
		}

		r.checkRecurringLine(run, line)
		r.checkAllocations(run, line)

		bothGL := line.Account_type == gtables.GenJournalLine_Account_type.GLAccount && line.Bal_account_type == gtables.GenJournalLine_Bal_account_type.GLAccount
		if line.Posting_date.IsZero() {
			r.addError(run.Text("MUST_BE_SPECIFIED", caption("posting_date")))
		} else {
			if line.Posting_date.IsClosing() {
				if !bothGL {
					r.addError(run.Text("ONLY_CLOSING_FOR_GL", caption("posting_date")))
				}
			}
			if r.dateNotAllowed(run, line.Posting_date) {
				r.addError(run.Text("DATE_NOT_ALLOWED", formatDate(line.Posting_date, language)))
			}
			if r.batch.No_series != "" {
				if r.noSeries != nil && r.noSeries.Date_order && line.Posting_date.Before(r.lastPostingDate) {
					r.addError(run.Text("NOT_IN_DATE_ORDER"))
				}
				r.lastPostingDate = line.Posting_date
			}
		}

		if !line.Document_date.IsZero() {
			if line.Document_date.IsClosing() && !bothGL {
				r.addError(run.Text("ONLY_CLOSING_FOR_GL", caption("document_date")))
			}
		}

		if line.Document_no == "" {
			r.addError(run.Text("MUST_BE_SPECIFIED", caption("document_no")))
		} else {
			r.checkDocumentNo(run, r.batch.No_series, line.Document_no)
		}

		custVendFA := func(accountType int) bool {
			return accountType == int(gtables.GenJournalLine_Account_type.Customer) || accountType == int(gtables.GenJournalLine_Account_type.Vendor) ||
				accountType == int(gtables.GenJournalLine_Account_type.FixedAsset)
		}
		if custVendFA(int(line.Account_type)) && custVendFA(int(line.Bal_account_type)) {
			r.addError(run.Text("GL_OR_BANK", caption("account_type"), caption("bal_account_type")))
		}

		if line.Amount.Mul(line.Amount_lcy).IsNegative() {
			r.addError(run.Text("SAME_SIGN", caption("amount_lcy"), caption("amount")))
		}

		if bothGL {
			if line.Applies_to_doc_no != "" {
				r.addError(run.Text("CANNOT_BE_SPECIFIED", caption("applies_to_doc_no")))
			}
		}

		if bothGL || line.Document_type != gtables.GenJournalLine_Document_type.Invoice {
			if !line.Pmt_discount_date.IsZero() {
				r.addError(run.Text("CANNOT_BE_SPECIFIED", caption("pmt_discount_date")))
			}
			if !line.Payment_discount_percent.IsZero() {
				r.addError(run.Text("MUST_BE_ZERO", caption("payment_discount_percent")))
			}
		}

		if bothGL || line.Applies_to_doc_no != "" {
			if line.Applies_to_id != "" {
				r.addError(run.Text("CANNOT_BE_SPECIFIED", caption("applies_to_id")))
			}
		}

		if line.Account_type != gtables.GenJournalLine_Account_type.BankAccount && line.Bal_account_type != gtables.GenJournalLine_Bal_account_type.BankAccount {
			if line.Bank_payment_type != gtables.GenJournalLine_Bank_payment_type.Blank {
				r.addError(run.Text("CANNOT_BE_SPECIFIED", caption("bank_payment_type")))
			}
		}

		accName := r.checkAccount(run, line)
		balAccName := ""
		if line.Bal_account_no != "" {
			exchanged := gtables.NewGenJournalLine(run)
			exchanged.Copy(line)
			genjnl.ExchAccGLJnlLine(exchanged)
			balAccName = r.checkAccount(run, exchanged)
		}
		run.SetValue("acc_name", accName)
		run.SetValue("bal_acc_name", balAccName)
	}

	r.checkBalance(run, line)
	return nil
}

// checkAccountType: the checks of the account's (balancing: the balancing account's) type —
// general posting type and posting groups, tax amounts, signs, project, checks, fixed assets
func (r *genJnlTest) checkAccountType(run *Run, line *gtables.GenJournalLine, bal bool) {
	language := fcodeunits.CurrentLanguage()
	caption := func(field string) string { return line.GetFieldCaption(field, language) }
	accountType := int(line.Account_type)
	genPostingType := int(line.Gen_posting_type)
	genBus, genProd, taxBus, taxProd := line.Gen_bus_posting_group, line.Gen_prod_posting_group, line.Tax_bus_posting_group, line.Tax_prod_posting_group
	prefix, accountTypeField, accountTypeValue := "", "account_type", line.Account_type.String()
	taxAmount, taxBaseAmount, taxAmountLCY, taxBaseAmountLCY := line.Tax_amount, line.Tax_base_amount, line.Tax_amount_lcy, line.Tax_base_amount_lcy
	amount, amountLCY := line.Amount, line.Amount_lcy
	if bal {
		accountType = int(line.Bal_account_type)
		genPostingType = int(line.Bal_gen_posting_type)
		genBus, genProd, taxBus, taxProd = line.Bal_gen_bus_posting_group, line.Bal_gen_prod_posting_group, line.Bal_tax_bus_posting_group, line.Bal_tax_prod_posting_group
		prefix, accountTypeField, accountTypeValue = "bal_", "bal_account_type", line.Bal_account_type.String()
		taxAmount, taxBaseAmount, taxAmountLCY, taxBaseAmountLCY = line.Bal_tax_amount, line.Bal_tax_base_amount, line.Bal_tax_amount_lcy, line.Bal_tax_base_amount_lcy
		amount, amountLCY = amount.Neg(), amountLCY.Neg()
	}
	accountTypeCaption := optionCaption(gtables.GenJournalLineTableName, accountTypeField, accountTypeValue, language)
	groupsGiven := genBus != "" || genProd != "" || taxBus != "" || taxProd != ""
	groupsNotCompleted := func() {
		r.addError(run.Text("GROUPS_NOT_COMPLETED", caption(prefix+"gen_bus_posting_group"), caption(prefix+"gen_prod_posting_group"),
			caption(prefix+"tax_bus_posting_group"), caption(prefix+"tax_prod_posting_group"), caption(accountTypeField), accountTypeCaption))
	}
	genPostingTypeBlank := func() {
		if genPostingType != 0 {
			r.addError(run.Text("MUST_BE_BLANK_WHEN", caption(prefix+"gen_posting_type"), caption(accountTypeField), accountTypeCaption))
		}
	}
	taxMustBe := "TAX_PLUS_BASE_MUST_BE"
	if bal {
		taxMustBe = "BAL_TAX_PLUS_BASE_MUST_BE"
	}

	switch accountType {
	case int(gtables.GenJournalLine_Account_type.GLAccount):
		if groupsGiven {
			if genPostingType == 0 {
				r.addError(run.Text("MUST_BE_SPECIFIED", caption(prefix+"gen_posting_type")))
			}
		}
		if genPostingType != 0 && line.Vat_posting == gtables.GenJournalLine_Vat_posting.AutomaticVATEntry {
			if !taxAmount.Add(taxBaseAmount).Equal(amount) {
				r.addError(run.Text(taxMustBe, caption(prefix+"tax_amount"), caption(prefix+"tax_base_amount"), caption("amount")))
			}
			if line.Currency_code != "" {
				if !taxAmountLCY.Add(taxBaseAmountLCY).Equal(amountLCY) {
					r.addError(run.Text(taxMustBe, caption(prefix+"tax_amount_lcy"), caption(prefix+"tax_base_amount_lcy"), caption("amount_lcy")))
				}
			}
		}
	case int(gtables.GenJournalLine_Account_type.Customer), int(gtables.GenJournalLine_Account_type.Vendor):
		genPostingTypeBlank()
		if groupsGiven {
			groupsNotCompleted()
		}
		if line.Document_type != gtables.GenJournalLine_Document_type.Blank {
			isCustomer := accountType == int(gtables.GenJournalLine_Account_type.Customer)
			paymentOrCrMemo := line.Document_type == gtables.GenJournalLine_Document_type.Payment || line.Document_type == gtables.GenJournalLine_Document_type.CreditMemo
			// the balancing account's amount is the line's with the opposite sign
			if isCustomer == paymentOrCrMemo {
				if amount.IsPositive() && !r.amountError {
					r.amountError = true
					r.addError(run.Text(map[bool]string{false: "MUST_BE_NEGATIVE", true: "MUST_BE_POSITIVE"}[bal], caption("amount")))
				}
			} else {
				if amount.IsNegative() && !r.amountError {
					r.amountError = true
					r.addError(run.Text(map[bool]string{false: "MUST_BE_POSITIVE", true: "MUST_BE_NEGATIVE"}[bal], caption("amount")))
				}
			}
		}
		if bal {
			if line.Amount.Mul(line.Sales_purch_lcy).IsPositive() {
				r.addError(run.Text("DIFFERENT_SIGN", caption("sales_purch_lcy"), caption("amount")))
			}
		} else {
			if line.Amount.Mul(line.Sales_purch_lcy).IsNegative() {
				r.addError(run.Text("SAME_SIGN", caption("sales_purch_lcy"), caption("amount")))
			}
		}
		if line.Project_no != "" {
			r.addError(run.Text("CANNOT_BE_SPECIFIED", caption("project_no")))
		}
	case int(gtables.GenJournalLine_Account_type.BankAccount):
		genPostingTypeBlank()
		if groupsGiven {
			groupsNotCompleted()
		}
		if line.Project_no != "" {
			r.addError(run.Text("CANNOT_BE_SPECIFIED", caption("project_no")))
		}
		if amount.IsNegative() && line.Bank_payment_type == gtables.GenJournalLine_Bank_payment_type.ComputerCheck {
			if !line.Check_printed {
				r.addError(run.Text("MUST_BE_YES", caption("check_printed")))
			}
		}
	case int(gtables.GenJournalLine_Account_type.FixedAsset):
		r.testFixedAsset(run, line)
	}
}

// checkRecurringLine (CheckRecurringLine): a recurring journal's line has a method and a
// frequency and no balancing account, a variable method an amount, a balance method none, a
// reversing method no fixed asset; another journal's line has neither method nor frequency
func (r *genJnlTest) checkRecurringLine(run *Run, line *gtables.GenJournalLine) {
	language := fcodeunits.CurrentLanguage()
	caption := func(field string) string { return line.GetFieldCaption(field, language) }
	method := line.Recurring_method
	if r.template.Recurring {
		if method == gtables.GenJournalLine_Recurring_method.Blank {
			r.addError(run.Text("MUST_BE_SPECIFIED", caption("recurring_method")))
		}
		if line.Recurring_frequency == "" {
			r.addError(run.Text("MUST_BE_SPECIFIED", caption("recurring_frequency")))
		}
		if line.Bal_account_no != "" {
			r.addError(run.Text("RECURRING_BAL_ACC", caption("bal_account_no")))
		}
		switch method {
		case gtables.GenJournalLine_Recurring_method.VVariable, gtables.GenJournalLine_Recurring_method.RVReversingVariable:
			if line.Amount.IsZero() && !r.amountError {
				r.amountError = true
				r.addError(run.Text("MUST_BE_SPECIFIED", caption("amount")))
			}
		case gtables.GenJournalLine_Recurring_method.BBalance, gtables.GenJournalLine_Recurring_method.RBReversingBalance:
			if !line.Amount.IsZero() && !r.amountError {
				r.amountError = true
				r.addError(run.Text("MUST_BE_ZERO", caption("amount")))
			}
		}
		if method > gtables.GenJournalLine_Recurring_method.VVariable {
			methodCaption := optionCaption(gtables.GenJournalLineTableName, "recurring_method", method.String(), language)
			if line.Account_type == gtables.GenJournalLine_Account_type.FixedAsset {
				r.addError(run.Text("RECURRING_METHOD_NOT_WHEN", caption("recurring_method"), methodCaption, caption("account_type"),
					optionCaption(gtables.GenJournalLineTableName, "account_type", line.Account_type.String(), language)))
			}
			if line.Bal_account_type == gtables.GenJournalLine_Bal_account_type.FixedAsset {
				r.addError(run.Text("RECURRING_METHOD_NOT_WHEN", caption("recurring_method"), methodCaption, caption("bal_account_type"),
					optionCaption(gtables.GenJournalLineTableName, "bal_account_type", line.Bal_account_type.String(), language)))
			}
		}
		return
	}
	if method != gtables.GenJournalLine_Recurring_method.Blank {
		r.addError(run.Text("CANNOT_BE_SPECIFIED", caption("recurring_method")))
	}
	if line.Recurring_frequency != "" {
		r.addError(run.Text("CANNOT_BE_SPECIFIED", caption("recurring_frequency")))
	}
}

// checkAllocations (CheckAllocations): allocations with amounts only in recurring journals, and
// each of them with an account
func (r *genJnlTest) checkAllocations(run *Run, line *gtables.GenJournalLine) {
	genJnlAlloc := gtables.NewGenJnlAllocation(run)
	genJnlAlloc.SetRange("journal_template_name", line.Journal_template_name)
	genJnlAlloc.SetRange("journal_batch_name", line.Journal_batch_name)
	genJnlAlloc.SetRange("journal_line_no", line.Line_no)
	genJnlAlloc.SetFilter("amount", "<>0")
	if genJnlAlloc.FindFirst() {
		if !r.template.Recurring {
			r.addError(run.Text("ALLOC_ONLY_RECURRING"))
		} else {
			genJnlAlloc.SetRange("account_no", types.Code(""))
			if genJnlAlloc.FindFirst() {
				r.addError(run.Text("ALLOC_ACCOUNT_NO", genJnlAlloc.GetFieldCaption("account_no", fcodeunits.CurrentLanguage()), genJnlAlloc.Count()))
			}
		}
	}
}

// makeRecurringText: the texts as posting writes them
func (r *genJnlTest) makeRecurringText(run *Run, line *gtables.GenJournalLine) {
	if !line.Posting_date.IsZero() && line.Account_no != "" && line.Recurring_method != gtables.GenJournalLine_Recurring_method.Blank {
		docNo, description := tables.RecurringText(run, line.Posting_date, line.Document_no, line.Description, 20, 50)
		line.Document_no, line.Description = docNo, description
	}
}

// checkBalance (CheckBalance): the line's Balance (LCY) adds to its document's, date's and the
// batch's balance; where the next line starts another document (Force Doc. Balance), another
// date or there is none, the finished one must balance — and with Force Doc. Balance a
// document posting tax may have one customer or vendor only
func (r *genJnlTest) checkBalance(run *Run, line *gtables.GenJournalLine) {
	language := fcodeunits.CurrentLanguage()
	docTypeCaption := func(docType gtables.GenJournalLineDocument_type) string {
		// a blank document type prints as nothing (FORMAT of the blank option)
		if docType == gtables.GenJournalLine_Document_type.Blank {
			return ""
		}
		return optionCaption(gtables.GenJournalLineTableName, "document_type", docType.String(), language)
	}
	lastLineNo := line.Line_no
	nextGenJnlLine := gtables.NewGenJournalLine(run)
	nextGenJnlLine.Copy(line)
	if !nextGenJnlLine.Find(">") {
		nextGenJnlLine.Copy(line)
	}
	r.makeRecurringText(run, nextGenJnlLine)

	if !tables.GenJnlLineEmptyLine(line) {
		r.docBalance = r.docBalance.Add(line.Balance_lcy)
		r.dateBalance = r.dateBalance.Add(line.Balance_lcy)
		r.totalBalance = r.totalBalance.Add(line.Balance_lcy)
		if line.Recurring_method >= gtables.GenJournalLine_Recurring_method.RFReversingFixed {
			r.docBalanceReverse = r.docBalanceReverse.Add(line.Balance_lcy)
			r.dateBalanceReverse = r.dateBalanceReverse.Add(line.Balance_lcy)
			r.totalBalanceReverse = r.totalBalanceReverse.Add(line.Balance_lcy)
		}
		r.lastDocType = line.Document_type
		r.lastDocNo = line.Document_no
		r.lastDate = line.Posting_date
		if r.totalBalance.IsZero() {
			r.currentCustomerVendors = 0
			r.taxEntryCreated = false
		}
		if r.template.Force_doc_balance {
			salesOrPurchase := func(t int) bool {
				return t == int(gtables.GenJournalLine_Gen_posting_type.Purchase) || t == int(gtables.GenJournalLine_Gen_posting_type.Sale)
			}
			r.taxEntryCreated = r.taxEntryCreated ||
				(line.Account_type == gtables.GenJournalLine_Account_type.GLAccount && line.Account_no != "" && salesOrPurchase(int(line.Gen_posting_type))) ||
				(line.Bal_account_type == gtables.GenJournalLine_Bal_account_type.GLAccount && line.Bal_account_no != "" && salesOrPurchase(int(line.Bal_gen_posting_type)))
			custVend := func(t int) bool {
				return t == int(gtables.GenJournalLine_Account_type.Customer) || t == int(gtables.GenJournalLine_Account_type.Vendor)
			}
			if (custVend(int(line.Account_type)) && line.Account_no != "") || (custVend(int(line.Bal_account_type)) && line.Bal_account_no != "") {
				r.currentCustomerVendors++
			}
			if r.currentCustomerVendors > 1 && r.taxEntryCreated {
				r.addError(run.Text("SEPARATE_EMPTY_LINE", strings.TrimSpace(docTypeCaption(line.Document_type)+" "+line.Document_no.String()), formatDate(line.Posting_date, language)))
			}
		}
	}

	next := nextGenJnlLine
	if r.template.Force_doc_balance && !r.lastDate.IsZero() && r.lastDocNo != "" &&
		(next.Posting_date != r.lastDate || next.Document_type != r.lastDocType || next.Document_no != r.lastDocNo || next.Line_no == lastLineNo) {
		switch {
		case !r.docBalance.IsZero():
			r.addError(run.Text("DOC_OUT_OF_BALANCE", strings.TrimSpace(docTypeCaption(r.lastDocType)+" "+r.lastDocNo.String()), formatValue(r.docBalance, "", language)))
		case !r.docBalanceReverse.IsZero():
			r.addError(run.Text("DOC_REVERSING_OUT_OF_BALANCE", strings.TrimSpace(docTypeCaption(r.lastDocType)+" "+r.lastDocNo.String()), formatValue(r.docBalanceReverse, "", language)))
		}
		r.docBalance = types.Decimal{}
		r.docBalanceReverse = types.Decimal{}
		if next.Posting_date != r.lastDate || next.Document_type != r.lastDocType || next.Document_no != r.lastDocNo {
			r.currentCustomerVendors = 0
			r.taxEntryCreated = false
			r.custPosting = false
			r.vendPosting = false
			r.salesPostingType = false
			r.purchPostingType = false
		}
	}

	if !r.lastDate.IsZero() && (next.Posting_date != r.lastDate || next.Line_no == lastLineNo) {
		switch {
		case !r.dateBalance.IsZero():
			r.addError(run.Text("DATE_OUT_OF_BALANCE", formatDate(r.lastDate, language), formatValue(r.dateBalance, "", language)))
		case !r.dateBalanceReverse.IsZero():
			r.addError(run.Text("DATE_REVERSING_OUT_OF_BALANCE", formatDate(r.lastDate, language), formatValue(r.dateBalanceReverse, "", language)))
		}
		r.docBalance = types.Decimal{}
		r.docBalanceReverse = types.Decimal{}
		r.dateBalance = types.Decimal{}
		r.dateBalanceReverse = types.Decimal{}
	}

	if next.Line_no == lastLineNo {
		switch {
		case !r.totalBalance.IsZero():
			r.addError(run.Text("TOTAL_OUT_OF_BALANCE", formatValue(r.totalBalance, "", language)))
		case !r.totalBalanceReverse.IsZero():
			r.addError(run.Text("TOTAL_REVERSING_OUT_OF_BALANCE", formatValue(r.totalBalanceReverse, "", language)))
		}
		r.docBalance = types.Decimal{}
		r.docBalanceReverse = types.Decimal{}
		r.dateBalance = types.Decimal{}
		r.dateBalanceReverse = types.Decimal{}
		r.totalBalance = types.Decimal{}
		r.totalBalanceReverse = types.Decimal{}
		r.lastDate = types.Date{}
		r.lastDocType = gtables.GenJournalLine_Document_type.Blank
		r.lastDocNo = ""
	}
}

// reconcileGLAccNo (ReconcileGLAccNo): the G/L account's net change in the journal and its
// balance after posting (the balance at date plus the net change)
func (r *genJnlTest) reconcileGLAccNo(run *Run, glAccNo types.Code, reconcileAmount types.Decimal) {
	netChange, ok := r.netChanges[glAccNo]
	if !ok {
		glAcc := gtables.NewGLAccount(run)
		if !glAcc.Get(glAccNo) {
			glAcc.Init()
			glAcc.No = glAccNo
		}
		glAcc.CalcFields("balance_at_date")
		netChange = &glAccNetChange{no: glAcc.No, name: glAcc.Name, balanceAfterPosting: glAcc.Balance_at_date}
		r.netChanges[glAccNo] = netChange
	}
	netChange.netChangeInJnl = netChange.netChangeInJnl.Add(reconcileAmount)
	netChange.balanceAfterPosting = netChange.balanceAfterPosting.Add(reconcileAmount)
}

// reconcileAmount: the line's Amount (LCY) without its tax (ROUND("Amount ($)" / (1 + "Tax %" / 100)))
func reconcileAmount(line *gtables.GenJournalLine) types.Decimal {
	divisor := types.MustDecimal("1").Add(line.Tax_percent.Div(types.MustDecimal("100")))
	return system.Round(line.Amount_lcy.Div(divisor), types.MustDecimal("0.01"), "=")
}

// checkAccount: the account of the line's Account Type (the balancing one on the exchanged
// line) — its name
func (r *genJnlTest) checkAccount(run *Run, line *gtables.GenJournalLine) string {
	if line.Account_no == "" {
		return ""
	}
	switch line.Account_type {
	case gtables.GenJournalLine_Account_type.GLAccount:
		return r.checkGLAcc(run, line)
	case gtables.GenJournalLine_Account_type.Customer:
		return r.checkCust(run, line)
	case gtables.GenJournalLine_Account_type.Vendor:
		return r.checkVend(run, line)
	case gtables.GenJournalLine_Account_type.BankAccount:
		return r.checkBankAcc(run, line)
	case gtables.GenJournalLine_Account_type.FixedAsset:
		return r.checkFixedAsset(run, line)
	}
	return ""
}

// mustBeFor: "%1 must be %2 for %3 %4." — a field of a table's record
func (r *genJnlTest) mustBeFor(run *Run, fieldCaption, value, tableCaption string, no types.Code) {
	r.addError(run.Text("MUST_BE_FOR", fieldCaption, value, tableCaption, no))
}

func (r *genJnlTest) checkGLAcc(run *Run, line *gtables.GenJournalLine) string {
	language := fcodeunits.CurrentLanguage()
	caption := func(field string) string { return line.GetFieldCaption(field, language) }
	glAcc := gtables.NewGLAccount(run)
	if !glAcc.Get(line.Account_no) {
		r.addError(run.Text("NOT_EXIST", glAcc.TableCaption(language), line.Account_no))
		return ""
	}
	accName := glAcc.Name.String()
	if glAcc.Blocked {
		r.mustBeFor(run, glAcc.GetFieldCaption("blocked", language), formatValue(false, "", language), glAcc.TableCaption(language), line.Account_no)
	}
	if glAcc.Account_type != gtables.GLAccount_Account_type.Posting {
		r.mustBeFor(run, glAcc.GetFieldCaption("account_type", language), glAcc.GetOptionCaption("account_type", "posting", language), glAcc.TableCaption(language), line.Account_no)
	}
	if !line.System_created_entry {
		if !line.Posting_date.IsClosing() {
			if !glAcc.Direct_posting {
				r.mustBeFor(run, glAcc.GetFieldCaption("direct_posting", language), formatValue(true, "", language), glAcc.TableCaption(language), line.Account_no)
			}
		}
	}

	r.checkDimensionPosting(run, glAcc, int(glAcc.Dimension_1_posting), glAcc.Dimension_1_code, line.Dimension_1_code, caption("dimension_1_code"))
	r.checkDimensionPosting(run, glAcc, int(glAcc.Dimension_2_posting), glAcc.Dimension_2_code, line.Dimension_2_code, caption("dimension_2_code"))

	if line.Gen_posting_type != gtables.GenJournalLine_Gen_posting_type.Blank {
		switch line.Gen_posting_type {
		case gtables.GenJournalLine_Gen_posting_type.Sale:
			r.salesPostingType = true
		case gtables.GenJournalLine_Gen_posting_type.Purchase:
			r.purchPostingType = true
		}
		r.testPostingType(run)

		taxPostingSetup := gtables.NewTaxPostingSetup(run)
		if !taxPostingSetup.Get(map[string]interface{}{"tax_bus_posting_group": line.Tax_bus_posting_group, "tax_prod_posting_group": line.Tax_prod_posting_group}) {
			r.addError(run.Text("SETUP_NOT_EXIST", taxPostingSetup.TableCaption(language), line.Tax_bus_posting_group, line.Tax_prod_posting_group))
		} else {
			if int(line.Tax_calculation_type) != int(taxPostingSetup.Tax_calculation_type) {
				r.addError(run.Text("MUST_BE", caption("tax_calculation_type"),
					optionCaption(gtables.TaxPostingSetupTableName, "tax_calculation_type", taxPostingSetup.Tax_calculation_type.String(), language)))
			}
		}
	}

	if glAcc.Reconciliation_account {
		r.reconcileGLAccNo(run, line.Account_no, reconcileAmount(line))
	}
	return accName
}

// checkDimensionPosting: the G/L account's rule for a dimension — Code Mandatory, Same Code, No
// Code (the classic Department/Project Posting)
func (r *genJnlTest) checkDimensionPosting(run *Run, glAcc *gtables.GLAccount, posting int, glAccCode, lineCode types.Code, fieldCaption string) {
	language := fcodeunits.CurrentLanguage()
	switch posting {
	case int(gtables.GLAccount_Dimension_1_posting.CodeMandatory):
		if lineCode == "" {
			r.addError(run.Text("MANDATORY_WHEN_POSTING", fieldCaption, glAcc.TableCaption(language), glAcc.No))
		}
	case int(gtables.GLAccount_Dimension_1_posting.SameCode):
		if lineCode != glAccCode {
			r.addError(run.Text("MUST_BE_WHEN_POSTING", fieldCaption, glAccCode, glAcc.TableCaption(language), glAcc.No))
		}
	case int(gtables.GLAccount_Dimension_1_posting.NoCode):
		if lineCode != "" {
			r.addError(run.Text("MUST_BE_BLANK_WHEN_POSTING", fieldCaption, glAcc.TableCaption(language), glAcc.No))
		}
	}
}

// checkCurrency: a currency code the Currency table has
func (r *genJnlTest) checkCurrency(run *Run, currencyCode types.Code) {
	if currencyCode != "" {
		currency := gtables.NewCurrency(run)
		if !currency.Get(currencyCode) {
			r.addError(run.Text("CURRENCY_NOT_FOUND", currencyCode))
		}
	}
}

// isPostedDocumentType: an invoice, credit memo or finance charge memo — a document the
// ledger has once
func isPostedDocumentType(docType gtables.GenJournalLineDocument_type) bool {
	return docType == gtables.GenJournalLine_Document_type.Invoice || docType == gtables.GenJournalLine_Document_type.CreditMemo ||
		docType == gtables.GenJournalLine_Document_type.FinanceChargeMemo
}

func (r *genJnlTest) checkCust(run *Run, line *gtables.GenJournalLine) string {
	language := fcodeunits.CurrentLanguage()
	cust := gtables.NewCustomer(run)
	if !cust.Get(line.Account_no) {
		r.addError(run.Text("NOT_EXIST", cust.TableCaption(language), line.Account_no))
		return ""
	}
	accName := cust.Name.String()
	if cust.Blocked {
		r.mustBeFor(run, cust.GetFieldCaption("blocked", language), formatValue(false, "", language), cust.TableCaption(language), line.Account_no)
	}
	r.checkCurrency(run, line.Currency_code)

	r.custPosting = true
	r.testPostingType(run)

	docTypeCaption := optionCaption(gtables.GenJournalLineTableName, "document_type", line.Document_type.String(), language)
	if line.Recurring_method == gtables.GenJournalLine_Recurring_method.Blank {
		if isPostedDocumentType(line.Document_type) {
			oldCustLedgEntry := gtables.NewCustomerLedgerEntry(run)
			oldCustLedgEntry.SetRange("document_type", gtables.CustomerLedgerEntryDocument_type(line.Document_type))
			oldCustLedgEntry.SetRange("document_no", line.Document_no)
			if oldCustLedgEntry.FindFirst() {
				r.addError(run.Text("SALES_DOC_EXISTS", docTypeCaption, line.Document_no))
			}
		}
	}

	// the classic TESTFIELD would stop the report: here it is a warning of the line
	if r.salesSetup.Ext_doc_no_mandatory {
		if line.External_document_no == "" {
			r.addError(run.Text("MUST_BE_SPECIFIED", line.GetFieldCaption("external_document_no", language)))
		}
	}

	// Reconcile Receivables Account
	custPostingGrp := gtables.NewCustomerPostingGroup(run)
	if custPostingGrp.Get(cust.Customer_posting_group) {
		if custPostingGrp.Receivables_account != "" {
			r.reconcileGLAccNo(run, custPostingGrp.Receivables_account, reconcileAmount(line))
		}
	}
	return accName
}

func (r *genJnlTest) checkVend(run *Run, line *gtables.GenJournalLine) string {
	language := fcodeunits.CurrentLanguage()
	vend := gtables.NewVendor(run)
	if !vend.Get(line.Account_no) {
		r.addError(run.Text("NOT_EXIST", vend.TableCaption(language), line.Account_no))
		return ""
	}
	accName := vend.Name.String()
	if vend.Blocked {
		r.mustBeFor(run, vend.GetFieldCaption("blocked", language), formatValue(false, "", language), vend.TableCaption(language), line.Account_no)
	}
	r.checkCurrency(run, line.Currency_code)

	r.vendPosting = true
	r.testPostingType(run)

	docTypeCaption := optionCaption(gtables.GenJournalLineTableName, "document_type", line.Document_type.String(), language)
	if line.Recurring_method == gtables.GenJournalLine_Recurring_method.Blank {
		if isPostedDocumentType(line.Document_type) {
			oldVendLedgEntry := gtables.NewVendorLedgerEntry(run)
			oldVendLedgEntry.SetRange("document_type", gtables.VendorLedgerEntryDocument_type(line.Document_type))
			oldVendLedgEntry.SetRange("document_no", line.Document_no)
			if oldVendLedgEntry.FindFirst() {
				r.addError(run.Text("PURCH_DOC_EXISTS", docTypeCaption, line.Document_no))
			}

			if r.purchSetup.Ext_doc_no_mandatory || line.External_document_no != "" {
				// Test vendor number
				if line.External_document_no == "" {
					r.addError(run.Text("MUST_BE_ENTERED", line.GetFieldCaption("external_document_no", language)))
				}
				oldVendLedgEntry.Reset()
				oldVendLedgEntry.SetRange("document_type", gtables.VendorLedgerEntryDocument_type(line.Document_type))
				oldVendLedgEntry.SetRange("external_document_no", line.External_document_no)
				oldVendLedgEntry.SetRange("vendor_no", line.Account_no)
				if oldVendLedgEntry.FindFirst() {
					r.addError(run.Text("PURCH_DOC_EXISTS_VENDOR", docTypeCaption, line.External_document_no))
				}
			}
		}
	}

	// Reconcile Payables Account
	vendPostingGrp := gtables.NewVendorPostingGroup(run)
	if vendPostingGrp.Get(vend.Vendor_posting_group) {
		if vendPostingGrp.Payables_account != "" {
			r.reconcileGLAccNo(run, vendPostingGrp.Payables_account, reconcileAmount(line))
		}
	}
	return accName
}

func (r *genJnlTest) checkBankAcc(run *Run, line *gtables.GenJournalLine) string {
	language := fcodeunits.CurrentLanguage()
	caption := func(field string) string { return line.GetFieldCaption(field, language) }
	bankAcc := gtables.NewBankAccount(run)
	if !bankAcc.Get(line.Account_no) {
		r.addError(run.Text("NOT_EXIST", bankAcc.TableCaption(language), line.Account_no))
		return ""
	}
	accName := bankAcc.Name.String()
	if bankAcc.Blocked {
		r.mustBeFor(run, bankAcc.GetFieldCaption("blocked", language), formatValue(false, "", language), bankAcc.TableCaption(language), line.Account_no)
	}
	if line.Currency_code != bankAcc.Currency_code && bankAcc.Currency_code != "" {
		r.addError(run.Text("MUST_BE", caption("currency_code"), bankAcc.Currency_code))
	}
	r.checkCurrency(run, line.Currency_code)

	if line.Bank_payment_type != gtables.GenJournalLine_Bank_payment_type.Blank {
		if line.Bank_payment_type == gtables.GenJournalLine_Bank_payment_type.ComputerCheck && line.Amount.IsNegative() {
			if bankAcc.Currency_code != line.Currency_code {
				r.addError(run.Text("BANK_PAYMENT_CURRENCY", caption("bank_payment_type"), caption("currency_code"),
					line.TableCaption(language), bankAcc.TableCaption(language)))
			}
		}
	}

	bankAccPostingGroup := gtables.NewBankAccountPostingGroup(run)
	if bankAccPostingGroup.Get(bankAcc.Bank_acc_posting_group) {
		if bankAccPostingGroup.G_l_bank_account_no != "" {
			r.reconcileGLAccNo(run, bankAccPostingGroup.G_l_bank_account_no, reconcileAmount(line))
		}
	}
	return accName
}

func (r *genJnlTest) checkFixedAsset(run *Run, line *gtables.GenJournalLine) string {
	language := fcodeunits.CurrentLanguage()
	fa := gtables.NewFixedAsset(run)
	if !fa.Get(line.Account_no) {
		r.addError(run.Text("NOT_EXIST", fa.TableCaption(language), line.Account_no))
		return ""
	}
	accName := fa.Description.String()
	if fa.Blocked {
		r.mustBeFor(run, fa.GetFieldCaption("blocked", language), formatValue(false, "", language), fa.TableCaption(language), line.Account_no)
	}
	if fa.Inactive {
		r.mustBeFor(run, fa.GetFieldCaption("inactive", language), formatValue(false, "", language), fa.TableCaption(language), line.Account_no)
	}
	if fa.Budgeted_asset {
		r.addError(run.Text("MUST_NOT_HAVE", fa.TableCaption(language), line.Account_no, fa.GetFieldCaption("budgeted_asset", language), formatValue(true, "", language)))
	}
	deprBook := gtables.NewDepreciationBook(run)
	if deprBook.Get(line.Depreciation_book_code) {
		r.checkFAIntegration(run, line, deprBook)
	} else {
		r.addError(run.Text("NOT_EXIST", deprBook.TableCaption(language), line.Depreciation_book_code))
	}
	faDeprBook := gtables.NewFADepreciationBook(run)
	if !faDeprBook.Get(map[string]interface{}{"fa_no": fa.No, "depreciation_book_code": line.Depreciation_book_code}) {
		r.addError(run.Text("SETUP_NOT_EXIST", faDeprBook.TableCaption(language), fa.No, line.Depreciation_book_code))
	}
	return accName
}

// testFixedAsset (TestFixedAsset): a fixed asset line's posting type, book, posting groups,
// the fields its posting type does not use, its FA posting date and insurance, and what a
// correction (FA Error Entry No.) cannot have
func (r *genJnlTest) testFixedAsset(run *Run, line *gtables.GenJournalLine) {
	language := fcodeunits.CurrentLanguage()
	caption := func(field string) string { return line.GetFieldCaption(field, language) }
	faPostingType := line.Fa_posting_type
	faPostingTypeCaption := optionCaption(gtables.GenJournalLineTableName, "fa_posting_type", faPostingType.String(), language)

	if line.Project_no != "" {
		r.addError(run.Text("FA_MUST_NOT_BE_SPECIFIED", caption("project_no")))
	}
	if faPostingType == gtables.GenJournalLine_Fa_posting_type.Blank {
		r.addError(run.Text("FA_MUST_BE_SPECIFIED", caption("fa_posting_type")))
	}
	if line.Depreciation_book_code == "" {
		r.addError(run.Text("FA_MUST_BE_SPECIFIED", caption("depreciation_book_code")))
	}
	if line.Depreciation_book_code == line.Duplicate_in_depreciation_book {
		r.addError(run.Text("MUST_BE_DIFFERENT", caption("depreciation_book_code"), caption("duplicate_in_depreciation_book")))
	}
	if int(line.Account_type) == int(line.Bal_account_type) {
		r.addError(run.Text("NOT_BOTH", caption("account_type"), caption("bal_account_type"),
			optionCaption(gtables.GenJournalLineTableName, "account_type", line.Account_type.String(), language)))
	}
	acqDisposalMaintenance := faPostingType == gtables.GenJournalLine_Fa_posting_type.AcquisitionCost ||
		faPostingType == gtables.GenJournalLine_Fa_posting_type.Disposal || faPostingType == gtables.GenJournalLine_Fa_posting_type.Maintenance
	checkGroups := func(prefix string, genPostingType int, genBus, genProd types.Code) {
		if acqDisposalMaintenance {
			if genBus != "" || genProd != "" {
				if genPostingType == 0 {
					r.addError(run.Text("MUST_BE_SPECIFIED", caption(prefix+"gen_posting_type")))
				}
			}
		} else {
			if genPostingType != 0 {
				r.addError(run.Text("NOT_SPECIFIED_WHEN", caption(prefix+"gen_posting_type"), caption("fa_posting_type"), faPostingTypeCaption))
			}
			if genBus != "" {
				r.addError(run.Text("NOT_SPECIFIED_WHEN", caption(prefix+"gen_bus_posting_group"), caption("fa_posting_type"), faPostingTypeCaption))
			}
			if genProd != "" {
				r.addError(run.Text("NOT_SPECIFIED_WHEN", caption(prefix+"gen_prod_posting_group"), caption("fa_posting_type"), faPostingTypeCaption))
			}
		}
	}
	if line.Account_type == gtables.GenJournalLine_Account_type.FixedAsset {
		checkGroups("", int(line.Gen_posting_type), line.Gen_bus_posting_group, line.Gen_prod_posting_group)
	}
	if line.Bal_account_type == gtables.GenJournalLine_Bal_account_type.FixedAsset {
		checkGroups("bal_", int(line.Bal_gen_posting_type), line.Bal_gen_bus_posting_group, line.Bal_gen_prod_posting_group)
	}

	notTogether := func(field string) {
		r.addError(run.Text("NOT_TOGETHER", caption(field), caption("fa_posting_type"), faPostingTypeCaption))
	}
	if faPostingType != gtables.GenJournalLine_Fa_posting_type.AcquisitionCost {
		if line.Depr_acquisition_cost {
			notTogether("depr_acquisition_cost")
		}
		if !line.Salvage_value.IsZero() {
			notTogether("salvage_value")
		}
		if faPostingType != gtables.GenJournalLine_Fa_posting_type.Maintenance {
			if !line.Quantity.IsZero() {
				notTogether("quantity")
			}
		}
		if line.Insurance_no != "" {
			notTogether("insurance_no")
		}
	}
	if faPostingType == gtables.GenJournalLine_Fa_posting_type.Maintenance && line.Depr_until_fa_posting_date {
		notTogether("depr_until_fa_posting_date")
	}
	if faPostingType != gtables.GenJournalLine_Fa_posting_type.Maintenance && line.Maintenance_code != "" {
		notTogether("maintenance_code")
	}
	if faPostingType != gtables.GenJournalLine_Fa_posting_type.Depreciation && faPostingType != gtables.GenJournalLine_Fa_posting_type.Custom1 &&
		line.No_of_depreciation_days != 0 {
		notTogether("no_of_depreciation_days")
	}
	if faPostingType == gtables.GenJournalLine_Fa_posting_type.Disposal && line.Fa_reclassification_entry {
		notTogether("fa_reclassification_entry")
	}

	faPostingDate := line.Fa_posting_date
	if faPostingDate.IsZero() {
		faPostingDate = line.Posting_date
	}
	deprBook := gtables.NewDepreciationBook(run)
	if deprBook.Get(line.Depreciation_book_code) {
		if deprBook.Use_same_fa_g_l_posting_dates && line.Posting_date != faPostingDate {
			r.addError(run.Text("IDENTICAL", caption("posting_date"), caption("fa_posting_date")))
		}
	}
	if !faPostingDate.IsZero() {
		if faPostingDate.IsClosing() {
			r.addError(run.Text("CANNOT_BE_CLOSING", caption("fa_posting_date")))
		}
		if r.faDateNotAllowed(run, faPostingDate) {
			r.addError(run.Text("FA_DATE_NOT_ALLOWED", caption("fa_posting_date")))
		}
	}
	faSetup := gtables.GetFASetup(run)
	if faPostingType == gtables.GenJournalLine_Fa_posting_type.AcquisitionCost && line.Insurance_no != "" && line.Depreciation_book_code != faSetup.Insurance_depr_book {
		r.addError(run.Text("INSURANCE_NOT_ACTIVATED", caption("depreciation_book_code"), line.Depreciation_book_code))
	}

	if line.Fa_error_entry_no > 0 {
		notWhen := func(fieldCaption string) {
			r.addError(run.Text("NOT_WHEN_SPECIFIED", fieldCaption, caption("fa_error_entry_no")))
		}
		if line.Depr_until_fa_posting_date {
			notWhen(caption("depr_until_fa_posting_date"))
		}
		if line.Depr_acquisition_cost {
			notWhen(caption("depr_acquisition_cost"))
		}
		if line.Duplicate_in_depreciation_book != "" {
			notWhen(caption("duplicate_in_depreciation_book"))
		}
		if line.Use_duplication_list {
			notWhen(caption("use_duplication_list"))
		}
		if !line.Salvage_value.IsZero() {
			notWhen(caption("salvage_value"))
		}
		if line.Insurance_no != "" {
			notWhen(caption("insurance_no"))
		}
		if line.Recurring_method != gtables.GenJournalLine_Recurring_method.Blank {
			notWhen(caption("recurring_method"))
		}
		if faPostingType == gtables.GenJournalLine_Fa_posting_type.Maintenance {
			notWhen(faPostingTypeCaption)
		}
	}
}

// faDateNotAllowed: the FA posting date is outside the year range or the user's allowed FA
// posting dates (User Setup, else FA Setup)
func (r *genJnlTest) faDateNotAllowed(run *Run, faPostingDate types.Date) bool {
	first, _ := system.DMY2Date(1, 1, 1)
	last, _ := system.DMY2Date(30, 12, 9999)
	if faPostingDate.Before(first) || last.Before(faPostingDate) {
		return true
	}
	if r.allowFAPostingFrom.IsZero() && r.allowFAPostingTo.IsZero() {
		if system.UserId() != "" {
			userSetup := gtables.NewUserSetup(run)
			if userSetup.Get(types.Code(system.UserId())) {
				r.allowFAPostingFrom = userSetup.Allow_fa_posting_from
				r.allowFAPostingTo = userSetup.Allow_fa_posting_to
			}
		}
		if r.allowFAPostingFrom.IsZero() && r.allowFAPostingTo.IsZero() {
			faSetup := gtables.GetFASetup(run)
			r.allowFAPostingFrom = faSetup.Allow_fa_posting_from
			r.allowFAPostingTo = faSetup.Allow_fa_posting_to
		}
		if r.allowFAPostingTo.IsZero() {
			r.allowFAPostingTo = last
		}
	}
	return faPostingDate.Before(r.allowFAPostingFrom) || r.allowFAPostingTo.Before(faPostingDate)
}

// checkFAIntegration (CheckFAIntegration): the book posts the line's FA posting type to the G/L
// (else it belongs in the FA journal); without depreciation integration no depreciation fields
func (r *genJnlTest) checkFAIntegration(run *Run, line *gtables.GenJournalLine, deprBook *gtables.DepreciationBook) {
	language := fcodeunits.CurrentLanguage()
	if line.Fa_posting_type == gtables.GenJournalLine_Fa_posting_type.Blank {
		return
	}
	glIntegration := false
	switch line.Fa_posting_type {
	case gtables.GenJournalLine_Fa_posting_type.AcquisitionCost:
		glIntegration = deprBook.G_l_integration_acq_cost
	case gtables.GenJournalLine_Fa_posting_type.Depreciation:
		glIntegration = deprBook.G_l_integration_depreciation
	case gtables.GenJournalLine_Fa_posting_type.WriteDown:
		glIntegration = deprBook.G_l_integration_write_down
	case gtables.GenJournalLine_Fa_posting_type.Appreciation:
		glIntegration = deprBook.G_l_integration_appreciation
	case gtables.GenJournalLine_Fa_posting_type.Custom1:
		glIntegration = deprBook.G_l_integration_custom_1
	case gtables.GenJournalLine_Fa_posting_type.Custom2:
		glIntegration = deprBook.G_l_integration_custom_2
	case gtables.GenJournalLine_Fa_posting_type.Disposal:
		glIntegration = deprBook.G_l_integration_disposal
	case gtables.GenJournalLine_Fa_posting_type.Maintenance:
		glIntegration = deprBook.G_l_integration_maintenance
	}
	if !glIntegration {
		r.addError(run.Text("GL_INTEGRATION_NOT_POSTED", optionCaption(gtables.GenJournalLineTableName, "fa_posting_type", line.Fa_posting_type.String(), language)))
	}
	if !deprBook.G_l_integration_depreciation {
		if line.Depr_until_fa_posting_date {
			r.addError(run.Text("GL_INTEGRATION_NOT_SPECIFIED", line.GetFieldCaption("depr_until_fa_posting_date", language)))
		}
		if line.Depr_acquisition_cost {
			r.addError(run.Text("GL_INTEGRATION_NOT_SPECIFIED", line.GetFieldCaption("depr_acquisition_cost", language)))
		}
	}
}

// testFixedAssetFields (TestFixedAssetFields): a line without fixed asset has no FA posting type
// or book
func (r *genJnlTest) testFixedAssetFields(run *Run, line *gtables.GenJournalLine) {
	language := fcodeunits.CurrentLanguage()
	if line.Fa_posting_type != gtables.GenJournalLine_Fa_posting_type.Blank {
		r.addError(run.Text("MUST_NOT_BE_SPECIFIED", line.GetFieldCaption("fa_posting_type", language)))
	}
	if line.Depreciation_book_code != "" {
		r.addError(run.Text("MUST_NOT_BE_SPECIFIED", line.GetFieldCaption("depreciation_book_code", language)))
	}
}

// testPostingType (TestPostingType): no customer with a purchase or vendor with a sale in a
// document
func (r *genJnlTest) testPostingType(run *Run) {
	switch {
	case r.custPosting && r.purchPostingType:
		r.addError(run.Text("CUSTOMER_PURCHASE"))
	case r.vendPosting && r.salesPostingType:
		r.addError(run.Text("VENDOR_SALES"))
	}
}
