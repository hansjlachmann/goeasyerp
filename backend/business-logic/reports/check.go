package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/checkmgt"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/formataddress"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 1401 Check (Payments › Print Check on the payment journal, Report Selections usage
// B.Check): the journal's computer check lines balanced on the chosen bank account become
// checks, numbered on from the bank account's Last Check No. Each check page has a stub with the
// documents it pays (the entry it is applied to, the entries of its Applies-to ID, or — One Check
// per Vendor per Document No. — every line of the vendor and document no.), at most ten; a check
// whose documents do not fit is printed VOID and the next check goes on. Each check gets a check
// ledger entry (Printed, or Voided), its lines Check Printed and the check no. as document no.;
// one check for several lines gets a bank line of its total. Reprint Checks first voids the
// printed checks of the journal; Test Print prints one sample check on a test number.
const CheckReportID = 1401

// The check's apply methods (classic ApplyMethod)
const (
	applyMethodPayment = iota
	applyMethodOneLineOneEntry
	applyMethodOneLineID
	applyMethodMoreLinesOneEntry
)

// The balancing account types of a check (classic BalancingType: G/L Account, Customer, Vendor,
// Bank Account — the journal line's account type)
const stubLinesPerCheck = 10

func init() {
	Register(CheckReportID, "check", func() Report { return &checkReport{} })
	DefaultReports[gtables.ReportSelections_Usage.BCheck] = CheckReportID
}

type checkReport struct {
	checkMgt                                    *checkmgt.CheckManagement
	bankAcc2                                    *gtables.BankAccount
	genJnlLine                                  *gtables.GenJournalLine
	moreLines                                   []*gtables.GenJournalLine
	moreLinesIndex                              int
	custLedgEntries                             []*gtables.CustomerLedgerEntry
	vendLedgEntries                             []*gtables.VendorLedgerEntry
	ledgEntryIndex                              int
	useCheckNo                                  types.Code
	balancingType                               gtables.GenJournalLineAccount_type
	balancingNo                                 types.Code
	applyMethod                                 int
	reprintChecks, testPrint, oneCheckPrVendor  bool
	preprintedStub                              bool
	firstPage, foundLast, foundNegative         bool
	remainingAmount, totalLineAmount            types.Decimal
	totalLineDiscount, lineAmount, lineDiscount types.Decimal
	checkToAddr                                 formataddress.AddrArray
	checksPrinted                               int
	language                                    string
}

func (r *checkReport) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "bank_account", Type: "text", Required: true, Relation: &OptionRelation{Table: gtables.BankAccountTableName, Columns: []string{"no", "name"}}},
			{Name: "last_check_no", Type: "text"},
			{Name: "one_check_per_vendor", Type: "bool", Default: "false"},
			{Name: "reprint_checks", Type: "bool", Default: "false"},
			{Name: "test_print", Type: "bool", Default: "false"},
			{Name: "preprinted_stub", Type: "bool", Default: "false"},
		},
		ContextTable: gtables.GenJournalLineTableName,
		Transaction:  true,
		OnInitReport: r.onInitReport,
		OnPreReport:  r.onPreReport,
		DataItems: []*DataItem{{
			Name: "GenJnlLine", Table: gtables.GenJournalLineTableName,
			Key:                 []string{"journal_template_name", "journal_batch_name", "posting_date", "document_no"},
			RequestFilterFields: []string{"journal_template_name", "journal_batch_name", "posting_date"},
			OnPreDataItem:       Trigger(r.genJnlLineOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.genJnlLineOnAfterGetRecord),
			DataItems: []*DataItem{{
				Name: "CheckPages", Table: IntegerTable,
				OnPreDataItem:    Trigger(r.checkPagesOnPreDataItem),
				OnAfterGetRecord: Trigger(r.checkPagesOnAfterGetRecord),
				OnPostDataItem:   Trigger(r.checkPagesOnPostDataItem),
				DataItems: []*DataItem{
					{
						Name: "PrintSettledLoop", Table: IntegerTable,
						OnPreDataItem:    Trigger(r.printSettledLoopOnPreDataItem),
						OnAfterGetRecord: Trigger(r.printSettledLoopOnAfterGetRecord),
					},
					{
						Name: "PrintCheck", Table: IntegerTable,
						OnPreDataItem: Trigger(func(rec *Integer, run *Run) error {
							rec.SetRange("number", 1, 1)
							return nil
						}),
						OnAfterGetRecord: Trigger(r.printCheckOnAfterGetRecord),
					},
				},
			}},
		}},
	}
}

// onInitReport: run from a payment journal, its batch's lines; the batch's bank account and its
// Last Check No.
func (r *checkReport) onInitReport(run *Run) error {
	genJnlLine, ok := run.Context().(*gtables.GenJournalLine)
	if !ok {
		return nil
	}
	templateName, batchName := genJnlLine.Journal_template_name, genJnlLine.Journal_batch_name
	if templateName == "" {
		templateName = genJnlLineFilter(genJnlLine, "journal_template_name")
	}
	if batchName == "" {
		batchName = genJnlLineFilter(genJnlLine, "journal_batch_name")
	}
	if templateName != "" {
		run.SetRequestFilter("GenJnlLine", "journal_template_name", templateName.String())
		run.SetRequestFilter("GenJnlLine", "journal_batch_name", batchName.String())
	}
	genJnlBatch := gtables.NewGenJournalBatch(run)
	if genJnlBatch.Get(map[string]interface{}{"journal_template_name": templateName, "name": batchName}) &&
		genJnlBatch.Bal_account_type == gtables.GenJournalBatch_Bal_account_type.BankAccount && genJnlBatch.Bal_account_no != "" {
		bankAcc := gtables.NewBankAccount(run)
		if bankAcc.Get(genJnlBatch.Bal_account_no) {
			run.SetOption("bank_account", bankAcc.No.String())
			run.SetOption("last_check_no", bankAcc.Last_check_no.String())
		}
	}
	return nil
}

