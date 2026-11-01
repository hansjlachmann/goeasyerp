package tables

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/salestax"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// reminderLineLogic is the business logic of table 296 Reminder Line: a text, an additional fee
// (G/L account) or an overdue customer entry with its interest.
type reminderLineLogic struct{}

func init() { gtables.RegisterReminderLineLogic(reminderLineLogic{}) }

// OnInsert: the reminder exists
func (reminderLineLogic) OnInsert(rec *gtables.ReminderLine) error {
	reminderHeader := gtables.NewReminderHeader(rec)
	err := reminderHeader.GetOrError(rec.Reminder_no)
	if err != nil {
		return err
	}
	rec.Attached_to_line_no = 0
	return nil
}

// OnDelete: the lines attached to it
func (reminderLineLogic) OnDelete(rec *gtables.ReminderLine) error {
	reminderLine := gtables.NewReminderLine(rec)
	reminderLine.SetRange("reminder_no", rec.Reminder_no)
	reminderLine.SetRange("attached_to_line_no", rec.Line_no)
	reminderLine.DeleteAll()
	return nil
}

// getReminderHeader (GetReminderHeader, ProcessReminderHeader): the line's reminder — with a
// customer, document date, posting group and terms — and its currency's rounding
func getReminderHeader(src ftables.Source, reminderNo types.Code) (*gtables.ReminderHeader, *gtables.Currency, error) {
	reminderHeader := gtables.NewReminderHeader(src)
	err := reminderHeader.GetOrError(reminderNo)
	if err != nil {
		return nil, nil, err
	}
	for _, field := range []string{"customer_no", "document_date", "customer_posting_group", "reminder_terms_code"} {
		err = reminderHeader.TestField(field)
		if err != nil {
			return nil, nil, err
		}
	}
	reminderTerms := gtables.NewReminderTerms(src)
	err = reminderTerms.GetOrError(reminderHeader.Reminder_terms_code)
	if err != nil {
		return nil, nil, err
	}
	currency := gtables.NewCurrency(src)
	if reminderHeader.Currency_code == "" {
		InitRoundingPrecision(currency)
	} else {
		err = currency.GetOrError(reminderHeader.Currency_code)
		if err == nil {
			err = currency.TestField("amount_rounding_precision")
		}
		if err != nil {
			return nil, nil, err
		}
	}
	return reminderHeader, currency, nil
}

// OnValidate_Type: a new type clears the line
func (reminderLineLogic) OnValidate_Type(rec *gtables.ReminderLine) error {
	xRec := rec.XRec()
	if rec.Type != xRec.Type {
		lineType := rec.Type
		rec.Init()
		rec.Type = lineType
		_, _, err := getReminderHeader(rec, rec.Reminder_no)
		return err
	}
	return nil
}

// OnValidate_Entry_no: an open entry of the reminder's customer in its currency — its dates,
// document, amounts, the next reminder level (the last reminder's + 1 when overdue) and interest
func (reminderLineLogic) OnValidate_Entry_no(rec *gtables.ReminderLine) error {
	err := rec.TestField("type", gtables.ReminderLine_Type.CustomerLedgerEntry)
	if err != nil {
		return err
	}
	reminderHeader, _, err := getReminderHeader(rec, rec.Reminder_no)
	if err != nil {
		return err
	}
	custLedgEntry := gtables.NewCustomerLedgerEntry(rec)
	err = custLedgEntry.GetOrError(rec.Entry_no)
	if err == nil {
		err = custLedgEntry.TestField("open", true)
	}
	if err == nil {
		err = custLedgEntry.TestField("customer_no", reminderHeader.Customer_no)
	}
	if err != nil {
		return err
	}
	if custLedgEntry.Currency_code != reminderHeader.Currency_code {
		return apperrors.ReminderCurrencyMismatch(gtables.ReminderHeaderTableName)
	}
	rec.Posting_date = custLedgEntry.Posting_date
	rec.Document_date = custLedgEntry.Document_date
	rec.Due_date = custLedgEntry.Due_date
	rec.Document_type = gtables.ReminderLineDocument_type(custLedgEntry.Document_type)
	rec.Document_no = custLedgEntry.Document_no
	rec.Description = custLedgEntry.Description
	rec.Original_amount = custLedgEntry.Amount
	rec.Remaining_amount = custLedgEntry.Remaining_amount
	reminderEntry := gtables.NewReminderFinChargeEntry(rec)
	reminderEntry.SetCurrentKey("customer_entry_no", "type")
	reminderEntry.SetRange("customer_entry_no", rec.Entry_no)
	reminderEntry.SetRange("type", gtables.ReminderFinChargeEntry_Type.Reminder)
	if reminderEntry.FindLast() {
		rec.Reminder_level = reminderEntry.Reminder_level
	} else {
		rec.Reminder_level = 0
	}
	if custLedgEntry.On_hold == "" && rec.Due_date.Before(reminderHeader.Document_date) {
		rec.Reminder_level = rec.Reminder_level + 1
	}
	return calcReminderFinChrg(rec, reminderHeader)
}

