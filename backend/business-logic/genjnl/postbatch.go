package genjnl

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/noseries"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// maxPostingNoSeries is how many posting number series one journal may use (classic array)
const maxPostingNoSeries = 10

// GenJnlPostBatch is codeunit 13 Gen. Jnl.-Post Batch: it posts the lines of a journal batch —
// every line checked, the batch's balance checked per posting date (per document with Force
// Doc. Balance), document numbers checked against the batch's number series and renumbered
// from the posting number series, every line posted — and deletes the posted lines. Open it on
// the posting's transaction; an error leaves everything to the caller's rollback.
//
// A recurring journal (recurring.go) posts its lines dated up to the work date that have not
// expired — with their allocations, the reversing lines the day after — and keeps them: each
// moves on by its Recurring Frequency, a variable or balance line without its amount. With an
// additional reporting currency a balance line takes the account's additional-currency balance
// too, and the allocations their share of it.
type GenJnlPostBatch struct {
	src             ftables.Source
	genJnlCheckLine GenJnlCheckLine
	genJnlPostLine  *GenJnlPostLine
	genJnlTemplate  *gtables.GenJournalTemplate
	genJnlBatch     *gtables.GenJournalBatch
	window          *system.Window

	noSeriesMgt        *noseries.Manager
	postingNoSeriesMgt map[types.Code]*noseries.Manager

	lastDate               types.Date
	currentBalanceReverse  types.Decimal
	startLineNoReverse     int
	lastDocType            gtables.GenJournalLineDocument_type
	lastDocNo              types.Code
	lastPostedDocNo        types.Code
	docCorrection          bool
	currentBalance         types.Decimal
	currencyBalance        types.Decimal
	lastCurrencyCode       types.Code
	startLineNo            int
	currentCustomerVendors int
	taxEntryCreated        bool
}

// PostBatchResult is what the posting tells its caller: the G/L Register it wrote (0: nothing
// posted) and the batch the journal is in afterwards (a numbered batch moves to the next name)
type PostBatchResult struct {
	RegisterNo       int
	JournalBatchName types.Code
}

// NewGenJnlPostBatch opens the batch posting in the posting's database (its transaction)
func NewGenJnlPostBatch(src ftables.Source) *GenJnlPostBatch {
	return &GenJnlPostBatch{src: src}
}