// onPreReport: the last check no. must be given and contain a number
func (r *checkReport) onPreReport(run *Run) error {
	r.language = fcodeunits.CurrentLanguage()
	r.checkMgt = checkmgt.New(run)
	r.reprintChecks = run.OptionBool("reprint_checks")
	r.testPrint = run.OptionBool("test_print")
	r.oneCheckPrVendor = run.OptionBool("one_check_per_vendor")
	r.preprintedStub = run.OptionBool("preprinted_stub")
	r.bankAcc2 = gtables.NewBankAccount(run)
	err := r.bankAcc2.GetOrError(types.Code(run.Option("bank_account")))
	if err != nil {
		return err
	}
	r.useCheckNo = types.Code(run.Option("last_check_no"))
	if r.useCheckNo == "" {
		r.useCheckNo = r.bankAcc2.Last_check_no
	}
	if r.useCheckNo == "" {
		return system.Error(run.Text("LAST_CHECK_NO_BLANK"))
	}
	if system.IncStr(r.useCheckNo.String()) == "" {
		return system.Error(run.Text("LAST_CHECK_NO_DIGIT"))
	}
	if run.Preview() {
		return system.Error(run.Text("PREVIEW_NOT_ALLOWED"))
	}
	setReportHeadValues(run)
	return nil
}

// genJnlLineOnPreDataItem (VoidGenJnlLine and GenJnlLine OnPreDataItem): Reprint Checks voids
// the journal's printed checks first; then the computer check lines not printed yet (a test
// print: one sample); fixed assets cannot be paid by check
func (r *checkReport) genJnlLineOnPreDataItem(genJnlLine *gtables.GenJournalLine, run *Run) error {
	if !r.testPrint && r.reprintChecks {
		if genJnlLine.GetFilter("line_no") != "" || genJnlLine.GetFilter("document_no") != "" {
			return system.Error(run.Text("NO_LINE_DOC_FILTERS"))
		}
		voidGenJnlLine := gtables.NewGenJournalLine(run)
		voidGenJnlLine.CopyFilters(genJnlLine)
		voidGenJnlLine.SetRange("bank_payment_type", gtables.GenJournalLine_Bank_payment_type.ComputerCheck)
		voidGenJnlLine.SetRange("check_printed", true)
		var printed []*gtables.GenJournalLine
		for found := voidGenJnlLine.FindSetBuffered(); found; found = voidGenJnlLine.Next() {
			line := gtables.NewGenJournalLine(run)
			line.Copy(voidGenJnlLine)
			printed = append(printed, line)
		}
		for _, line := range printed {
			current := gtables.NewGenJournalLine(run)
			if !current.Get(map[string]interface{}{"journal_template_name": line.Journal_template_name, "journal_batch_name": line.Journal_batch_name, "line_no": line.Line_no}) || !current.Check_printed {
				continue // voided with another line of its check
			}
			err := r.checkMgt.VoidCheck(current)
			if err != nil {
				return err
			}
		}
	}

	companyInfo := gtables.GetCompanyInformation(run)
	companyAddr := formataddress.AddrArray{}
	if !r.testPrint {
		var err error
		companyAddr, err = formataddress.Company(companyInfo)
		if err != nil {
			return err
		}
		err = r.bankAcc2.TestField("blocked", false)
		if err != nil {
			return err
		}
		genJnlLine.SetRange("bank_payment_type", gtables.GenJournalLine_Bank_payment_type.ComputerCheck)
		genJnlLine.SetRange("check_printed", false)
	} else {
		for i := 0; i < 5; i++ {
			companyAddr[i] = "XXXXXXXXXXXXXXXX"
		}
	}
	run.SetValue("company_addr", companyAddr[:])
	r.checksPrinted = 0
	faLine := gtables.NewGenJournalLine(run)
	faLine.CopyFilters(genJnlLine)
	faLine.SetRange("account_type", gtables.GenJournalLine_Account_type.FixedAsset)
	if faLine.FindFirst() {
		return faLine.FieldError("account_type", "")
	}
	return nil
}

