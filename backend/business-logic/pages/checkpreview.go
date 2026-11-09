package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/checkmgt"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/formataddress"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 404 Check Preview (Payments › Preview Check on the payment journal): the check a payment
// journal line gives — the check amount (the lines of the check; a printed check the lines
// printed with it), the amount in words, the payee's and the company's addresses and whether it
// is printed (form 404 CalcCheck)
const checkPreviewPageID = 404

func init() {
	pages.OnAfterGetRecord(checkPreviewPageID, func(rec *gtables.GenJournalLine, page *pages.Control) {
		language := fcodeunits.CurrentLanguage()
		message := func(key string) string { return i18n.GetInstance().Message(key, language) }

		companyAddr, _ := formataddress.Company(gtables.GetCompanyInformation(rec))
		genJnlLine := gtables.NewGenJournalLine(rec)
		genJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
		genJnlLine.SetRange("journal_batch_name", rec.Journal_batch_name)
		genJnlLine.SetRange("posting_date", rec.Posting_date)
		genJnlLine.SetRange("document_no", rec.Document_no)
		checkStatusText := ""
		if rec.Check_printed {
			if rec.Bal_account_no == "" {
				genJnlLine.SetRange("bank_payment_type", gtables.GenJournalLine_Bank_payment_type.Blank)
			} else {
				genJnlLine.SetRange("bank_payment_type", gtables.GenJournalLine_Bank_payment_type.ComputerCheck)
			}
			genJnlLine.SetRange("check_printed", true)
			checkStatusText = message("CHECK_PREVIEW_PRINTED")
		} else {
			genJnlLine.SetRange("account_type", rec.Account_type)
			genJnlLine.SetRange("account_no", rec.Account_no)
			genJnlLine.SetRange("bal_account_type", rec.Bal_account_type)
			genJnlLine.SetRange("bal_account_no", rec.Bal_account_no)
			genJnlLine.SetRange("bank_payment_type", rec.Bank_payment_type)
			checkStatusText = message("CHECK_PREVIEW_NOT_PRINTED")
		}

		checkAmount := types.ZeroDecimal()
		lastLine := gtables.NewGenJournalLine(rec)
		lastLine.Copy(rec)
		for found := genJnlLine.FindSetBuffered(); found; found = genJnlLine.Next() {
			checkAmount = checkAmount.Add(genJnlLine.Amount)
			lastLine.Copy(genJnlLine)
		}
		if checkAmount.IsNegative() {
			checkAmount = types.ZeroDecimal()
		}

		bankAccNo := rec.Bal_account_no
		if rec.Account_type == gtables.GenJournalLine_Account_type.BankAccount {
			bankAccNo = rec.Account_no
		}
		bankAcc := gtables.NewBankAccount(rec)
		currencyCode := types.Code("")
		if bankAcc.Get(bankAccNo) {
			currencyCode = bankAcc.Currency_code
		}
		numberText, _ := checkmgt.FormatNoText(checkAmount, checkmgt.CurrencyText(rec, currencyCode), language)

		checkToAddr := formataddress.AddrArray{}
		switch lastLine.Account_type {
		case gtables.GenJournalLine_Account_type.GLAccount:
			checkToAddr[0] = lastLine.Description.String()
		case gtables.GenJournalLine_Account_type.Customer:
			cust := gtables.NewCustomer(rec)
			if cust.Get(lastLine.Account_no) {
				cust.Contact = ""
				checkToAddr, _ = formataddress.Customer(cust)
			}
		case gtables.GenJournalLine_Account_type.Vendor:
			vend := gtables.NewVendor(rec)
			if vend.Get(lastLine.Account_no) {
				vend.Contact = ""
				checkToAddr, _ = formataddress.Vendor(vend)
			}
		case gtables.GenJournalLine_Account_type.BankAccount:
			payeeBank := gtables.NewBankAccount(rec)
			if payeeBank.Get(lastLine.Account_no) {
				payeeBank.Contact = ""
				checkToAddr, _ = formataddress.BankAcc(payeeBank)
			}
		}

		page.SetValue("check_status_text", checkStatusText)
		page.SetValue("check_amount", checkAmount)
		page.SetValue("number_text_1", numberText[0])
		page.SetValue("number_text_2", numberText[1])
		for i, field := range []string{"pay_to_1", "pay_to_2", "pay_to_3", "pay_to_4"} {
			page.SetValue(field, checkToAddr[i])
		}
		for i, field := range []string{"company_addr_1", "company_addr_2", "company_addr_3", "company_addr_4"} {
			page.SetValue(field, companyAddr[i])
		}
	})
}
