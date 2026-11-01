package tables

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/noseries"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/functions"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// financeChargeMemoHeaderLogic is the business logic of table 302 Finance Charge Memo Header:
// the interest of a customer's late payments and the terms' additional fee; codeunit 395 issues
// it.
//
// Not migrated: Currency for Fin. Charge Terms (328) — a fee in another currency is the terms'
// fee converted; extended texts; the empty issued memo that fills the issuing series' gap.
type financeChargeMemoHeaderLogic struct{}

func init() {
	gtables.RegisterFinanceChargeMemoHeaderLogic(financeChargeMemoHeaderLogic{})
	functions.Register("table", gtables.FinanceChargeMemoHeaderTableName, "AssistEdit", functions.ForAssistEdit(AssistEditFinChrgMemoHeader), functions.WithAssistEdit())
}

// OnInsert: the number from the setup's Fin. Chrg. Memo Nos., the issuing series, the dates; a
// single customer in the filter is taken
func (financeChargeMemoHeaderLogic) OnInsert(rec *gtables.FinanceChargeMemoHeader) error {
	salesSetup := gtables.GetSalesReceivablesSetup(rec)
	noSeriesMgt := noseries.New(rec)
	if rec.No == "" {
		err := salesSetup.TestField("fin_chrg_memo_nos")
		if err == nil {
			err = salesSetup.TestField("issued_fin_chrg_m_nos")
		}
		if err != nil {
			return err
		}
		xRec := rec.XRec()
		err = noSeriesMgt.InitSeries(salesSetup.Fin_chrg_memo_nos, xRec.No_series, rec.Posting_date, &rec.No, &rec.No_series)
		if err != nil {
			return err
		}
	}
	setFinChrgMemoPostingDescription(rec)
	if rec.No_series != "" && salesSetup.Fin_chrg_memo_nos == salesSetup.Issued_fin_chrg_m_nos {
		rec.Issuing_no_series = rec.No_series
	} else {
		err := noSeriesMgt.SetDefaultSeries(&rec.Issuing_no_series, salesSetup.Issued_fin_chrg_m_nos)
		if err != nil {
			return err
		}
	}
	if rec.Posting_date.IsZero() {
		rec.Posting_date = system.WorkDate()
	}
	rec.Document_date = system.WorkDate()
	rec.Due_date = system.WorkDate()
	if rec.GetFilter("customer_no") != "" {
		min, _ := rec.GetRangeMin("customer_no").(types.Code)
		max, _ := rec.GetRangeMax("customer_no").(types.Code)
		if min == max {
			return rec.Validate("customer_no", min)
		}
	}
	return nil
}

// OnDelete: the memo's lines and comments
func (financeChargeMemoHeaderLogic) OnDelete(rec *gtables.FinanceChargeMemoHeader) error {
	finChrgMemoLine := gtables.NewFinanceChargeMemoLine(rec)
	finChrgMemoLine.SetRange("finance_charge_memo_no", rec.No)
	finChrgMemoLine.DeleteAll()
	finChrgCommentLine := gtables.NewFinChargeCommentLine(rec)
	finChrgCommentLine.SetRange("type", gtables.FinChargeCommentLine_Type.FinanceChargeMemo)
	finChrgCommentLine.SetRange("no", rec.No)
	finChrgCommentLine.DeleteAll()
	return nil
}

// OnRename: never renamed
func (financeChargeMemoHeaderLogic) OnRename(rec *gtables.FinanceChargeMemoHeader) error {
	return apperrors.RenameNotAllowed(gtables.FinanceChargeMemoHeaderTableName)
}

// setFinChrgMemoPostingDescription: "Finance Charge Memo <No.>"
func setFinChrgMemoPostingDescription(rec *gtables.FinanceChargeMemoHeader) {
	description := system.StrSubstNo(i18n.GetInstance().Message("FIN_CHRG_MEMO_POSTING_DESCRIPTION", fcodeunits.CurrentLanguage()), rec.No)
	rec.Posting_description = types.Text(system.CopyStr(description, 1, rec.MaxStrLen("posting_description")))
}

