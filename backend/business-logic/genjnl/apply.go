package genjnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Pages 232 Apply Customer Entries and 233 Apply Vendor Entries, opened from a journal line
// (Apply Entries, codeunit 225) with the line's Applies-to ID, amount, posting date and document
// type as page parameters
const (
	ApplyCustEntriesPageID = 232
	ApplyVendEntriesPageID = 233
)

func init() {
	functions.Register("table", gtables.GenJournalLineTableName, "ApplyEntries", functions.For(GenJnlApply), functions.WithDialog())
	// (the ledger entry tables by their API names: a function finds its record type by it)
	functions.Register("table", "Customer_ledger_entry", "SetAppliesToID", functions.For(SetCustApplID))
	functions.Register("table", "Vendor_ledger_entry", "SetAppliesToID", functions.For(SetVendApplID))
	pages.OnAfterGetRecord(ApplyCustEntriesPageID, func(rec *gtables.CustomerLedgerEntry, page *pages.Control) {
		custLedgEntry := gtables.NewCustomerLedgerEntry(rec)
		custLedgEntry.SetRange("customer_no", rec.Customer_no)
		setApplnTotals(page, func(add func(remaining, pmtDiscPossible types.Decimal, invoice bool, pmtDiscountDate types.Date)) {
			custLedgEntry.SetRange("open", true)
			custLedgEntry.SetRange("applies_to_id", system.PageParam("applies_to_id"))
			for found := custLedgEntry.FindSet(); found; found = custLedgEntry.Next() {
				add(custLedgEntry.Remaining_amount, custLedgEntry.Pmt_disc_possible,
					custLedgEntry.Document_type == gtables.CustomerLedgerEntry_Document_type.Invoice, custLedgEntry.Pmt_discount_date)
			}
		})
	})
	pages.OnAfterGetRecord(ApplyVendEntriesPageID, func(rec *gtables.VendorLedgerEntry, page *pages.Control) {
		vendLedgEntry := gtables.NewVendorLedgerEntry(rec)
		vendLedgEntry.SetRange("vendor_no", rec.Vendor_no)
		setApplnTotals(page, func(add func(remaining, pmtDiscPossible types.Decimal, invoice bool, pmtDiscountDate types.Date)) {
			vendLedgEntry.SetRange("open", true)
			vendLedgEntry.SetRange("applies_to_id", system.PageParam("applies_to_id"))
			for found := vendLedgEntry.FindSet(); found; found = vendLedgEntry.Next() {
				add(vendLedgEntry.Remaining_amount, vendLedgEntry.Pmt_disc_possible,
					vendLedgEntry.Document_type == gtables.VendorLedgerEntry_Document_type.Invoice, vendLedgEntry.Pmt_discount_date)
			}
		})
	})
}

// setApplnTotals fills the Apply Entries footer: the journal line's amount, the amount of the
// entries marked with its Applies-to ID (a payment takes the discount still possible), and the
// balance of both
func setApplnTotals(page *pages.Control, entries func(add func(remaining, pmtDiscPossible types.Decimal, invoice bool, pmtDiscountDate types.Date))) {
	applyingAmount, _ := types.NewDecimalFromString(system.PageParam("amount"))
	postingDate, _ := types.NewDateFromString(system.PageParam("posting_date"))
	payment := system.PageParam("document_type") == "1"
	var appliedAmount types.Decimal
	if system.PageParam("applies_to_id") != "" {
		entries(func(remaining, pmtDiscPossible types.Decimal, invoice bool, pmtDiscountDate types.Date) {
			if payment && invoice && !pmtDiscountDate.Before(postingDate) {
				remaining = remaining.Sub(pmtDiscPossible)
			}
			appliedAmount = appliedAmount.Add(remaining)
		})
	}
	page.SetValue("appln_amount", applyingAmount)
	page.SetValue("applied_amount", appliedAmount)
	page.SetValue("appln_balance", appliedAmount.Add(applyingAmount))
}