// OnValidate_Document_type: the entry again by the document
func (reminderLineLogic) OnValidate_Document_type(rec *gtables.ReminderLine) error {
	err := rec.TestField("type", gtables.ReminderLine_Type.CustomerLedgerEntry)
	if err != nil {
		return err
	}
	return rec.Validate("document_no", rec.Document_no)
}

// OnValidate_Document_no: the customer's open entry of the document
func (reminderLineLogic) OnValidate_Document_no(rec *gtables.ReminderLine) error {
	err := rec.TestField("type", gtables.ReminderLine_Type.CustomerLedgerEntry)
	if err != nil {
		return err
	}
	rec.Entry_no = 0
	if rec.Document_no != "" {
		reminderHeader, _, err := getReminderHeader(rec, rec.Reminder_no)
		if err != nil {
			return err
		}
		custLedgEntry := gtables.NewCustomerLedgerEntry(rec)
		custLedgEntry.SetCurrentKey("customer_no", "open")
		custLedgEntry.SetRange("customer_no", reminderHeader.Customer_no)
		custLedgEntry.SetRange("open", true)
		if rec.Document_type != 0 {
			custLedgEntry.SetRange("document_type", gtables.CustomerLedgerEntryDocument_type(rec.Document_type))
		}
		custLedgEntry.SetRange("document_no", rec.Document_no)
		if custLedgEntry.FindFirst() {
			return rec.Validate("entry_no", custLedgEntry.Entry_no)
		}
	}
	return nil
}

// OnValidate_No: a standard text's description, or a G/L account (posting, direct posting, with a
// general product posting group) with its tax groups; not on customer entry lines
func (reminderLineLogic) OnValidate_No(rec *gtables.ReminderLine) error {
	if rec.No == "" {
		return nil
	}
	switch rec.Type {
	case gtables.ReminderLine_Type.Blank:
		standardText := gtables.NewStandardText(rec)
		err := standardText.GetOrError(rec.No)
		if err != nil {
			return err
		}
		rec.Description = standardText.Description
	case gtables.ReminderLine_Type.CustomerLedgerEntry:
		return apperrors.ReminderLineTypeMustBe(gtables.ReminderLineTableName, "type", gtables.ReminderLine_Type.Blank, gtables.ReminderLine_Type.GLAccount)
	case gtables.ReminderLine_Type.GLAccount:
		glAcc := gtables.NewGLAccount(rec)
		err := glAcc.GetOrError(rec.No)
		if err == nil {
			err = CheckGLAcc(glAcc)
		}
		if err == nil {
			err = glAcc.TestField("direct_posting", true)
		}
		if err == nil {
			err = glAcc.TestField("gen_prod_posting_group")
		}
		if err != nil {
			return err
		}
		rec.Description = glAcc.Name
		_, _, err = getReminderHeader(rec, rec.Reminder_no)
		if err != nil {
			return err
		}
		rec.Tax_group_code = glAcc.Tax_group_code
		err = rec.Validate("gen_prod_posting_group", glAcc.Gen_prod_posting_group)
		if err == nil {
			err = rec.Validate("tax_prod_posting_group", glAcc.Tax_prod_posting_group)
		}
		return err
	}
	return nil
}

// OnValidate_Amount: not on text lines; rounded, with its tax (sales tax: the reminder's tax
// area)
func (reminderLineLogic) OnValidate_Amount(rec *gtables.ReminderLine) error {
	if rec.Type == gtables.ReminderLine_Type.Blank {
		return apperrors.ReminderLineTypeMustBe(gtables.ReminderLineTableName, "type", gtables.ReminderLine_Type.GLAccount, gtables.ReminderLine_Type.CustomerLedgerEntry)
	}
	reminderHeader, currency, err := getReminderHeader(rec, rec.Reminder_no)
	if err != nil {
		return err
	}
	rec.Amount = system.Round(rec.Amount, currency.Amount_rounding_precision)
	rec.Tax_amount, rec.Tax_percent, err = reminderLineTax(rec, reminderHeader.Tax_area_code, rec.Tax_group_code, reminderHeader.Tax_liable,
		reminderHeader.Posting_date, rec.Amount, rec.Tax_percent, int(rec.Tax_calculation_type))
	if err != nil {
		return err
	}
	rec.Tax_amount = system.Round(rec.Tax_amount, currency.Amount_rounding_precision)
	return nil
}