// OnValidate_No: a number typed by hand needs Manual Nos.
func (financeChargeMemoHeaderLogic) OnValidate_No(rec *gtables.FinanceChargeMemoHeader) error {
	xRec := rec.XRec()
	if rec.No != xRec.No {
		salesSetup := gtables.GetSalesReceivablesSetup(rec)
		noSeriesMgt := noseries.New(rec)
		series, issued := noSeriesMgt.IssuedBy(salesSetup.Fin_chrg_memo_nos, rec.No)
		if issued && xRec.No == "" {
			rec.No_series = series
		} else {
			err := noSeriesMgt.TestManual(salesSetup.Fin_chrg_memo_nos)
			if err != nil {
				return err
			}
			rec.No_series = ""
		}
	}
	setFinChrgMemoPostingDescription(rec)
	return nil
}

// undoFinChrgMemoLines (Undo): a changed customer, currency, document date or terms deletes the
// lines (the classic asks first — treated as Yes)
func undoFinChrgMemoLines(rec *gtables.FinanceChargeMemoHeader) {
	finChrgMemoLine := gtables.NewFinanceChargeMemoLine(rec)
	finChrgMemoLine.SetRange("finance_charge_memo_no", rec.No)
	finChrgMemoLine.DeleteAll()
}

// OnValidate_Customer_no: the customer's address, currency, dimensions, posting groups, tax and
// finance charge terms
func (financeChargeMemoHeaderLogic) OnValidate_Customer_no(rec *gtables.FinanceChargeMemoHeader) error {
	xRec := rec.XRec()
	if rec.Customer_no != xRec.Customer_no {
		undoFinChrgMemoLines(rec)
	}
	if rec.Customer_no == "" {
		return nil
	}
	cust := gtables.NewCustomer(rec)
	err := cust.GetOrError(rec.Customer_no)
	if err != nil {
		return err
	}
	rec.Name = cust.Name
	rec.Name_2 = cust.Name_2
	rec.Address = cust.Address
	rec.Address_2 = cust.Address_2
	rec.Post_code = cust.Post_code
	rec.City = cust.City
	rec.State = cust.State
	rec.Contact = cust.Contact
	rec.Country_code = cust.Country_code
	rec.Language_code = cust.Language_code
	rec.Currency_code = cust.Currency_code
	rec.Department_code = cust.Department_code
	rec.Project_code = cust.Project_code
	rec.Vat_registration_no = cust.Vat_registration_no
	err = cust.TestField("customer_posting_group")
	if err != nil {
		return err
	}
	rec.Customer_posting_group = cust.Customer_posting_group
	rec.Gen_bus_posting_group = cust.Gen_bus_posting_group
	rec.Tax_bus_posting_group = cust.Tax_bus_posting_group
	rec.Tax_area_code = cust.Tax_area_code
	rec.Tax_liable = cust.Tax_liable
	rec.Fin_charge_terms_code = cust.Fin_charge_terms_code
	return rec.Validate("fin_charge_terms_code", rec.Fin_charge_terms_code)
}

// OnValidate_Post_code: the post code's city
func (financeChargeMemoHeaderLogic) OnValidate_Post_code(rec *gtables.FinanceChargeMemoHeader) error {
	postCode := gtables.NewPostCode(rec)
	if postCode.Get(rec.Post_code) {
		rec.City = postCode.City
	}
	return nil
}

// OnValidate_Currency_code: the lines are deleted
func (financeChargeMemoHeaderLogic) OnValidate_Currency_code(rec *gtables.FinanceChargeMemoHeader) error {
	xRec := rec.XRec()
	if rec.Currency_code != xRec.Currency_code {
		undoFinChrgMemoLines(rec)
	}
	return nil
}

// OnValidate_Document_date: the lines are deleted; the due date by the terms
func (financeChargeMemoHeaderLogic) OnValidate_Document_date(rec *gtables.FinanceChargeMemoHeader) error {
	xRec := rec.XRec()
	if !rec.Document_date.Equal(xRec.Document_date) {
		undoFinChrgMemoLines(rec)
	}
	return rec.Validate("fin_charge_terms_code", rec.Fin_charge_terms_code)
}