// SetCustApplID (codeunit 101 CustEntry-SetAppl.ID) marks the selected customer entries with the
// journal line's Applies-to ID (the page's parameter, else the user ID), or takes the mark off
// when the first one has it
func SetCustApplID(rec *gtables.CustomerLedgerEntry) (fcodeunits.Result, error) {
	custLedgEntry := rec
	custLedgEntry.SetSelectionFilter()
	custLedgEntry.LockTable()
	if custLedgEntry.FindFirst() {
		custEntryApplID := entryApplID(custLedgEntry.Applies_to_id)
		for found := custLedgEntry.FindSetBuffered(); found; found = custLedgEntry.Next() { // modified in the loop: read all first
			err := custLedgEntry.TestField("open", true)
			if err != nil {
				return fcodeunits.Result{}, err
			}
			custLedgEntry.Applies_to_id = custEntryApplID
			err = custLedgEntry.ModifyOrError(false)
			if err != nil {
				return fcodeunits.Result{}, err
			}
		}
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// SetVendApplID (codeunit 111 VendEntry-SetAppl.ID) marks the selected vendor entries
func SetVendApplID(rec *gtables.VendorLedgerEntry) (fcodeunits.Result, error) {
	vendLedgEntry := rec
	vendLedgEntry.SetSelectionFilter()
	vendLedgEntry.LockTable()
	if vendLedgEntry.FindFirst() {
		vendEntryApplID := entryApplID(vendLedgEntry.Applies_to_id)
		for found := vendLedgEntry.FindSetBuffered(); found; found = vendLedgEntry.Next() { // modified in the loop: read all first
			err := vendLedgEntry.TestField("open", true)
			if err != nil {
				return fcodeunits.Result{}, err
			}
			vendLedgEntry.Applies_to_id = vendEntryApplID
			err = vendLedgEntry.ModifyOrError(false)
			if err != nil {
				return fcodeunits.Result{}, err
			}
		}
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// entryApplID: an entry with an Applies-to ID loses it; otherwise it gets the journal line's,
// the user ID without one, or ***
func entryApplID(current types.Code) types.Code {
	if current != "" {
		return ""
	}
	appliesToID := types.Code(system.PageParam("applies_to_id"))
	if appliesToID == "" {
		appliesToID = types.Code(system.UserId())
		if appliesToID == "" {
			appliesToID = "***"
		}
	}
	return appliesToID
}

// GenJnlApply is codeunit 225 Gen. Jnl.-Apply (Apply Entries, Shift+F9): it opens the open
// entries of the line's customer or vendor; the user marks the entries the line pays with its
// Applies-to ID (F9) — the line's amount, when it has none, becomes what they still owe. When
// the user picks an entry without marking any, the line applies to that document (as the
// Applies-to Doc. No. lookup does).
func GenJnlApply(rec *gtables.GenJournalLine) (fcodeunits.Result, error) {
	genJnlLine := rec
	accType := genJnlLine.Account_type
	accNo := genJnlLine.Account_no
	if genJnlLine.Bal_account_type == gtables.GenJournalLine_Bal_account_type.Customer ||
		genJnlLine.Bal_account_type == gtables.GenJournalLine_Bal_account_type.Vendor {
		accType = gtables.GenJournalLineAccount_type(genJnlLine.Bal_account_type)
		accNo = genJnlLine.Bal_account_no
	}
	if accType != gtables.GenJournalLine_Account_type.Customer && accType != gtables.GenJournalLine_Account_type.Vendor {
		return fcodeunits.Result{}, apperrors.JnlCustOrVend(jnlLine, "account_type", "bal_account_type")
	}
	if genJnlLine.Applies_to_id == "" {
		genJnlLine.Applies_to_id = genJnlLine.Document_no
	}
	if genJnlLine.Applies_to_id == "" {
		return fcodeunits.Result{}, apperrors.JnlSpecifyOne(jnlLine, "document_no", "applies_to_id")
	}
	currency, err := applyCurrency(genJnlLine)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	var entries []appliedEntry
	var picked *appliedEntry
	var ok bool
	if accType == gtables.GenJournalLine_Account_type.Customer {
		entries, picked, ok, err = applyCustEntries(genJnlLine, accNo)
	} else {
		entries, picked, ok, err = applyVendEntries(genJnlLine, accNo)
	}
	if err != nil || !ok {
		return fcodeunits.Result{Success: true}, err
	}

	if len(entries) == 0 && picked != nil {
		// No entry marked: the line applies to the picked document
		entries = []appliedEntry{*picked}
		genJnlLine.Applies_to_id = ""
		genJnlLine.Applies_to_doc_type = picked.docType
		genJnlLine.Applies_to_doc_no = picked.docNo
	} else if len(entries) > 0 {
		genJnlLine.Applies_to_doc_type = gtables.GenJournalLine_Applies_to_doc_type.Blank
		genJnlLine.Applies_to_doc_no = ""
	} else {
		genJnlLine.Applies_to_id = ""
	}
	if len(entries) > 0 {
		currencyCode2 := entries[0].currencyCode
		for _, entry := range entries {
			err = CheckCurrency(currencyCode2, entry.currencyCode)
			if err != nil {
				return fcodeunits.Result{}, err
			}
		}
		if genJnlLine.Amount.IsZero() {
			if genJnlLine.Currency_code != currencyCode2 {
				err = changeLineCurrency(genJnlLine, currencyCode2)
				if err != nil {
					return fcodeunits.Result{}, err
				}
				currency, err = applyCurrency(genJnlLine)
				if err != nil {
					return fcodeunits.Result{}, err
				}
			}
			amount := genJnlLine.Amount
			for _, entry := range entries {
				remaining := system.Round(entry.remaining, currency.Amount_rounding_precision)
				pmtDiscPossible := system.Round(entry.pmtDiscPossible, currency.Amount_rounding_precision)
				if genJnlLine.Document_type == gtables.GenJournalLine_Document_type.Payment &&
					entry.docType == gtables.GenJournalLine_Applies_to_doc_type.Invoice &&
					!entry.pmtDiscountDate.Before(genJnlLine.Posting_date) {
					amount = amount.Sub(remaining.Sub(pmtDiscPossible))
				} else {
					amount = amount.Sub(remaining)
				}
			}
			if genJnlLine.Bal_account_type == gtables.GenJournalLine_Bal_account_type.Customer ||
				genJnlLine.Bal_account_type == gtables.GenJournalLine_Bal_account_type.Vendor {
				amount = amount.Neg()
			}
			err = genJnlLine.Validate("amount", amount)
			if err != nil {
				return fcodeunits.Result{}, err
			}
		} else {
			err = CheckCurrency(genJnlLine.Currency_code, currencyCode2)
			if err != nil {
				return fcodeunits.Result{}, err
			}
		}
	}
	err = genJnlLine.ModifyOrError(true)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// appliedEntry is an open customer or vendor entry the journal line applies to
type appliedEntry struct {
	docType         gtables.GenJournalLineApplies_to_doc_type
	docNo           types.Code
	currencyCode    types.Code
	remaining       types.Decimal
	pmtDiscPossible types.Decimal
	pmtDiscountDate types.Date
}

// applyPageParams are the journal line's values the Apply Entries page shows and its F9 uses
func applyPage(pageID int, genJnlLine *gtables.GenJournalLine) *system.Page {
	page := system.NewPage(pageID)
	page.LookupMode(true)
	page.SetParam("applies_to_id", genJnlLine.Applies_to_id.String())
	page.SetParam("amount", genJnlLine.Amount.String())
	page.SetParam("posting_date", genJnlLine.Posting_date.String())
	page.SetParam("document_type", itoa(int(genJnlLine.Document_type)))
	return page
}

// applyCustEntries opens Apply Customer Entries over the customer's open entries and returns
// the entries marked with the line's Applies-to ID and the entry the user picked; ok is false
// when the user cancelled
func applyCustEntries(genJnlLine *gtables.GenJournalLine, accNo types.Code) ([]appliedEntry, *appliedEntry, bool, error) {
	custLedgEntry := gtables.NewCustomerLedgerEntry(genJnlLine)
	custLedgEntry.SetCurrentKey("customer_no", "open", "positive")
	custLedgEntry.SetRange("customer_no", accNo)
	custLedgEntry.SetRange("open", true)
	page := applyPage(ApplyCustEntriesPageID, genJnlLine)
	page.SetTableView(custLedgEntry)
	if page.RunModal() != system.ActionLookupOK {
		return nil, nil, false, nil
	}
	var picked *appliedEntry
	if page.GetRecord(custLedgEntry) {
		picked = &appliedEntry{gtables.GenJournalLineApplies_to_doc_type(custLedgEntry.Document_type), custLedgEntry.Document_no,
			custLedgEntry.Currency_code, custLedgEntry.Remaining_amount, custLedgEntry.Pmt_disc_possible, custLedgEntry.Pmt_discount_date}
	}
	marked := gtables.NewCustomerLedgerEntry(genJnlLine)
	marked.SetCurrentKey("customer_no", "open", "positive")
	marked.SetRange("customer_no", accNo)
	marked.SetRange("open", true)
	marked.SetRange("applies_to_id", genJnlLine.Applies_to_id)
	var entries []appliedEntry
	for found := marked.FindSet(); found; found = marked.Next() {
		entries = append(entries, appliedEntry{gtables.GenJournalLineApplies_to_doc_type(marked.Document_type), marked.Document_no,
			marked.Currency_code, marked.Remaining_amount, marked.Pmt_disc_possible, marked.Pmt_discount_date})
	}
	return entries, picked, true, nil
}

// applyVendEntries opens Apply Vendor Entries (see applyCustEntries)
func applyVendEntries(genJnlLine *gtables.GenJournalLine, accNo types.Code) ([]appliedEntry, *appliedEntry, bool, error) {
	vendLedgEntry := gtables.NewVendorLedgerEntry(genJnlLine)
	vendLedgEntry.SetCurrentKey("vendor_no", "open", "positive")
	vendLedgEntry.SetRange("vendor_no", accNo)
	vendLedgEntry.SetRange("open", true)
	page := applyPage(ApplyVendEntriesPageID, genJnlLine)
	page.SetTableView(vendLedgEntry)
	if page.RunModal() != system.ActionLookupOK {
		return nil, nil, false, nil
	}
	var picked *appliedEntry
	if page.GetRecord(vendLedgEntry) {
		picked = &appliedEntry{gtables.GenJournalLineApplies_to_doc_type(vendLedgEntry.Document_type), vendLedgEntry.Document_no,
			vendLedgEntry.Currency_code, vendLedgEntry.Remaining_amount, vendLedgEntry.Pmt_disc_possible, vendLedgEntry.Pmt_discount_date}
	}
	marked := gtables.NewVendorLedgerEntry(genJnlLine)
	marked.SetCurrentKey("vendor_no", "open", "positive")
	marked.SetRange("vendor_no", accNo)
	marked.SetRange("open", true)
	marked.SetRange("applies_to_id", genJnlLine.Applies_to_id)
	var entries []appliedEntry
	for found := marked.FindSet(); found; found = marked.Next() {
		entries = append(entries, appliedEntry{gtables.GenJournalLineApplies_to_doc_type(marked.Document_type), marked.Document_no,
			marked.Currency_code, marked.Remaining_amount, marked.Pmt_disc_possible, marked.Pmt_discount_date})
	}
	return entries, picked, true, nil
}

// applyCurrency is the line's currency (its rounding precision)
func applyCurrency(genJnlLine *gtables.GenJournalLine) (*gtables.Currency, error) {
	currency := gtables.NewCurrency(genJnlLine)
	if genJnlLine.Currency_code == "" {
		tables.InitRoundingPrecision(currency)
		return currency, nil
	}
	err := currency.GetOrError(genJnlLine.Currency_code)
	if err != nil {
		return nil, err
	}
	return currency, nil
}

// changeLineCurrency: after asking, the line takes the currency of the entries it applies to
func changeLineCurrency(genJnlLine *gtables.GenJournalLine, currencyCode types.Code) error {
	language := fcodeunits.CurrentLanguage()
	ts := i18n.GetInstance()
	fromCurrency := genJnlLine.Currency_code.String()
	if fromCurrency == "" {
		fromCurrency = gtables.GetGeneralLedgerSetup(genJnlLine).Lcy_code.String()
	}
	if !system.Confirm(ts.Message("GEN_JNL_APPLY_CHANGE_CURRENCY", language),
		ts.FieldCaption(jnlLine, "currency_code", language), ts.TableCaption(jnlLine, language), fromCurrency, currencyCode) {
		return apperrors.JnlUpdateInterrupted()
	}
	return genJnlLine.Validate("currency_code", currencyCode)
}