// genJnlLineOnAfterGetRecord: a check line balanced on the bank account — its payee (the line's
// account), how its documents are found and the payee's address
func (r *checkReport) genJnlLineOnAfterGetRecord(genJnlLine *gtables.GenJournalLine, run *Run) error {
	r.genJnlLine = genJnlLine
	if r.testPrint {
		if r.checksPrinted > 0 {
			return Break()
		}
		r.balancingType = gtables.GenJournalLine_Account_type.Vendor
		r.balancingNo = "XXXXXXXXXX"
		r.checkToAddr = formataddress.AddrArray{}
		for i := 0; i < 5; i++ {
			r.checkToAddr[i] = "XXXXXXXXXXXXXXXX"
		}
		run.SetValue("check_date_text", "XX.XXXXXXXXXX.XXXX")
		run.SetValue("check_to_addr", r.checkToAddr[:])
		run.SetValue("check_to_addr_1", r.checkToAddr[0])
		return nil
	}

	// a line printed meanwhile with another line of its vendor's check is left out
	current := gtables.NewGenJournalLine(run)
	if !current.Get(map[string]interface{}{"journal_template_name": genJnlLine.Journal_template_name, "journal_batch_name": genJnlLine.Journal_batch_name, "line_no": genJnlLine.Line_no}) ||
		current.Check_printed || current.Bank_payment_type != gtables.GenJournalLine_Bank_payment_type.ComputerCheck {
		return Skip()
	}
	if genJnlLine.Amount.IsZero() {
		return Skip()
	}
	err := genJnlLine.TestField("bal_account_type", gtables.GenJournalLine_Bal_account_type.BankAccount)
	if err != nil {
		return err
	}
	if genJnlLine.Bal_account_no != r.bankAcc2.No {
		return Skip()
	}
	if genJnlLine.Account_no == "" {
		return genJnlLine.FieldError("account_no", run.Text("MUST_BE_ENTERED"))
	}
	r.balancingType = genJnlLine.Account_type
	r.balancingNo = genJnlLine.Account_no
	r.remainingAmount = genJnlLine.Amount
	r.moreLines = nil
	switch {
	case r.oneCheckPrVendor:
		r.applyMethod = applyMethodMoreLinesOneEntry
		genJnlLine2 := gtables.NewGenJournalLine(run)
		genJnlLine2.SetRange("journal_template_name", genJnlLine.Journal_template_name)
		genJnlLine2.SetRange("journal_batch_name", genJnlLine.Journal_batch_name)
		genJnlLine2.SetRange("posting_date", genJnlLine.Posting_date)
		genJnlLine2.SetRange("document_no", genJnlLine.Document_no)
		genJnlLine2.SetRange("account_type", genJnlLine.Account_type)
		genJnlLine2.SetRange("account_no", genJnlLine.Account_no)
		genJnlLine2.SetRange("bal_account_type", genJnlLine.Bal_account_type)
		genJnlLine2.SetRange("bal_account_no", genJnlLine.Bal_account_no)
		genJnlLine2.SetRange("bank_payment_type", genJnlLine.Bank_payment_type)
		for found := genJnlLine2.FindSetBuffered(); found; found = genJnlLine2.Next() {
			line := gtables.NewGenJournalLine(run)
			line.Copy(genJnlLine2)
			r.moreLines = append(r.moreLines, line)
		}
		r.moreLinesIndex = 0
		r.remainingAmount = types.ZeroDecimal()
	case genJnlLine.Applies_to_doc_no != "":
		r.applyMethod = applyMethodOneLineOneEntry
	case genJnlLine.Applies_to_id != "":
		r.applyMethod = applyMethodOneLineID
	default:
		r.applyMethod = applyMethodPayment
	}

	r.checkToAddr = formataddress.AddrArray{}
	switch r.balancingType {
	case gtables.GenJournalLine_Account_type.GLAccount:
		r.checkToAddr[0] = genJnlLine.Description.String()
	case gtables.GenJournalLine_Account_type.Customer:
		cust := gtables.NewCustomer(run)
		err = cust.GetOrError(r.balancingNo)
		if err == nil {
			err = cust.TestField("blocked", false)
		}
		if err != nil {
			return err
		}
		cust.Contact = ""
		r.checkToAddr, err = formataddress.Customer(cust)
		if err != nil {
			return err
		}
		if r.bankAcc2.Currency_code != genJnlLine.Currency_code {
			return system.Error(run.Text("SAME_CURRENCY"))
		}
	case gtables.GenJournalLine_Account_type.Vendor:
		vend := gtables.NewVendor(run)
		err = vend.GetOrError(r.balancingNo)
		if err == nil {
			err = vend.TestField("blocked", false)
		}
		if err != nil {
			return err
		}
		vend.Contact = ""
		r.checkToAddr, err = formataddress.Vendor(vend)
		if err != nil {
			return err
		}
		if r.bankAcc2.Currency_code != genJnlLine.Currency_code {
			return system.Error(run.Text("SAME_CURRENCY"))
		}
	case gtables.GenJournalLine_Account_type.BankAccount:
		bankAcc := gtables.NewBankAccount(run)
		err = bankAcc.GetOrError(r.balancingNo)
		if err == nil {
			err = bankAcc.TestField("blocked", false)
		}
		if err != nil {
			return err
		}
		bankAcc.Contact = ""
		r.checkToAddr, err = formataddress.BankAcc(bankAcc)
		if err != nil {
			return err
		}
		if r.bankAcc2.Currency_code != bankAcc.Currency_code {
			return system.Error(run.Text("BOTH_BANKS_SAME_CURRENCY"))
		}
	}
	run.SetValue("check_date_text", genJnlLine.Posting_date)
	run.SetValue("check_to_addr", r.checkToAddr[:])
	run.SetValue("check_to_addr_1", r.checkToAddr[0])
	return nil
}