// OnValidate_Fin_charge_terms_code: the terms' posting of interest and fees, the due date by the
// terms' Due Date Calculation
func (financeChargeMemoHeaderLogic) OnValidate_Fin_charge_terms_code(rec *gtables.FinanceChargeMemoHeader) error {
	xRec := rec.XRec()
	if rec.Fin_charge_terms_code != xRec.Fin_charge_terms_code && xRec.Customer_no == rec.Customer_no {
		undoFinChrgMemoLines(rec)
	}
	if rec.Fin_charge_terms_code != "" {
		finChrgTerms := gtables.NewFinanceChargeTerms(rec)
		err := finChrgTerms.GetOrError(rec.Fin_charge_terms_code)
		if err != nil {
			return err
		}
		rec.Post_interest = finChrgTerms.Post_interest
		rec.Post_additional_fee = finChrgTerms.Post_additional_fee
		if !rec.Document_date.IsZero() {
			rec.Due_date, err = CalcDateFormula(finChrgTerms.Due_date_calculation, rec.Document_date)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// OnValidate_Issuing_no_series: a series of the setup's Issued Fin. Chrg. M. Nos.
func (financeChargeMemoHeaderLogic) OnValidate_Issuing_no_series(rec *gtables.FinanceChargeMemoHeader) error {
	if rec.Issuing_no_series != "" {
		salesSetup := gtables.GetSalesReceivablesSetup(rec)
		err := salesSetup.TestField("fin_chrg_memo_nos")
		if err == nil {
			err = salesSetup.TestField("issued_fin_chrg_m_nos")
		}
		if err == nil {
			err = noseries.New(rec).TestSeries(salesSetup.Issued_fin_chrg_m_nos, rec.Issuing_no_series)
		}
		if err != nil {
			return err
		}
	}
	return rec.TestField("issuing_no", types.Code(""))
}

// AssistEditFinChrgMemoHeader (AssistEdit): No. from the Fin. Chrg. Memo Nos. or a related series
func AssistEditFinChrgMemoHeader(rec *gtables.FinanceChargeMemoHeader) (bool, error) {
	salesSetup := gtables.GetSalesReceivablesSetup(rec)
	err := salesSetup.TestField("fin_chrg_memo_nos")
	if err == nil {
		err = salesSetup.TestField("issued_fin_chrg_m_nos")
	}
	if err != nil {
		return false, err
	}
	noSeriesMgt := noseries.New(rec)
	xRec := rec.XRec()
	noSeries := rec.No_series
	picked, err := noSeriesMgt.SelectSeries(salesSetup.Fin_chrg_memo_nos, xRec.No_series, &noSeries)
	if err != nil || !picked {
		return false, err
	}
	err = noSeriesMgt.SetSeries(&rec.No)
	if err != nil {
		return false, err
	}
	rec.No_series = noSeries
	return true, nil
}

// finChrgMemoLineInserter: the text and fee lines InsertLines/UpdateLines add (classic globals)
type finChrgMemoLineInserter struct {
	rec         *gtables.FinanceChargeMemoHeader
	finChrgText *gtables.FinanceChargeText
	nextLineNo  int
	lineSpacing int
}

// InsertFinChrgMemoLines (InsertLines): the terms' beginning texts, the additional fee line and
// the ending texts
func InsertFinChrgMemoLines(rec *gtables.FinanceChargeMemoHeader) error {
	err := rec.TestField("fin_charge_terms_code")
	if err != nil {
		return err
	}
	finChrgTerms := gtables.NewFinanceChargeTerms(rec)
	err = finChrgTerms.GetOrError(rec.Fin_charge_terms_code)
	if err != nil {
		return err
	}
	err = insertFinChrgMemoText(rec, gtables.FinanceChargeText_Position.Beginning)
	if err != nil {
		return err
	}
	if !finChrgTerms.Additional_fee_lcy.IsZero() {
		ins := &finChrgMemoLineInserter{rec: rec}
		last := gtables.NewFinanceChargeMemoLine(rec)
		last.SetRange("finance_charge_memo_no", rec.No)
		if last.FindLast() {
			ins.nextLineNo = last.Line_no
		}
		if last.Type != gtables.FinanceChargeMemoLine_Type.Blank || last.Description != "" {
			ins.lineSpacing = 10000
			err = ins.insertBlankLine()
			if err != nil {
				return err
			}
		}
		ins.nextLineNo += 10000
		finChrgMemoLine := gtables.NewFinanceChargeMemoLine(rec)
		finChrgMemoLine.Init()
		finChrgMemoLine.Finance_charge_memo_no = rec.No
		finChrgMemoLine.Line_no = ins.nextLineNo
		finChrgMemoLine.Type = gtables.FinanceChargeMemoLine_Type.GLAccount
		err = rec.TestField("customer_posting_group")
		if err != nil {
			return err
		}
		custPostingGroup := gtables.NewCustomerPostingGroup(rec)
		err = custPostingGroup.GetOrError(rec.Customer_posting_group)
		if err == nil {
			err = custPostingGroup.TestField("additional_fee_account")
		}
		if err == nil {
			err = finChrgMemoLine.Validate("no", custPostingGroup.Additional_fee_account)
		}
		if err != nil {
			return err
		}
		lang := fcodeunits.CurrentLanguage()
		fee := finChrgTerms.Additional_fee_lcy
		if rec.Currency_code == "" {
			finChrgMemoLine.Description = types.Text(i18n.GetInstance().FieldCaption(gtables.FinanceChargeTermsTableName, "additional_fee_lcy", lang))
		} else {
			currExchRate := ExchangeRates(rec)
			rate, err := currExchRate.ExchangeRate(rec.Posting_date, rec.Currency_code)
			if err == nil {
				fee, err = currExchRate.ExchangeAmtLCYToFCY(rec.Posting_date, rec.Currency_code, finChrgTerms.Additional_fee_lcy, rate)
			}
			if err != nil {
				return err
			}
			finChrgMemoLine.Description = types.Text(system.StrSubstNo("%1 (%2)", i18n.GetInstance().Message("REMINDER_ADDITIONAL_FEE", lang), rec.Currency_code))
		}
		err = finChrgMemoLine.Validate("amount", fee)
		if err == nil {
			err = finChrgMemoLine.InsertOrError(false)
		}
		if err != nil {
			return err
		}
	}
	err = insertFinChrgMemoText(rec, gtables.FinanceChargeText_Position.Ending)
	if err != nil {
		return err
	}
	return rec.ModifyOrError(false)
}

// UpdateFinChrgMemoLines (UpdateLines): the text lines at the beginning and the end again (report
// 186 Update Finance Charge Text)
func UpdateFinChrgMemoLines(rec *gtables.FinanceChargeMemoHeader) error {
	finChrgMemoLine := gtables.NewFinanceChargeMemoLine(rec)
	finChrgMemoLine.SetRange("finance_charge_memo_no", rec.No)
	for ok := finChrgMemoLine.FindFirst(); ok; {
		ok = finChrgMemoLine.Type == gtables.FinanceChargeMemoLine_Type.Blank && finChrgMemoLine.Attached_to_line_no == 0
		if ok {
			err := finChrgMemoLine.DeleteOrError(true)
			if err != nil {
				return err
			}
			ok = finChrgMemoLine.FindFirst()
		}
	}
	for ok := finChrgMemoLine.FindLast(); ok; {
		ok = finChrgMemoLine.Type == gtables.FinanceChargeMemoLine_Type.Blank && finChrgMemoLine.Attached_to_line_no == 0
		if ok {
			err := finChrgMemoLine.DeleteOrError(true)
			if err != nil {
				return err
			}
			ok = finChrgMemoLine.FindLast()
		}
	}
	err := insertFinChrgMemoText(rec, gtables.FinanceChargeText_Position.Beginning)
	if err == nil {
		err = insertFinChrgMemoText(rec, gtables.FinanceChargeText_Position.Ending)
	}
	return err
}

// insertFinChrgMemoText (InsertBeginText, InsertEndText): the terms' texts of a position
func insertFinChrgMemoText(rec *gtables.FinanceChargeMemoHeader, position gtables.FinanceChargeTextPosition) error {
	finChrgText := gtables.NewFinanceChargeText(rec)
	finChrgText.SetRange("fin_charge_terms_code", rec.Fin_charge_terms_code)
	finChrgText.SetRange("position", position)
	ins := &finChrgMemoLineInserter{rec: rec, finChrgText: finChrgText, lineSpacing: 10000}
	finChrgMemoLine := gtables.NewFinanceChargeMemoLine(rec)
	finChrgMemoLine.SetRange("finance_charge_memo_no", rec.No)
	if position == gtables.FinanceChargeText_Position.Beginning {
		if finChrgMemoLine.FindFirst() {
			ins.lineSpacing = finChrgMemoLine.Line_no / (finChrgText.Count() + 2)
			if ins.lineSpacing == 0 {
				return apperrors.ReminderNoSpaceForText()
			}
		}
		ins.nextLineNo = 0
	} else {
		if finChrgMemoLine.FindLast() {
			ins.nextLineNo = finChrgMemoLine.Line_no
		}
	}
	return ins.insertTextLines()
}

// insertTextLines (InsertTextLines): the texts with their placeholders filled — %1 document
// date, %2 due date, %3 interest rate, %4 remaining amount, %5 interest, %6 additional fee, %7
// total, %9 currency code
func (ins *finChrgMemoLineInserter) insertTextLines() error {
	rec := ins.rec
	finChrgText := ins.finChrgText
	if !finChrgText.FindFirst() {
		return nil
	}
	if finChrgText.Position == gtables.FinanceChargeText_Position.Ending {
		err := ins.insertBlankLine()
		if err != nil {
			return err
		}
	}
	finChrgTerms := gtables.NewFinanceChargeTerms(rec)
	if !finChrgTerms.Get(rec.Fin_charge_terms_code) {
		finChrgTerms.Init()
	}
	rec.CalcFields("remaining_amount", "interest_amount", "additional_fee", "tax_amount")
	finChrgMemoTotal := rec.Interest_amount.Add(rec.Additional_fee).Add(rec.Tax_amount).Add(rec.Remaining_amount)
	lang := fcodeunits.CurrentLanguage()
	for found := finChrgText.FindSet(); found; found = finChrgText.Next() {
		ins.nextLineNo += ins.lineSpacing
		finChrgMemoLine := gtables.NewFinanceChargeMemoLine(rec)
		finChrgMemoLine.Init()
		finChrgMemoLine.Finance_charge_memo_no = rec.No
		finChrgMemoLine.Line_no = ins.nextLineNo
		finChrgMemoLine.Description = types.Text(system.CopyStr(system.StrSubstNo(finChrgText.Text.String(),
			formatTextDate(rec.Document_date, lang), formatTextDate(rec.Due_date, lang), finChrgTerms.Interest_rate,
			formatTextAmount(rec.Remaining_amount, lang), formatTextAmount(rec.Interest_amount, lang), formatTextAmount(rec.Additional_fee, lang),
			formatTextAmount(finChrgMemoTotal, lang), "", rec.Currency_code), 1, finChrgMemoLine.MaxStrLen("description")))
		err := finChrgMemoLine.InsertOrError(false)
		if err != nil {
			return err
		}
	}
	if finChrgText.Position == gtables.FinanceChargeText_Position.Beginning {
		return ins.insertBlankLine()
	}
	return nil
}

// insertBlankLine (InsertBlankLine)
func (ins *finChrgMemoLineInserter) insertBlankLine() error {
	ins.nextLineNo += ins.lineSpacing
	finChrgMemoLine := gtables.NewFinanceChargeMemoLine(ins.rec)
	finChrgMemoLine.Init()
	finChrgMemoLine.Finance_charge_memo_no = ins.rec.No
	finChrgMemoLine.Line_no = ins.nextLineNo
	return finChrgMemoLine.InsertOrError(false)
}
