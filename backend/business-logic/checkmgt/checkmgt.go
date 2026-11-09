// Package checkmgt is codeunit 367 CheckManagement: the check ledger entries of printed checks
// (InsertCheck), voiding a printed check in the payment journal (VoidCheck) — the check's
// journal lines go back to unprinted, the check ledger entry is voided — and the check amount
// in words (FormatNoText, classic report 1401's FormatNoText, shared with page 404 Check
// Preview).
package checkmgt

import (
	"strconv"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// CheckManagement holds the next check ledger entry no. of a run (classic NextCheckEntryNo)
type CheckManagement struct {
	src              ftables.Source
	nextCheckEntryNo int
}

// New returns the codeunit on src
func New(src ftables.Source) *CheckManagement {
	return &CheckManagement{src: src}
}

// InsertCheck inserts a check ledger entry with the next entry no.; a check no. printed,
// posted or financially voided before for the bank account (exported: transmitted, posted,
// financially voided) is refused
func (m *CheckManagement) InsertCheck(checkLedgEntry *gtables.CheckLedgerEntry) error {
	checkLedgEntry2 := gtables.NewCheckLedgerEntry(m.src)
	if m.nextCheckEntryNo == 0 {
		checkLedgEntry2.LockTable()
		if checkLedgEntry2.FindLast() {
			m.nextCheckEntryNo = checkLedgEntry2.Entry_no + 1
		} else {
			m.nextCheckEntryNo = 1
		}
	}
	checkLedgEntry2 = gtables.NewCheckLedgerEntry(m.src)
	checkLedgEntry2.SetRange("bank_account_no", checkLedgEntry.Bank_account_no)
	statuses := []gtables.CheckLedgerEntryEntry_status{
		gtables.CheckLedgerEntry_Entry_status.Printed,
		gtables.CheckLedgerEntry_Entry_status.Posted,
		gtables.CheckLedgerEntry_Entry_status.FinanciallyVoided,
	}
	if checkLedgEntry.Entry_status == gtables.CheckLedgerEntry_Entry_status.Exported {
		statuses[0] = gtables.CheckLedgerEntry_Entry_status.Transmitted
	}
	var filter []string
	for _, status := range statuses {
		filter = append(filter, strconv.Itoa(int(status)))
	}
	checkLedgEntry2.SetFilter("entry_status", strings.Join(filter, "|"))
	checkLedgEntry2.SetRange("check_no", checkLedgEntry.Document_no)
	if checkLedgEntry2.FindFirst() {
		return apperrors.CheckAlreadyExists(checkLedgEntry.Document_no.String())
	}
	checkLedgEntry.Open = !checkLedgEntry.Amount.IsZero()
	checkLedgEntry.User_id = types.Code(system.UserId())
	checkLedgEntry.Entry_no = m.nextCheckEntryNo
	err := checkLedgEntry.InsertOrError(false)
	if err != nil {
		return err
	}
	m.nextCheckEntryNo++
	return nil
}

// VoidCheck voids the printed computer check of a payment journal line: a check printed for
// several lines (its bank line has no balancing account) loses that bank line; the other lines
// of the check get their balancing bank account back, no document no. and Check Printed off (a
// currency rounding line of the check is deleted); the check ledger entry becomes Voided
func (m *CheckManagement) VoidCheck(genJnlLine *gtables.GenJournalLine) error {
	err := genJnlLine.TestField("bank_payment_type", gtables.GenJournalLine_Bank_payment_type.ComputerCheck)
	if err == nil {
		err = genJnlLine.TestField("check_printed", true)
	}
	if err == nil {
		err = genJnlLine.TestField("document_no")
	}
	if err != nil {
		return err
	}

	if genJnlLine.Bal_account_no == "" {
		genJnlLine.Check_printed = false
		err = genJnlLine.DeleteOrError(true)
		if err != nil {
			return err
		}
	}

	checkAmountLCY := genJnlLine.Amount_lcy
	currency := gtables.NewCurrency(m.src)
	if genJnlLine.Currency_code != "" {
		err = currency.GetOrError(genJnlLine.Currency_code)
		if err != nil {
			return err
		}
	}
	genJnlLine2 := gtables.NewGenJournalLine(m.src)
	genJnlLine2.SetRange("journal_template_name", genJnlLine.Journal_template_name)
	genJnlLine2.SetRange("journal_batch_name", genJnlLine.Journal_batch_name)
	genJnlLine2.SetRange("posting_date", genJnlLine.Posting_date)
	genJnlLine2.SetRange("document_no", genJnlLine.Document_no)
	for found := genJnlLine2.FindSetBuffered(); found; found = genJnlLine2.Next() {
		roundingLine := genJnlLine2.Line_no > genJnlLine.Line_no &&
			checkAmountLCY.Equal(genJnlLine2.Amount_lcy.Neg()) &&
			genJnlLine2.Currency_code == "" && genJnlLine.Currency_code != "" &&
			genJnlLine2.Account_type == gtables.GenJournalLine_Account_type.GLAccount &&
			(genJnlLine2.Account_no == currency.Conv_lcy_rndg_debit_acc || genJnlLine2.Account_no == currency.Conv_lcy_rndg_credit_acc) &&
			genJnlLine2.Bal_account_no == "" && !genJnlLine2.Check_printed
		if roundingLine {
			err = genJnlLine2.DeleteOrError(false)
			if err != nil {
				return err
			}
		} else {
			if genJnlLine.Bal_account_no == "" {
				if genJnlLine2.Account_no == "" {
					genJnlLine2.Account_type = gtables.GenJournalLine_Account_type.BankAccount
					genJnlLine2.Account_no = genJnlLine.Account_no
					checkAmountLCY = checkAmountLCY.Sub(genJnlLine2.Amount_lcy)
				} else {
					genJnlLine2.Bal_account_type = gtables.GenJournalLine_Bal_account_type.BankAccount
					genJnlLine2.Bal_account_no = genJnlLine.Account_no
					checkAmountLCY = checkAmountLCY.Add(genJnlLine2.Amount_lcy)
				}
			}
			genJnlLine2.Document_no = ""
			genJnlLine2.Check_printed = false
			tables.UpdateGenJnlLineSource(genJnlLine2)
			err = genJnlLine2.ModifyOrError(false)
			if err != nil {
				return err
			}
		}
	}

	checkLedgEntry2 := gtables.NewCheckLedgerEntry(m.src)
	if !genJnlLine.Amount.IsPositive() {
		checkLedgEntry2.SetRange("bank_account_no", genJnlLine.Account_no)
	} else {
		checkLedgEntry2.SetRange("bank_account_no", genJnlLine.Bal_account_no)
	}
	checkLedgEntry2.SetRange("entry_status", gtables.CheckLedgerEntry_Entry_status.Printed)
	checkLedgEntry2.SetRange("check_no", genJnlLine.Document_no)
	if !checkLedgEntry2.FindFirst() {
		return apperrors.TableRecordNotFound(gtables.CheckLedgerEntryTableName, genJnlLine.Document_no.String())
	}
	checkLedgEntry2.Original_entry_status = gtables.CheckLedgerEntryOriginal_entry_status(checkLedgEntry2.Entry_status)
	checkLedgEntry2.Entry_status = gtables.CheckLedgerEntry_Entry_status.Voided
	checkLedgEntry2.Open = false
	return checkLedgEntry2.ModifyOrError(false)
}

// FormatNoText writes an amount in words on two lines of at most 80 characters, as a check
// prints it ("**** ONE THOUSAND TWO HUNDRED THIRTY-FOUR AND 56/100 NOK"): the words of the
// language, the currency code (blank: the local currency's)
func FormatNoText(no types.Decimal, currencyCode string, language string) ([2]string, error) {
	message := func(key string) string { return i18n.GetInstance().Message(key, language) }
	onesText := strings.Split(message("CHECK_WORDS_ONES"), ",")
	tensText := strings.Split(message("CHECK_WORDS_TENS"), ",")
	exponentText := strings.Split(message("CHECK_WORDS_EXPONENTS"), ",")
	var noText [2]string
	noTextIndex := 0
	noText[0] = "****"
	addToNoText := func(addText string) error {
		if addText == "" {
			return nil
		}
		for system.StrLen(strings.TrimLeft(noText[noTextIndex]+" "+addText, " ")) > 80 {
			noTextIndex++
			if noTextIndex >= len(noText) {
				return apperrors.CheckAmountTooLong(addText)
			}
		}
		noText[noTextIndex] = strings.TrimLeft(noText[noTextIndex]+" "+addText, " ")
		return nil
	}
	word := func(words []string, i int) string {
		if i < 1 || i > len(words) {
			return ""
		}
		return strings.TrimSpace(words[i-1])
	}

	cents, _ := strconv.ParseInt(no.Mul(types.NewDecimalFromInt(100)).Round(0).StringFixed(0), 10, 64)
	whole := cents / 100
	if whole < 1 {
		err := addToNoText(message("CHECK_WORDS_ZERO"))
		if err != nil {
			return noText, err
		}
	} else {
		power := int64(1000000000)
		for exponent := 4; exponent >= 1; exponent-- {
			group := whole / power
			hundreds := group / 100
			tens := (group % 100) / 10
			ones := group % 10
			printExponent := false
			if hundreds > 0 {
				printExponent = true
				err := addToNoText(word(onesText, int(hundreds)))
				if err == nil {
					err = addToNoText(message("CHECK_WORDS_HUNDRED"))
				}
				if err != nil {
					return noText, err
				}
			}
			if tens >= 2 {
				printExponent = true
				err := addToNoText(word(tensText, int(tens)))
				if err == nil && ones > 0 {
					err = addToNoText(word(onesText, int(ones)))
				}
				if err != nil {
					return noText, err
				}
			} else {
				if tens*10+ones > 0 {
					printExponent = true
					err := addToNoText(word(onesText, int(tens*10+ones)))
					if err != nil {
						return noText, err
					}
				}
			}
			if printExponent && exponent > 1 {
				err := addToNoText(word(exponentText, exponent-1))
				if err != nil {
					return noText, err
				}
			}
			whole -= group * power
			power /= 1000
		}
	}
	err := addToNoText(message("CHECK_WORDS_AND"))
	if err == nil {
		err = addToNoText(system.Format(cents%100) + "/100")
	}
	if err == nil {
		err = addToNoText(currencyCode)
	}
	return noText, err
}

// CurrencyText is the currency a check is written in: the bank account's, else the local
// currency code of the General Ledger Setup
func CurrencyText(src ftables.Source, bankCurrencyCode types.Code) string {
	if bankCurrencyCode != "" {
		return bankCurrencyCode.String()
	}
	return gtables.GetGeneralLedgerSetup(src).Lcy_code.String()
}