func (r *checkReport) checkPagesOnPreDataItem(rec *Integer, run *Run) error {
	r.firstPage = true
	r.foundLast = false
	r.totalLineAmount = types.ZeroDecimal()
	r.totalLineDiscount = types.ZeroDecimal()
	rec.SetRange("number", 1, 1000000)
	return nil
}

// checkPagesOnAfterGetRecord: each page a check with the next check no.
func (r *checkReport) checkPagesOnAfterGetRecord(rec *Integer, run *Run) error {
	if r.foundLast {
		return Break()
	}
	r.useCheckNo = types.Code(system.IncStr(r.useCheckNo.String()))
	checkNoText := r.useCheckNo.String()
	if r.testPrint {
		checkNoText = "XXXX"
	}
	run.SetValue("check_no_text", checkNoText)
	run.SetValue("total_text", "")
	run.SetValue("transport_amount", types.ZeroDecimal())
	run.SetValue("transport_discount", types.ZeroDecimal())
	run.SetValue("not_first_page", !r.firstPage)
	return nil
}

// checkPagesOnPostDataItem: the journal lines of the check get its no. and Check Printed (one
// check for several lines: a bank line of the check's total); the bank account's Last Check No.
func (r *checkReport) checkPagesOnPostDataItem(rec *Integer, run *Run) error {
	if !r.testPrint {
		genJnlLine := r.genJnlLine
		if r.useCheckNo != genJnlLine.Document_no {
			genJnlLine3 := gtables.NewGenJournalLine(run)
			genJnlLine3.SetRange("journal_template_name", genJnlLine.Journal_template_name)
			genJnlLine3.SetRange("journal_batch_name", genJnlLine.Journal_batch_name)
			genJnlLine3.SetRange("posting_date", genJnlLine.Posting_date)
			genJnlLine3.SetRange("document_no", r.useCheckNo)
			if genJnlLine3.FindFirst() {
				return genJnlLine3.FieldError("document_no", run.Text("ALREADY_EXISTS", r.useCheckNo))
			}
		}
		if r.applyMethod != applyMethodMoreLinesOneEntry {
			genJnlLine3 := gtables.NewGenJournalLine(run)
			err := genJnlLine3.GetOrError(map[string]interface{}{"journal_template_name": genJnlLine.Journal_template_name, "journal_batch_name": genJnlLine.Journal_batch_name, "line_no": genJnlLine.Line_no})
			if err == nil {
				err = genJnlLine3.TestField("posting_no_series", types.Code(""))
			}
			if err != nil {
				return err
			}
			genJnlLine3.Document_no = r.useCheckNo
			genJnlLine3.Check_printed = true
			err = genJnlLine3.ModifyOrError(false)
			if err != nil {
				return err
			}
		} else {
			totalLineAmountLCY := types.ZeroDecimal()
			highestLineNo := 0
			for _, line := range r.moreLines {
				if line.Line_no > highestLineNo {
					highestLineNo = line.Line_no
				}
				genJnlLine3 := gtables.NewGenJournalLine(run)
				err := genJnlLine3.GetOrError(map[string]interface{}{"journal_template_name": line.Journal_template_name, "journal_batch_name": line.Journal_batch_name, "line_no": line.Line_no})
				if err == nil {
					err = genJnlLine3.TestField("posting_no_series", types.Code(""))
				}
				if err != nil {
					return err
				}
				genJnlLine3.Bal_account_no = ""
				genJnlLine3.Bank_payment_type = gtables.GenJournalLine_Bank_payment_type.Blank
				genJnlLine3.Document_no = r.useCheckNo
				genJnlLine3.Check_printed = true
				err = genJnlLine3.Validate("amount", genJnlLine3.Amount)
				if err != nil {
					return err
				}
				totalLineAmountLCY = totalLineAmountLCY.Add(genJnlLine3.Amount_lcy)
				err = genJnlLine3.ModifyOrError(false)
				if err != nil {
					return err
				}
			}
			bankLine := gtables.NewGenJournalLine(run)
			bankLine.Journal_template_name = genJnlLine.Journal_template_name
			bankLine.Journal_batch_name = genJnlLine.Journal_batch_name
			bankLine.Line_no = r.lineNoAfter(run, genJnlLine, highestLineNo)
			err := bankLine.Validate("posting_date", genJnlLine.Posting_date)
			if err != nil {
				return err
			}
			bankLine.Document_type = genJnlLine.Document_type
			bankLine.Document_no = r.useCheckNo
			bankLine.Account_type = gtables.GenJournalLine_Account_type.BankAccount
			err = bankLine.Validate("account_no", r.bankAcc2.No)
			if err != nil {
				return err
			}
			if r.balancingType != gtables.GenJournalLine_Account_type.GLAccount {
				bankLine.Description = types.Text(run.Text("CHECK_FOR",
					optionCaption(gtables.GenJournalLineTableName, "account_type", r.balancingType.String(), r.language), r.balancingNo))
			}
			err = bankLine.Validate("amount", r.totalLineAmount.Neg())
			if err != nil {
				return err
			}
			if !r.totalLineAmount.Equal(totalLineAmountLCY) {
				err = bankLine.Validate("amount_lcy", totalLineAmountLCY.Neg())
				if err != nil {
					return err
				}
			}
			bankLine.Bank_payment_type = gtables.GenJournalLine_Bank_payment_type.ComputerCheck
			bankLine.Check_printed = true
			bankLine.Source_code = genJnlLine.Source_code
			bankLine.Reason_code = genJnlLine.Reason_code
			err = bankLine.InsertOrError(false)
			if err != nil {
				return err
			}
		}
	}
	r.bankAcc2.Last_check_no = r.useCheckNo
	return r.bankAcc2.ModifyOrError(false)
}