// reminderLineTax: the tax of a reminder or finance charge memo line's amount — VAT at the line's
// Tax %, sales tax from the tax area (its Tax % follows)
func reminderLineTax(src ftables.Source, taxAreaCode, taxGroupCode types.Code, taxLiable bool, date types.Date, amount, taxPercent types.Decimal, taxCalculationType int) (types.Decimal, types.Decimal, error) {
	switch gtables.TaxPostingSetupTax_calculation_type(taxCalculationType) {
	case gtables.TaxPostingSetup_Tax_calculation_type.SalesTax:
		taxAmount, err := salestax.New(src).CalculateTax(taxAreaCode, taxGroupCode, taxLiable, date, amount, types.Decimal{}, types.NewDecimalFromInt(100))
		if err != nil {
			return types.Decimal{}, types.Decimal{}, err
		}
		if amount.Sub(taxAmount).IsZero() {
			return taxAmount, types.Decimal{}, nil
		}
		return taxAmount, system.Round(taxAmount.Mul(types.NewDecimalFromInt(100)).Div(amount), types.MustDecimal("0.00001")), nil
	default:
		return amount.Mul(taxPercent).Div(types.NewDecimalFromInt(100)), taxPercent, nil
	}
}

// OnValidate_Interest_rate: the interest again (CalcFinChrg takes the finance charge terms' rate)
func (reminderLineLogic) OnValidate_Interest_rate(rec *gtables.ReminderLine) error {
	err := rec.TestField("type", gtables.ReminderLine_Type.CustomerLedgerEntry)
	if err == nil {
		err = rec.TestField("entry_no")
	}
	if err != nil {
		return err
	}
	reminderHeader, _, err := getReminderHeader(rec, rec.Reminder_no)
	if err != nil {
		return err
	}
	return calcReminderFinChrg(rec, reminderHeader)
}

// OnValidate_Gen_prod_posting_group: the group's default tax product posting group
func (reminderLineLogic) OnValidate_Gen_prod_posting_group(rec *gtables.ReminderLine) error {
	xRec := rec.XRec()
	if xRec.Gen_prod_posting_group != rec.Gen_prod_posting_group {
		genProdPostingGroup := gtables.NewGenProductPostingGroup(rec)
		ok, err := ValidateTaxProdPostingGroup(genProdPostingGroup, rec.Gen_prod_posting_group)
		if err != nil {
			return err
		}
		if ok {
			return rec.Validate("tax_prod_posting_group", genProdPostingGroup.Def_tax_prod_posting_group)
		}
	}
	return nil
}

// OnValidate_Tax_group_code: the tax again
func (reminderLineLogic) OnValidate_Tax_group_code(rec *gtables.ReminderLine) error {
	return rec.Validate("tax_prod_posting_group", rec.Tax_prod_posting_group)
}

// OnValidate_Tax_prod_posting_group: the Tax Posting Setup's Tax % and calculation type, the tax
// again
func (reminderLineLogic) OnValidate_Tax_prod_posting_group(rec *gtables.ReminderLine) error {
	reminderHeader, _, err := getReminderHeader(rec, rec.Reminder_no)
	if err != nil {
		return err
	}
	taxPostingSetup := gtables.NewTaxPostingSetup(rec)
	if !taxPostingSetup.Get(map[string]interface{}{"tax_bus_posting_group": reminderHeader.Tax_bus_posting_group, "tax_prod_posting_group": rec.Tax_prod_posting_group}) {
		taxPostingSetup.Init()
	}
	rec.Tax_percent = taxPostingSetup.Vat_percent
	rec.Tax_calculation_type = gtables.ReminderLineTax_calculation_type(taxPostingSetup.Tax_calculation_type)
	if rec.Tax_calculation_type == gtables.ReminderLine_Tax_calculation_type.ReverseChargeVAT {
		rec.Tax_percent = types.Decimal{}
	}
	if rec.Type == gtables.ReminderLine_Type.Blank {
		return nil
	}
	return rec.Validate("amount", rec.Amount)
}