// Run posts the batch of a journal line (Code)
func (b *GenJnlPostBatch) Run(rec *gtables.GenJournalLine) (PostBatchResult, error) {
	genJnlLine := gtables.NewGenJournalLine(b.src)
	genJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	genJnlLine.SetRange("journal_batch_name", rec.Journal_batch_name)
	genJnlLine.LockTable()
	result := PostBatchResult{JournalBatchName: rec.Journal_batch_name}

	b.genJnlTemplate = gtables.NewGenJournalTemplate(b.src)
	err := b.genJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return result, err
	}
	b.genJnlBatch = gtables.NewGenJournalBatch(b.src)
	err = b.genJnlBatch.GetOrError(tables.GenJnlBatchKey(rec.Journal_template_name, rec.Journal_batch_name))
	if err != nil {
		return result, err
	}
	if system.StrLen(system.IncStr(b.genJnlBatch.Name.String())) > b.genJnlBatch.MaxStrLen("name") {
		return result, apperrors.JnlNameTooLong(gtables.GenJournalBatchTableName, "name", b.genJnlBatch.MaxStrLen("name"))
	}
	if b.genJnlTemplate.Recurring {
		workDate := system.WorkDate()
		genJnlLine.SetFilter("posting_date", "<="+workDate.String())
		genJnlLine.SetFilter("expiration_date", "''|>="+workDate.String())
	}
	if !genJnlLine.FindFirst() {
		return result, nil
	}

	language := fcodeunits.CurrentLanguage()
	windowText := "GEN_JNL_POST_WINDOW"
	if b.genJnlTemplate.Recurring {
		windowText = "GEN_JNL_POST_WINDOW_RECURRING"
	}
	b.window = system.OpenWindow(i18n.GetInstance().Message(windowText, language))
	defer b.window.Close()
	b.window.Update(1, rec.Journal_batch_name)

	// Check lines
	lineCount := 0
	for found := genJnlLine.FindSet(); found; found = genJnlLine.Next() {
		lineCount = lineCount + 1
		b.window.Update(2, lineCount)
		err = b.checkRecurringLine(genJnlLine)
		if err != nil {
			return result, err
		}
		err = b.updateRecurringAmt(genJnlLine)
		if err != nil {
			return result, err
		}
		err = b.checkAllocations(genJnlLine)
		if err != nil {
			return result, err
		}
		genJnlLine5 := gtables.NewGenJournalLine(b.src)
		genJnlLine5.Copy(genJnlLine)
		prepareGenJnlLineAddCurr(b.src, genJnlLine5)
		err = b.genJnlCheckLine.RunCheck(genJnlLine5)
		if err != nil {
			return result, err
		}
	}
	noOfRecords := lineCount

	// Check balance
	err = b.checkLinesBalance(genJnlLine, noOfRecords)
	if err != nil {
		return result, err
	}

	// Post lines
	b.genJnlPostLine = NewGenJnlPostLine(b.src)
	b.postingNoSeriesMgt = map[types.Code]*noseries.Manager{}
	lineCount = 0
	b.lastDocNo = ""
	b.lastPostedDocNo = ""
	var genJnlLine4 []*gtables.GenJournalLine // the reversing lines (a temporary table)
	for found := genJnlLine.FindSet(); found; found = genJnlLine.Next() {
		genJnlLine3 := gtables.NewGenJournalLine(b.src)
		genJnlLine3.Copy(genJnlLine)
		lineCount = lineCount + 1
		b.window.Update(5, lineCount)
		b.window.Update(6, system.Round(types.NewDecimalFromInt(int64(lineCount*10000/noOfRecords)), 1))
		b.makeRecurringText(genJnlLine3)
		err = b.checkDocumentNo(genJnlLine3)
		if err != nil {
			return result, err
		}
		genJnlLine5 := gtables.NewGenJournalLine(b.src)
		genJnlLine5.Copy(genJnlLine3)
		prepareGenJnlLineAddCurr(b.src, genJnlLine5)
		err = b.genJnlPostLine.RunWithoutCheck(genJnlLine5)
		if err != nil {
			return result, err
		}
		if isReversingMethod(genJnlLine3.Recurring_method) && !genJnlLine3.Posting_date.IsZero() {
			reversing := gtables.NewGenJournalLine(b.src)
			reversing.Copy(genJnlLine3)
			reversing.Posting_date = reversing.Posting_date.AddDays(1)
			multiplyAmounts(reversing, types.NewDecimalFromInt(-1))
			reversing.Reversing_entry = true
			genJnlLine4 = append(genJnlLine4, reversing)
		}
		err = b.postAllocations(genJnlLine3, false)
		if err != nil {
			return result, err
		}
	}

	// Post reversing lines
	lineCount = 0
	b.lastDocNo = ""
	b.lastPostedDocNo = ""
	for _, genJnlLine3 := range genJnlLine4 {
		lineCount = lineCount + 1
		b.window.Update(7, lineCount)
		b.window.Update(8, system.Round(types.NewDecimalFromInt(int64(lineCount*10000/len(genJnlLine4))), 1))
		err = b.checkDocumentNo(genJnlLine3)
		if err != nil {
			return result, err
		}
		genJnlLine5 := gtables.NewGenJournalLine(b.src)
		genJnlLine5.Copy(genJnlLine3)
		prepareGenJnlLineAddCurr(b.src, genJnlLine5)
		err = b.genJnlPostLine.RunWithoutCheck(genJnlLine5)
		if err != nil {
			return result, err
		}
		err = b.postAllocations(genJnlLine3, true)
		if err != nil {
			return result, err
		}
	}

	glReg := b.genJnlPostLine.GetGLReg()
	if glReg.To_entry_no != 0 {
		result.RegisterNo = glReg.No
	}

	// A recurring journal keeps its lines, the next date ahead
	if result.RegisterNo != 0 && b.genJnlTemplate.Recurring {
		err = b.updateRecurringLines(genJnlLine, noOfRecords)
		if err != nil {
			return result, err
		}
	}
	// Delete the posted lines; a numbered batch that is now empty moves to the next name
	if result.RegisterNo != 0 && !b.genJnlTemplate.Recurring {
		genJnlLine2 := gtables.NewGenJournalLine(b.src)
		genJnlLine2.CopyFilters(genJnlLine)
		genJnlLine2.SetFilter("account_no", "<>''")
		_ = genJnlLine2.FindLast() // the last line: a new line below it takes its values
		genJnlLine.DeleteAll()
		genJnlLine3 := gtables.NewGenJournalLine(b.src)
		genJnlLine3.SetRange("journal_template_name", rec.Journal_template_name)
		genJnlLine3.SetRange("journal_batch_name", rec.Journal_batch_name)
		if !genJnlLine3.FindLast() {
			if system.IncStr(rec.Journal_batch_name.String()) != "" {
				err = b.genJnlBatch.DeleteOrError(false)
				if err != nil {
					return result, err
				}
				b.genJnlBatch.Name = types.Code(system.IncStr(rec.Journal_batch_name.String()))
				_ = b.genJnlBatch.Insert(false) // IF INSERT THEN: an existing batch of that name is used
				result.JournalBatchName = b.genJnlBatch.Name
			}
		}
		genJnlLine3.SetRange("journal_batch_name", result.JournalBatchName)
		if b.genJnlBatch.No_series == "" && !genJnlLine3.FindLast() {
			genJnlLine3.Init()
			genJnlLine3.Journal_template_name = rec.Journal_template_name
			genJnlLine3.Journal_batch_name = result.JournalBatchName
			genJnlLine3.Line_no = 10000
			err = genJnlLine3.InsertOrError(false)
			if err != nil {
				return result, err
			}
			err = tables.SetUpNewGenJnlLine(genJnlLine3, genJnlLine2, types.Decimal{}, true)
			if err != nil {
				return result, err
			}
			err = genJnlLine3.ModifyOrError(false)
			if err != nil {
				return result, err
			}
		}
	}
	if b.noSeriesMgt != nil {
		err = b.noSeriesMgt.SaveNoSeries()
		if err != nil {
			return result, err
		}
	}
	for _, noSeriesMgt2 := range b.postingNoSeriesMgt {
		err = noSeriesMgt2.SaveNoSeries()
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

// checkLinesBalance checks the batch's lines: their document numbers against the batch's
// number series, Correction the same within a document, and the balance per posting date (per
// document with Force Doc. Balance) in LCY and in the lines' currency
func (b *GenJnlPostBatch) checkLinesBalance(genJnlLine *gtables.GenJournalLine, noOfRecords int) error {
	lineCount := 0
	b.lastDate = types.Date{}
	b.lastDocType = gtables.GenJournalLine_Document_type.Blank
	b.lastDocNo = ""
	b.currentCustomerVendors = 0
	b.taxEntryCreated = false
	b.currentBalance = types.Decimal{}
	b.currentBalanceReverse = types.Decimal{}
	b.currencyBalance = types.Decimal{}
	found := genJnlLine.FindSet()
	b.lastCurrencyCode = genJnlLine.Currency_code
	for ; found; found = genJnlLine.Next() {
		lineCount = lineCount + 1
		b.window.Update(3, lineCount)
		b.window.Update(4, system.Round(types.NewDecimalFromInt(int64(lineCount*10000/noOfRecords)), 1))
		if !tables.GenJnlLineEmptyLine(genJnlLine) {
			if b.genJnlBatch.No_series != "" && !genJnlLine.Check_printed && genJnlLine.Document_no != b.lastDocNo {
				if b.noSeriesMgt == nil {
					b.noSeriesMgt = noseries.New(b.src)
				}
				nextNo, err := b.noSeriesMgt.GetNextNo(b.genJnlBatch.No_series, genJnlLine.Posting_date, false)
				if err != nil {
					return err
				}
				if genJnlLine.Document_no != nextNo {
					err = b.noSeriesMgt.TestManual(b.genJnlBatch.No_series) // allow use of manual document numbers
					if err != nil {
						return err
					}
				}
			}
			if genJnlLine.Posting_date != b.lastDate || genJnlLine.Document_type != b.lastDocType || genJnlLine.Document_no != b.lastDocNo {
				if genJnlLine.Correction {
					err := b.genJnlTemplate.TestField("force_doc_balance", true)
					if err != nil {
						return err
					}
				}
				b.docCorrection = genJnlLine.Correction
			} else {
				if genJnlLine.Correction != b.docCorrection {
					return apperrors.JnlCorrectionSame(jnlLine, "correction")
				}
			}
		}
		if genJnlLine.Posting_date != b.lastDate ||
			b.genJnlTemplate.Force_doc_balance && (genJnlLine.Document_type != b.lastDocType || genJnlLine.Document_no != b.lastDocNo) {
			err := b.checkBalance()
			if err != nil {
				return err
			}
			b.currentCustomerVendors = 0
			b.taxEntryCreated = false
			b.currencyBalance = types.Decimal{}
			b.lastCurrencyCode = genJnlLine.Currency_code
		}
		if b.currentBalance.IsZero() {
			b.startLineNo = genJnlLine.Line_no
			b.currentCustomerVendors = 0
			b.taxEntryCreated = false
		}
		if b.currentBalanceReverse.IsZero() {
			b.startLineNoReverse = genJnlLine.Line_no
		}
		err := tables.UpdateGenJnlLineBalance(genJnlLine)
		if err != nil {
			return err
		}
		b.currentBalance = b.currentBalance.Add(genJnlLine.Balance_lcy)
		if isReversingMethod(genJnlLine.Recurring_method) {
			b.currentBalanceReverse = b.currentBalanceReverse.Add(genJnlLine.Balance_lcy)
		}
		if genJnlLine.Currency_code != b.lastCurrencyCode {
			b.lastCurrencyCode = ""
		} else {
			if genJnlLine.Currency_code != "" && (genJnlLine.Account_no == "") != (genJnlLine.Bal_account_no == "") {
				if genJnlLine.Account_no != "" {
					b.currencyBalance = b.currencyBalance.Add(genJnlLine.Amount)
				} else {
					b.currencyBalance = b.currencyBalance.Sub(genJnlLine.Amount)
				}
			}
		}
		b.lastDate = genJnlLine.Posting_date
		b.lastDocType = genJnlLine.Document_type
		b.lastDocNo = genJnlLine.Document_no
		if b.genJnlTemplate.Force_doc_balance {
			b.taxEntryCreated = b.taxEntryCreated ||
				genJnlLine.Account_type == gtables.GenJournalLine_Account_type.GLAccount && genJnlLine.Account_no != "" &&
					(genJnlLine.Gen_posting_type == gtables.GenJournalLine_Gen_posting_type.Purchase ||
						genJnlLine.Gen_posting_type == gtables.GenJournalLine_Gen_posting_type.Sale) ||
				genJnlLine.Bal_account_type == gtables.GenJournalLine_Bal_account_type.GLAccount && genJnlLine.Bal_account_no != "" &&
					(genJnlLine.Bal_gen_posting_type == gtables.GenJournalLine_Bal_gen_posting_type.Purchase ||
						genJnlLine.Bal_gen_posting_type == gtables.GenJournalLine_Bal_gen_posting_type.Sale)
			if (genJnlLine.Account_type == gtables.GenJournalLine_Account_type.Customer ||
				genJnlLine.Account_type == gtables.GenJournalLine_Account_type.Vendor) && genJnlLine.Account_no != "" ||
				(genJnlLine.Bal_account_type == gtables.GenJournalLine_Bal_account_type.Customer ||
					genJnlLine.Bal_account_type == gtables.GenJournalLine_Bal_account_type.Vendor) && genJnlLine.Bal_account_no != "" {
				b.currentCustomerVendors = b.currentCustomerVendors + 1
			}
			if b.currentCustomerVendors > 1 && b.taxEntryCreated {
				return apperrors.JnlTaxMultipleCustVend(jnlLine, genJnlLine.Document_no.String(), genJnlLine.Posting_date.String())
			}
		}
	}
	return b.checkBalance()
}

// checkBalance: the lines since the last balance (a date, or a document with Force Doc.
// Balance) add up to 0 — in LCY, and in their currency when they are all in one
func (b *GenJnlPostBatch) checkBalance() error {
	if !b.currentBalance.IsZero() {
		if b.genJnlTemplate.Force_doc_balance {
			return apperrors.JnlDocOutOfBalance(jnlLine, b.lastDocNo.String(), b.currentBalance.String())
		}
		return apperrors.JnlDateOutOfBalance(jnlLine, b.lastDate.String(), b.currentBalance.String())
	}
	if !b.currentBalanceReverse.IsZero() {
		if b.genJnlTemplate.Force_doc_balance {
			return apperrors.JnlReversingDocOutOfBalance(jnlLine, b.lastDocNo.String(), b.currentBalanceReverse.String())
		}
		return apperrors.JnlReversingOutOfBalance(jnlLine, b.lastDate.String(), b.currentBalanceReverse.String())
	}
	if b.lastCurrencyCode != "" && !b.currencyBalance.IsZero() {
		if b.genJnlTemplate.Force_doc_balance {
			return apperrors.JnlDocOutOfBalanceCurrency(jnlLine, b.lastDocNo.String(), b.currencyBalance.String(), b.lastCurrencyCode.String())
		}
		return apperrors.JnlDateOutOfBalanceCurrency(jnlLine, b.lastDate.String(), b.currencyBalance.String(), b.lastCurrencyCode.String())
	}
	return nil
}

// checkDocumentNo: a line without a posting number series records the batch's series; with
// one, its document gets the next posted number (all lines of a document the same)
func (b *GenJnlPostBatch) checkDocumentNo(genJnlLine2 *gtables.GenJournalLine) error {
	if genJnlLine2.Posting_no_series == "" {
		genJnlLine2.Posting_no_series = b.genJnlBatch.No_series
	} else {
		if !tables.GenJnlLineEmptyLine(genJnlLine2) {
			if genJnlLine2.Document_no == b.lastDocNo {
				genJnlLine2.Document_no = b.lastPostedDocNo
			} else {
				noSeriesMgt2, ok := b.postingNoSeriesMgt[genJnlLine2.Posting_no_series]
				if !ok {
					if len(b.postingNoSeriesMgt) >= maxPostingNoSeries {
						return apperrors.JnlMaxPostingNoSeries(maxPostingNoSeries)
					}
					noSeriesMgt2 = noseries.New(b.src)
					b.postingNoSeriesMgt[genJnlLine2.Posting_no_series] = noSeriesMgt2
				}
				b.lastDocNo = genJnlLine2.Document_no
				no, err := noSeriesMgt2.GetNextNo(genJnlLine2.Posting_no_series, genJnlLine2.Posting_date, false)
				if err != nil {
					return err
				}
				genJnlLine2.Document_no = no
				b.lastPostedDocNo = genJnlLine2.Document_no
			}
		}
	}
	return nil
}