// lineNoAfter is a free line no. after the check's highest line (halfway to the next line, or
// 10000 after the last)
func (r *checkReport) lineNoAfter(run *Run, genJnlLine *gtables.GenJournalLine, highestLineNo int) int {
	next := gtables.NewGenJournalLine(run)
	next.SetRange("journal_template_name", genJnlLine.Journal_template_name)
	next.SetRange("journal_batch_name", genJnlLine.Journal_batch_name)
	next.SetFilter("line_no", ">"+system.Format(highestLineNo))
	if !next.FindFirst() {
		return highestLineNo + 10000
	}
	return (next.Line_no + highestLineNo) / 2
}

// printSettledLoopOnPreDataItem: on the first page of a check, whether it has documents to list
func (r *checkReport) printSettledLoopOnPreDataItem(rec *Integer, run *Run) error {
	rec.SetRange("number", 1, stubLinesPerCheck)
	if r.testPrint {
		r.foundLast = false
		return nil
	}
	if !r.firstPage {
		r.foundLast = false
		return nil
	}
	r.foundLast = true
	switch r.applyMethod {
	case applyMethodOneLineOneEntry, applyMethodMoreLinesOneEntry:
		r.foundLast = false
	case applyMethodOneLineID:
		r.ledgEntryIndex = 0
		switch r.balancingType {
		case gtables.GenJournalLine_Account_type.Customer:
			r.custLedgEntries = r.custEntriesOfID(run, true)
			r.foundNegative = false
			if len(r.custLedgEntries) == 0 {
				r.custLedgEntries = r.custEntriesOfID(run, false)
				r.foundNegative = true
			}
			r.foundLast = len(r.custLedgEntries) == 0
		case gtables.GenJournalLine_Account_type.Vendor:
			r.vendLedgEntries = r.vendEntriesOfID(run, true)
			r.foundNegative = false
			if len(r.vendLedgEntries) == 0 {
				r.vendLedgEntries = r.vendEntriesOfID(run, false)
				r.foundNegative = true
			}
			r.foundLast = len(r.vendLedgEntries) == 0
		}
	}
	return nil
}

func (r *checkReport) custEntriesOfID(run *Run, positive bool) []*gtables.CustomerLedgerEntry {
	custLedgEntry := gtables.NewCustomerLedgerEntry(run)
	custLedgEntry.SetRange("customer_no", r.balancingNo)
	custLedgEntry.SetRange("open", true)
	custLedgEntry.SetRange("positive", positive)
	custLedgEntry.SetRange("applies_to_id", r.genJnlLine.Applies_to_id)
	var entries []*gtables.CustomerLedgerEntry
	for found := custLedgEntry.FindSetBuffered(); found; found = custLedgEntry.Next() {
		entry := gtables.NewCustomerLedgerEntry(run)
		entry.Copy(custLedgEntry)
		entries = append(entries, entry)
	}
	return entries
}

func (r *checkReport) vendEntriesOfID(run *Run, positive bool) []*gtables.VendorLedgerEntry {
	vendLedgEntry := gtables.NewVendorLedgerEntry(run)
	vendLedgEntry.SetRange("vendor_no", r.balancingNo)
	vendLedgEntry.SetRange("open", true)
	vendLedgEntry.SetRange("positive", positive)
	vendLedgEntry.SetRange("applies_to_id", r.genJnlLine.Applies_to_id)
	var entries []*gtables.VendorLedgerEntry
	for found := vendLedgEntry.FindSetBuffered(); found; found = vendLedgEntry.Next() {
		entry := gtables.NewVendorLedgerEntry(run)
		entry.Copy(vendLedgEntry)
		entries = append(entries, entry)
	}
	return entries
}