// calcReminderFinChrg (CalcFinChrg): the interest of an overdue entry at the reminder's level —
// when the level calculates interest and the reminder has finance charge terms: Average Daily
// Balance (since the last interest posted or the due date, with the closed entries' interest by
// Interest Calculation) or Balance Due (after the grace period)
func calcReminderFinChrg(rec *gtables.ReminderLine, reminderHeader *gtables.ReminderHeader) error {
	rec.Interest_rate = types.Decimal{}
	rec.Amount = types.Decimal{}
	rec.Tax_amount = types.Decimal{}
	rec.Tax_calculation_type = gtables.ReminderLine_Tax_calculation_type.NormalVAT
	rec.Gen_prod_posting_group = ""
	rec.Tax_prod_posting_group = ""
	custLedgEntry := gtables.NewCustomerLedgerEntry(rec)
	err := custLedgEntry.GetOrError(rec.Entry_no)
	if err != nil {
		return err
	}
	if custLedgEntry.On_hold != "" || !rec.Due_date.Before(reminderHeader.Document_date) {
		return nil
	}
	reminderLevel := gtables.NewReminderLevel(rec)
	reminderLevel.SetRange("reminder_terms_code", reminderHeader.Reminder_terms_code)
	reminderLevel.SetRange("no", 1, reminderHeader.Reminder_level)
	if !reminderLevel.FindLast() {
		reminderLevel.Init()
	}
	if !reminderLevel.Calculate_interest || reminderHeader.Fin_charge_terms_code == "" {
		return nil
	}
	amount, interestRate, err := CalcEntryInterest(rec, custLedgEntry, reminderHeader.Fin_charge_terms_code, rec.Due_date, rec.Remaining_amount, reminderHeader.Document_date, false)
	if err != nil {
		return err
	}
	rec.Interest_rate = interestRate
	rec.Amount = amount
	if !rec.Amount.IsZero() {
		return setInterestPostingGroups(rec, reminderHeader.Customer_posting_group, func(genProd, taxProd types.Code) error {
			err := rec.Validate("gen_prod_posting_group", genProd)
			if err == nil {
				err = rec.Validate("tax_prod_posting_group", taxProd)
			}
			return err
		})
	}
	return nil
}

// setInterestPostingGroups: the product posting groups of the customer posting group's Interest
// Account
func setInterestPostingGroups(src ftables.Source, customerPostingGroup types.Code, set func(genProd, taxProd types.Code) error) error {
	custPostingGroup := gtables.NewCustomerPostingGroup(src)
	err := custPostingGroup.GetOrError(customerPostingGroup)
	if err == nil {
		err = custPostingGroup.TestField("interest_account")
	}
	if err != nil {
		return err
	}
	glAcc := gtables.NewGLAccount(src)
	err = glAcc.GetOrError(custPostingGroup.Interest_account)
	if err == nil {
		err = glAcc.TestField("gen_prod_posting_group")
	}
	if err != nil {
		return err
	}
	return set(glAcc.Gen_prod_posting_group, glAcc.Tax_prod_posting_group)
}