// printSettledLoopOnAfterGetRecord: the stub's next document line — the entry the line is
// applied to, the next entry of its Applies-to ID, the vendor's next line, or the rest of the
// payment
func (r *checkReport) printSettledLoopOnAfterGetRecord(rec *Integer, run *Run) error {
	docType, docNo := "", ""
	docDate := types.Date{}
	if r.testPrint {
		if r.foundLast {
			return Break()
		}
		r.foundLast = true
		r.setStubLine(run, "XXX", "XXXXXXXXXX", types.Date{}, types.ZeroDecimal(), types.ZeroDecimal())
		return nil
	}
	if r.foundLast {
		if r.remainingAmount.IsZero() {
			return Break()
		}
		r.lineAmount = r.remainingAmount
		r.lineDiscount = types.ZeroDecimal()
		r.remainingAmount = types.ZeroDecimal()
		r.totalLineAmount = r.totalLineAmount.Add(r.lineAmount)
		r.setStubLine(run, run.Text("PAYMENT"), "", types.Date{}, r.lineAmount, r.lineDiscount)
		return nil
	}
	switch r.applyMethod {
	case applyMethodOneLineOneEntry:
		switch r.balancingType {
		case gtables.GenJournalLine_Account_type.Customer:
			custLedgEntry := gtables.NewCustomerLedgerEntry(run)
			custLedgEntry.SetRange("document_type", int(r.genJnlLine.Applies_to_doc_type))
			custLedgEntry.SetRange("document_no", r.genJnlLine.Applies_to_doc_no)
			custLedgEntry.SetRange("customer_no", r.balancingNo)
			if !custLedgEntry.FindFirst() {
				return r.genJnlLine.FieldError("applies_to_doc_no", "")
			}
			var err error
			docType, docNo, docDate, err = r.custUpdateAmounts(run, custLedgEntry, r.remainingAmount, r.genJnlLine.Line_no)
			if err != nil {
				return err
			}
		case gtables.GenJournalLine_Account_type.Vendor:
			vendLedgEntry := gtables.NewVendorLedgerEntry(run)
			vendLedgEntry.SetRange("document_type", int(r.genJnlLine.Applies_to_doc_type))
			vendLedgEntry.SetRange("document_no", r.genJnlLine.Applies_to_doc_no)
			vendLedgEntry.SetRange("vendor_no", r.balancingNo)
			if !vendLedgEntry.FindFirst() {
				return r.genJnlLine.FieldError("applies_to_doc_no", "")
			}
			var err error
			docType, docNo, docDate, err = r.vendUpdateAmounts(run, vendLedgEntry, r.remainingAmount, r.genJnlLine.Line_no)
			if err != nil {
				return err
			}
		}
		r.remainingAmount = r.remainingAmount.Sub(r.lineAmount)
		r.foundLast = true
	case applyMethodOneLineID:
		switch r.balancingType {
		case gtables.GenJournalLine_Account_type.Customer:
			var err error
			docType, docNo, docDate, err = r.custUpdateAmounts(run, r.custLedgEntries[r.ledgEntryIndex], r.remainingAmount, 0)
			if err != nil {
				return err
			}
			r.ledgEntryIndex++
			r.foundLast = r.ledgEntryIndex >= len(r.custLedgEntries) || !r.remainingAmount.IsPositive()
			if r.foundLast && !r.foundNegative {
				r.custLedgEntries = r.custEntriesOfID(run, false)
				r.ledgEntryIndex = 0
				r.foundLast = len(r.custLedgEntries) == 0
				r.foundNegative = true
			}
		case gtables.GenJournalLine_Account_type.Vendor:
			var err error
			docType, docNo, docDate, err = r.vendUpdateAmounts(run, r.vendLedgEntries[r.ledgEntryIndex], r.remainingAmount, 0)
			if err != nil {
				return err
			}
			r.ledgEntryIndex++
			r.foundLast = r.ledgEntryIndex >= len(r.vendLedgEntries) || !r.remainingAmount.IsPositive()
			if r.foundLast && !r.foundNegative {
				r.vendLedgEntries = r.vendEntriesOfID(run, false)
				r.ledgEntryIndex = 0
				r.foundLast = len(r.vendLedgEntries) == 0
				r.foundNegative = true
			}
		}
		r.remainingAmount = r.remainingAmount.Sub(r.lineAmount)
	case applyMethodMoreLinesOneEntry:
		genJnlLine2 := r.moreLines[r.moreLinesIndex]
		currentLineAmount := genJnlLine2.Amount
		if genJnlLine2.Applies_to_id != "" {
			return system.Error(run.Text("NO_ONE_CHECK_WITH_ID"))
		}
		err := genJnlLine2.TestField("check_printed", false)
		if err == nil {
			err = genJnlLine2.TestField("bank_payment_type", gtables.GenJournalLine_Bank_payment_type.ComputerCheck)
		}
		if err != nil {
			return err
		}
		if genJnlLine2.Applies_to_doc_no == "" {
			docType = run.Text("PAYMENT")
			r.lineAmount = currentLineAmount
			r.lineDiscount = types.ZeroDecimal()
		} else {
			switch r.balancingType {
			case gtables.GenJournalLine_Account_type.Customer:
				custLedgEntry := gtables.NewCustomerLedgerEntry(run)
				custLedgEntry.SetRange("document_type", int(genJnlLine2.Applies_to_doc_type))
				custLedgEntry.SetRange("document_no", genJnlLine2.Applies_to_doc_no)
				custLedgEntry.SetRange("customer_no", r.balancingNo)
				if !custLedgEntry.FindFirst() {
					return genJnlLine2.FieldError("applies_to_doc_no", "")
				}
				docType, docNo, docDate, err = r.custUpdateAmounts(run, custLedgEntry, currentLineAmount, genJnlLine2.Line_no)
			case gtables.GenJournalLine_Account_type.Vendor:
				vendLedgEntry := gtables.NewVendorLedgerEntry(run)
				vendLedgEntry.SetRange("document_type", int(genJnlLine2.Applies_to_doc_type))
				vendLedgEntry.SetRange("document_no", genJnlLine2.Applies_to_doc_no)
				vendLedgEntry.SetRange("vendor_no", r.balancingNo)
				if !vendLedgEntry.FindFirst() {
					return genJnlLine2.FieldError("applies_to_doc_no", "")
				}
				docType, docNo, docDate, err = r.vendUpdateAmounts(run, vendLedgEntry, currentLineAmount, genJnlLine2.Line_no)
			default:
				docType = optionCaption(gtables.GenJournalLineTableName, "document_type", genJnlLine2.Document_type.String(), r.language)
				docNo = genJnlLine2.Document_no.String()
				r.lineAmount = currentLineAmount
				r.lineDiscount = types.ZeroDecimal()
			}
			if err != nil {
				return err
			}
		}
		r.moreLinesIndex++
		r.foundLast = r.moreLinesIndex >= len(r.moreLines)
	default: // Payment: the whole amount, no documents
		r.lineAmount = r.remainingAmount
		r.lineDiscount = types.ZeroDecimal()
		r.remainingAmount = types.ZeroDecimal()
		docType = run.Text("PAYMENT")
		r.foundLast = true
	}
	r.totalLineAmount = r.totalLineAmount.Add(r.lineAmount)
	r.totalLineDiscount = r.totalLineDiscount.Add(r.lineDiscount)
	r.setStubLine(run, docType, docNo, docDate, r.lineAmount, r.lineDiscount)
	return nil
}