// CalcEntryInterest: the interest of a customer entry by finance charge terms up to a document
// date — the shared part of the reminder line's and the finance charge memo line's CalcFinChrg:
// Average Daily Balance (remaining amount × days since the last interest posted or the due date,
// with the amounts of the entries closed with it, per Interest Period) when past the grace
// period; Balance Due: the remaining amount × rate after the grace period. A reminder counts the
// grace period from the last interest date and adds closed entries by Interest Calculation; a
// finance charge memo (financeChargeMemo) counts it from the due date and adds them when the entry
// has Calculate Interest.
func CalcEntryInterest(src ftables.Source, custLedgEntry *gtables.CustomerLedgerEntry, finChrgTermsCode types.Code, dueDate types.Date,
	remainingAmount types.Decimal, documentDate types.Date, financeChargeMemo bool) (types.Decimal, types.Decimal, error) {
	finChrgTerms := gtables.NewFinanceChargeTerms(src)
	err := finChrgTerms.GetOrError(finChrgTermsCode)
	if err != nil {
		return types.Decimal{}, types.Decimal{}, err
	}
	interestRate := finChrgTerms.Interest_rate
	var amount types.Decimal
	switch finChrgTerms.Interest_calculation_method {
	case gtables.FinanceChargeTerms_Interest_calculation_method.AverageDailyBalance:
		calcInterest := false
		err = finChrgTerms.TestField("interest_period_days")
		if err != nil {
			return types.Decimal{}, types.Decimal{}, err
		}
		interestCalcDate := lastInterestDate(src, custLedgEntry.Entry_no)
		if interestCalcDate.IsZero() {
			interestCalcDate = dueDate
		}
		graceStart := interestCalcDate
		if financeChargeMemo {
			graceStart = dueDate
		}
		graceEnd, err := CalcDateFormula(finChrgTerms.Grace_period, graceStart)
		if err != nil {
			return types.Decimal{}, types.Decimal{}, err
		}
		if graceEnd.Before(documentDate) && !remainingAmount.IsZero() {
			calcInterest = true
		}
		if interestCalcDate.Before(documentDate) {
			amount = remainingAmount.Mul(types.NewDecimalFromInt(int64(interestCalcDate.DaysBetween(documentDate))))
		}
		includeClosed := finChrgTerms.Interest_calculation == gtables.FinanceChargeTerms_Interest_calculation.ClosedEntries
		if financeChargeMemo {
			includeClosed = custLedgEntry.Calculate_interest
		}
		if includeClosed {
			// the entries applied to it, closed after the interest date
			calcFinChrg2 := func(custLedgEntry2 *gtables.CustomerLedgerEntry, appliedAmount types.Decimal) (bool, error) {
				if interestCalcDate.Before(custLedgEntry2.Document_date) && !custLedgEntry2.Closing_interest_calculated {
					amount = amount.Add(appliedAmount.Mul(types.NewDecimalFromInt(int64(interestCalcDate.DaysBetween(custLedgEntry2.Document_date)))))
					graceEnd, err := CalcDateFormula(finChrgTerms.Grace_period, interestCalcDate)
					if err != nil {
						return false, err
					}
					return graceEnd.Before(custLedgEntry2.Document_date) && !appliedAmount.IsZero(), nil
				}
				return false, nil
			}
			custLedgEntry2 := gtables.NewCustomerLedgerEntry(src)
			if custLedgEntry2.Get(custLedgEntry.Closed_by_entry_no) {
				ok, err := calcFinChrg2(custLedgEntry2, custLedgEntry.Closed_by_amount)
				if err != nil {
					return types.Decimal{}, types.Decimal{}, err
				}
				calcInterest = calcInterest || ok
			}
			custLedgEntry2 = gtables.NewCustomerLedgerEntry(src)
			custLedgEntry2.SetRange("closed_by_entry_no", custLedgEntry.Entry_no)
			for found := custLedgEntry2.FindSet(); found; found = custLedgEntry2.Next() {
				ok, err := calcFinChrg2(custLedgEntry2, custLedgEntry2.Closed_by_amount.Neg())
				if err != nil {
					return types.Decimal{}, types.Decimal{}, err
				}
				calcInterest = calcInterest || ok
			}
		}
		if calcInterest {
			amount = amount.Div(types.NewDecimalFromInt(int64(finChrgTerms.Interest_period_days))).Mul(interestRate).Div(types.NewDecimalFromInt(100))
		} else {
			amount = types.Decimal{}
		}
	case gtables.FinanceChargeTerms_Interest_calculation_method.BalanceDue:
		graceEnd, err := CalcDateFormula(finChrgTerms.Grace_period, dueDate)
		if err != nil {
			return types.Decimal{}, types.Decimal{}, err
		}
		if graceEnd.Before(documentDate) {
			amount = remainingAmount.Mul(interestRate).Div(types.NewDecimalFromInt(100))
		}
	}
	return amount, interestRate, nil
}

// lastInterestDate: the document date of the last reminder that posted interest on the entry or
// of its last finance charge memo, whichever is later (blank: none)
func lastInterestDate(src ftables.Source, entryNo int) types.Date {
	var interestCalcDate types.Date
	reminderEntry := gtables.NewReminderFinChargeEntry(src)
	reminderEntry.SetCurrentKey("customer_entry_no", "type")
	reminderEntry.SetRange("customer_entry_no", entryNo)
	reminderEntry.SetRange("type", gtables.ReminderFinChargeEntry_Type.Reminder)
	reminderEntry.SetRange("interest_posted", true)
	if reminderEntry.FindLast() {
		interestCalcDate = reminderEntry.Document_date
	}
	reminderEntry.SetRange("type", gtables.ReminderFinChargeEntry_Type.FinanceChargeMemo)
	reminderEntry.SetRange("interest_posted")
	if reminderEntry.FindLast() && reminderEntry.Document_date.After(interestCalcDate) {
		interestCalcDate = reminderEntry.Document_date
	}
	return interestCalcDate
}