// setStubLine sets the values of a stub line and the totals carried to the next page
func (r *checkReport) setStubLine(run *Run, docType, docNo string, docDate types.Date, lineAmount, lineDiscount types.Decimal) {
	run.SetValue("doc_type", docType)
	run.SetValue("doc_no", docNo)
	if docDate.IsZero() {
		run.SetValue("doc_date", "")
	} else {
		run.SetValue("doc_date", docDate)
	}
	run.SetValue("line_amount", lineAmount.Add(lineDiscount))
	run.SetValue("line_discount", lineDiscount)
	run.SetValue("net_amount", lineAmount)
}

// alreadyApplied: another journal line is applied to the document (CustUpdateAmounts /
// VendUpdateAmounts with one entry per line)
func (r *checkReport) alreadyApplied(run *Run, accountType gtables.GenJournalLineAccount_type, accountNo types.Code, docType int, docNo types.Code, lineNo int) bool {
	genJnlLine3 := gtables.NewGenJournalLine(run)
	genJnlLine3.SetRange("account_type", accountType)
	genJnlLine3.SetRange("account_no", accountNo)
	genJnlLine3.SetRange("applies_to_doc_type", docType)
	genJnlLine3.SetRange("applies_to_doc_no", docNo)
	genJnlLine3.SetFilter("line_no", "<>"+system.Format(lineNo))
	return genJnlLine3.FindFirst()
}

// custUpdateAmounts (CustUpdateAmounts): the stub line of a customer entry — what is paid of it
// (less the payment discount of an invoice paid in time) up to the amount left
func (r *checkReport) custUpdateAmounts(run *Run, custLedgEntry2 *gtables.CustomerLedgerEntry, remainingAmount2 types.Decimal, lineNo int) (string, string, types.Date, error) {
	if lineNo != 0 && r.alreadyApplied(run, gtables.GenJournalLine_Account_type.Customer, custLedgEntry2.Customer_no, int(custLedgEntry2.Document_type), custLedgEntry2.Document_no, lineNo) {
		return "", "", types.Date{}, system.Error(run.Text("ALREADY_APPLIED_CUSTOMER",
			optionCaption(gtables.CustomerLedgerEntryTableName, "document_type", custLedgEntry2.Document_type.String(), r.language),
			custLedgEntry2.Document_no, custLedgEntry2.Customer_no))
	}
	docType := optionCaption(gtables.CustomerLedgerEntryTableName, "document_type", custLedgEntry2.Document_type.String(), r.language)
	if custLedgEntry2.Document_type == gtables.CustomerLedgerEntry_Document_type.Invoice &&
		!r.genJnlLine.Posting_date.After(custLedgEntry2.Pmt_discount_date) &&
		!custLedgEntry2.Remaining_amount.Sub(custLedgEntry2.Pmt_disc_possible).GreaterThan(remainingAmount2) {
		r.lineAmount = custLedgEntry2.Remaining_amount.Sub(custLedgEntry2.Pmt_disc_possible)
		r.lineDiscount = custLedgEntry2.Pmt_disc_possible
	} else {
		if !custLedgEntry2.Remaining_amount.GreaterThan(remainingAmount2) {
			r.lineAmount = custLedgEntry2.Remaining_amount
		} else {
			r.lineAmount = remainingAmount2
		}
		r.lineDiscount = types.ZeroDecimal()
	}
	return docType, custLedgEntry2.Document_no.String(), custLedgEntry2.Document_date, nil
}

// vendUpdateAmounts (VendUpdateAmounts): the stub line of a vendor entry — its external document
// no. and what is paid of it (less the payment discount of an invoice paid in time)
func (r *checkReport) vendUpdateAmounts(run *Run, vendLedgEntry2 *gtables.VendorLedgerEntry, remainingAmount2 types.Decimal, lineNo int) (string, string, types.Date, error) {
	if lineNo != 0 && r.alreadyApplied(run, gtables.GenJournalLine_Account_type.Vendor, vendLedgEntry2.Vendor_no, int(vendLedgEntry2.Document_type), vendLedgEntry2.Document_no, lineNo) {
		return "", "", types.Date{}, system.Error(run.Text("ALREADY_APPLIED_VENDOR",
			optionCaption(gtables.VendorLedgerEntryTableName, "document_type", vendLedgEntry2.Document_type.String(), r.language),
			vendLedgEntry2.Document_no, vendLedgEntry2.Vendor_no))
	}
	docType := optionCaption(gtables.VendorLedgerEntryTableName, "document_type", vendLedgEntry2.Document_type.String(), r.language)
	if vendLedgEntry2.Document_type == gtables.VendorLedgerEntry_Document_type.Invoice &&
		!r.genJnlLine.Posting_date.After(vendLedgEntry2.Pmt_discount_date) &&
		!vendLedgEntry2.Remaining_amount.Sub(vendLedgEntry2.Pmt_disc_possible).Neg().GreaterThan(remainingAmount2) {
		r.lineAmount = vendLedgEntry2.Remaining_amount.Sub(vendLedgEntry2.Pmt_disc_possible).Neg()
		r.lineDiscount = vendLedgEntry2.Pmt_disc_possible.Neg()
	} else {
		if !vendLedgEntry2.Remaining_amount.Neg().GreaterThan(remainingAmount2) {
			r.lineAmount = vendLedgEntry2.Remaining_amount.Neg()
		} else {
			r.lineAmount = remainingAmount2
		}
		r.lineDiscount = types.ZeroDecimal()
	}
	return docType, vendLedgEntry2.External_document_no.String(), vendLedgEntry2.Document_date, nil
}

// printCheckOnAfterGetRecord: the check — its check ledger entry (Printed with the check's total,
// or Voided when its documents go on to the next check), the amount in figures and words
func (r *checkReport) printCheckOnAfterGetRecord(rec *Integer, run *Run) error {
	checkLedgEntry := gtables.NewCheckLedgerEntry(run)
	genJnlLine := r.genJnlLine
	checkAmountText := ""
	descriptionLine := [2]string{}
	voidText := ""
	if !r.testPrint {
		checkLedgEntry.Bank_account_no = r.bankAcc2.No
		checkLedgEntry.Posting_date = genJnlLine.Posting_date
		checkLedgEntry.Document_type = gtables.CheckLedgerEntryDocument_type(genJnlLine.Document_type)
		checkLedgEntry.Document_no = r.useCheckNo
		checkLedgEntry.Description = types.Text(r.checkToAddr[0])
		checkLedgEntry.Bank_payment_type = gtables.CheckLedgerEntryBank_payment_type(genJnlLine.Bank_payment_type)
		checkLedgEntry.Bal_account_type = gtables.CheckLedgerEntryBal_account_type(r.balancingType)
		checkLedgEntry.Bal_account_no = r.balancingNo
		if r.foundLast {
			if r.totalLineAmount.IsNegative() {
				return system.Error(run.Text("CHECK_AMOUNT_NEGATIVE", r.useCheckNo, formatDecimal(r.totalLineAmount, "2:2", r.language)))
			}
			checkLedgEntry.Entry_status = gtables.CheckLedgerEntry_Entry_status.Printed
			checkLedgEntry.Amount = r.totalLineAmount
		} else {
			checkLedgEntry.Entry_status = gtables.CheckLedgerEntry_Entry_status.Voided
			checkLedgEntry.Amount = types.ZeroDecimal()
		}
		checkLedgEntry.Check_date = genJnlLine.Posting_date
		checkLedgEntry.Check_no = r.useCheckNo
		err := r.checkMgt.InsertCheck(checkLedgEntry)
		if err != nil {
			return err
		}
		if r.foundLast {
			checkAmountText = formatDecimal(checkLedgEntry.Amount, "2:2", r.language)
			descriptionLine, err = checkmgt.FormatNoText(checkLedgEntry.Amount, checkmgt.CurrencyText(run, r.bankAcc2.Currency_code), r.language)
			if err != nil {
				return err
			}
		} else {
			descriptionLine[0] = run.Text("VOID_LINE")
			descriptionLine[1] = descriptionLine[0]
			voidText = run.Text("NON_NEGOTIABLE")
		}
	} else {
		checkLedgEntry.Bank_account_no = r.bankAcc2.No
		checkLedgEntry.Posting_date = genJnlLine.Posting_date
		checkLedgEntry.Document_no = r.useCheckNo
		checkLedgEntry.Description = types.Text(run.Text("TEST_PRINT"))
		checkLedgEntry.Bank_payment_type = gtables.CheckLedgerEntry_Bank_payment_type.ComputerCheck
		checkLedgEntry.Entry_status = gtables.CheckLedgerEntry_Entry_status.TestPrint
		checkLedgEntry.Check_date = genJnlLine.Posting_date
		checkLedgEntry.Check_no = r.useCheckNo
		err := r.checkMgt.InsertCheck(checkLedgEntry)
		if err != nil {
			return err
		}
		checkAmountText = "XXXX.XX"
		descriptionLine[0] = "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
		descriptionLine[1] = descriptionLine[0]
		voidText = run.Text("NON_NEGOTIABLE")
	}
	run.SetValue("check_amount_text", "***"+checkAmountText)
	run.SetValue("description_line_1", descriptionLine[0])
	run.SetValue("description_line_2", descriptionLine[1])
	run.SetValue("void_text", voidText)
	run.SetValue("total_line_amount", r.totalLineAmount)
	r.checksPrinted++
	r.firstPage = false
	return nil
}
