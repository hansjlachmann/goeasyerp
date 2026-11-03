package tables

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/noseries"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/format"
	"github.com/hansjlachmann/openerp/backend/foundation/functions"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// reminderHeaderLogic is the business logic of table 295 Reminder Header: a reminder to a
// customer of the overdue entries, with interest (Finance Charge Terms) and the reminder level's
// additional fee; codeunit 393 issues it.
//
// Not migrated: Currency for Reminder Level (329) — an additional fee in another currency is the
// level's fee converted; extended texts; the empty issued reminder that fills the issuing series'
// gap when a numbered reminder is deleted (ConfirmDeletion, DeleteHeader).
type reminderHeaderLogic struct{}

func init() {
	gtables.RegisterReminderHeaderLogic(reminderHeaderLogic{})
	functions.Register("table", gtables.ReminderHeaderTableName, "AssistEdit", functions.ForAssistEdit(AssistEditReminderHeader), functions.WithAssistEdit())
}

// OnInsert: the number from the Sales & Receivables Setup's Reminder Nos., the issuing series,
// the dates; a single customer in the filter (a new reminder from the customer) is taken
func (reminderHeaderLogic) OnInsert(rec *gtables.ReminderHeader) error {
	salesSetup := gtables.GetSalesReceivablesSetup(rec)
	noSeriesMgt := noseries.New(rec)
	if rec.No == "" {
		err := salesSetup.TestField("reminder_nos")
		if err == nil {
			err = salesSetup.TestField("issued_reminder_nos")
		}
		if err != nil {
			return err
		}
		xRec := rec.XRec()
		err = noSeriesMgt.InitSeries(salesSetup.Reminder_nos, xRec.No_series, rec.Posting_date, &rec.No, &rec.No_series)
		if err != nil {
			return err
		}
	}
	setReminderPostingDescription(rec)
	if rec.No_series != "" && salesSetup.Reminder_nos == salesSetup.Issued_reminder_nos {
		rec.Issuing_no_series = rec.No_series
	} else {
		err := noSeriesMgt.SetDefaultSeries(&rec.Issuing_no_series, salesSetup.Issued_reminder_nos)
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

// OnDelete: the reminder's lines and comments
func (reminderHeaderLogic) OnDelete(rec *gtables.ReminderHeader) error {
	reminderLine := gtables.NewReminderLine(rec)
	reminderLine.SetRange("reminder_no", rec.No)
	reminderLine.DeleteAll()
	reminderCommentLine := gtables.NewReminderCommentLine(rec)
	reminderCommentLine.SetRange("type", gtables.ReminderCommentLine_Type.Reminder)
	reminderCommentLine.SetRange("no", rec.No)
	reminderCommentLine.DeleteAll()
	return nil
}

// OnRename: a reminder is never renamed
func (reminderHeaderLogic) OnRename(rec *gtables.ReminderHeader) error {
	return apperrors.RenameNotAllowed(gtables.ReminderHeaderTableName)
}

// setReminderPostingDescription: "Reminder <No.>"
func setReminderPostingDescription(rec *gtables.ReminderHeader) {
	description := system.StrSubstNo(i18n.GetInstance().Message("REMINDER_POSTING_DESCRIPTION", fcodeunits.CurrentLanguage()), rec.No)
	rec.Posting_description = types.Text(system.CopyStr(description, 1, rec.MaxStrLen("posting_description")))
}

// OnValidate_No: a number typed by hand needs Manual Nos.
func (reminderHeaderLogic) OnValidate_No(rec *gtables.ReminderHeader) error {
	xRec := rec.XRec()
	if rec.No != xRec.No {
		salesSetup := gtables.GetSalesReceivablesSetup(rec)
		noSeriesMgt := noseries.New(rec)
		series, issued := noSeriesMgt.IssuedBy(salesSetup.Reminder_nos, rec.No)
		if issued && xRec.No == "" {
			rec.No_series = series
		} else {
			err := noSeriesMgt.TestManual(salesSetup.Reminder_nos)
			if err != nil {
				return err
			}
			rec.No_series = ""
		}
	}
	setReminderPostingDescription(rec)
	return nil
}

// undoReminderLines (Undo): a changed customer, currency, document date or terms deletes the
// reminder's lines (the classic asks first — a table trigger cannot: treated as Yes)
func undoReminderLines(rec *gtables.ReminderHeader) {
	reminderLine := gtables.NewReminderLine(rec)
	reminderLine.SetRange("reminder_no", rec.No)
	reminderLine.DeleteAll()
}

// OnValidate_Customer_no: the customer's address, currency, dimensions, posting groups, tax and
// reminder and finance charge terms
func (reminderHeaderLogic) OnValidate_Customer_no(rec *gtables.ReminderHeader) error {
	xRec := rec.XRec()
	if rec.Customer_no != xRec.Customer_no {
		undoReminderLines(rec)
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
	rec.Dimension_1_code = cust.Dimension_1_code
	rec.Dimension_2_code = cust.Dimension_2_code
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
	rec.Reminder_terms_code = cust.Reminder_terms_code
	rec.Fin_charge_terms_code = cust.Fin_charge_terms_code
	return rec.Validate("reminder_terms_code", rec.Reminder_terms_code)
}

// OnValidate_Post_code: the post code's city
func (reminderHeaderLogic) OnValidate_Post_code(rec *gtables.ReminderHeader) error {
	postCode := gtables.NewPostCode(rec)
	if postCode.Get(rec.Post_code) {
		rec.City = postCode.City
	}
	return nil
}

// OnValidate_Currency_code: the lines are deleted
func (reminderHeaderLogic) OnValidate_Currency_code(rec *gtables.ReminderHeader) error {
	xRec := rec.XRec()
	if rec.Currency_code != xRec.Currency_code {
		undoReminderLines(rec)
	}
	return nil
}

// OnValidate_Document_date: the lines are deleted; the due date by the reminder level
func (reminderHeaderLogic) OnValidate_Document_date(rec *gtables.ReminderHeader) error {
	xRec := rec.XRec()
	if !rec.Document_date.Equal(xRec.Document_date) {
		undoReminderLines(rec)
	}
	return rec.Validate("reminder_level", rec.Reminder_level)
}

// OnValidate_Reminder_terms_code: the terms' posting of interest and fees, the due date
func (reminderHeaderLogic) OnValidate_Reminder_terms_code(rec *gtables.ReminderHeader) error {
	xRec := rec.XRec()
	if rec.Reminder_terms_code != xRec.Reminder_terms_code && xRec.Customer_no == rec.Customer_no {
		undoReminderLines(rec)
	}
	if rec.Reminder_terms_code != "" {
		reminderTerms := gtables.NewReminderTerms(rec)
		err := reminderTerms.GetOrError(rec.Reminder_terms_code)
		if err != nil {
			return err
		}
		rec.Post_interest = reminderTerms.Post_interest
		rec.Post_additional_fee = reminderTerms.Post_additional_fee
		return rec.Validate("reminder_level", rec.Reminder_level)
	}
	return nil
}

// OnValidate_Fin_charge_terms_code: the lines are deleted
func (reminderHeaderLogic) OnValidate_Fin_charge_terms_code(rec *gtables.ReminderHeader) error {
	xRec := rec.XRec()
	if rec.Fin_charge_terms_code != xRec.Fin_charge_terms_code {
		undoReminderLines(rec)
	}
	return nil
}

// OnValidate_Reminder_level: the due date by the level's (or the last level's) Due Date
// Calculation
func (reminderHeaderLogic) OnValidate_Reminder_level(rec *gtables.ReminderHeader) error {
	if rec.Reminder_level != 0 && rec.Reminder_terms_code != "" {
		reminderTerms := gtables.NewReminderTerms(rec)
		err := reminderTerms.GetOrError(rec.Reminder_terms_code)
		if err != nil {
			return err
		}
		reminderLevel := gtables.NewReminderLevel(rec)
		reminderLevel.SetRange("reminder_terms_code", rec.Reminder_terms_code)
		reminderLevel.SetRange("no", 1, rec.Reminder_level)
		if reminderLevel.FindLast() && !rec.Document_date.IsZero() {
			rec.Due_date, err = CalcDateFormula(reminderLevel.Due_date_calculation, rec.Document_date)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// OnValidate_Issuing_no_series: a series of the setup's Issued Reminder Nos.; not after a number
// was taken
func (reminderHeaderLogic) OnValidate_Issuing_no_series(rec *gtables.ReminderHeader) error {
	if rec.Issuing_no_series != "" {
		salesSetup := gtables.GetSalesReceivablesSetup(rec)
		err := salesSetup.TestField("reminder_nos")
		if err == nil {
			err = salesSetup.TestField("issued_reminder_nos")
		}
		if err == nil {
			err = noseries.New(rec).TestSeries(salesSetup.Issued_reminder_nos, rec.Issuing_no_series)
		}
		if err != nil {
			return err
		}
	}
	return rec.TestField("issuing_no", types.Code(""))
}

// AssistEditReminderHeader (AssistEdit): No. from the Reminder Nos. or a series related to it
func AssistEditReminderHeader(rec *gtables.ReminderHeader) (bool, error) {
	salesSetup := gtables.GetSalesReceivablesSetup(rec)
	err := salesSetup.TestField("reminder_nos")
	if err == nil {
		err = salesSetup.TestField("issued_reminder_nos")
	}
	if err != nil {
		return false, err
	}
	noSeriesMgt := noseries.New(rec)
	xRec := rec.XRec()
	noSeries := rec.No_series
	picked, err := noSeriesMgt.SelectSeries(salesSetup.Reminder_nos, xRec.No_series, &noSeries)
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

// reminderLineInserter: the text and fee lines InsertLines/UpdateLines add (classic globals)
type reminderLineInserter struct {
	rec           *gtables.ReminderHeader
	reminderLevel *gtables.ReminderLevel
	reminderText  *gtables.ReminderText
	nextLineNo    int
	lineSpacing   int
}

// InsertReminderLines (InsertLines): the level's beginning texts, the additional fee line (the
// customer posting group's Additional Fee Account) and the ending texts
func InsertReminderLines(rec *gtables.ReminderHeader) error {
	err := insertReminderBeginText(rec)
	if err != nil {
		return err
	}
	reminderLevel := gtables.NewReminderLevel(rec)
	reminderLevel.SetRange("reminder_terms_code", rec.Reminder_terms_code)
	reminderLevel.SetRange("no", 1, rec.Reminder_level)
	if reminderLevel.FindLast() && !reminderLevel.Additional_fee_lcy.IsZero() {
		ins := &reminderLineInserter{rec: rec, lineSpacing: 10000}
		last := gtables.NewReminderLine(rec)
		last.SetRange("reminder_no", rec.No)
		if last.FindLast() {
			ins.nextLineNo = last.Line_no
		}
		err = ins.insertBlankLine()
		if err != nil {
			return err
		}
		ins.nextLineNo += 10000
		reminderLine := gtables.NewReminderLine(rec)
		reminderLine.Init()
		reminderLine.Reminder_no = rec.No
		reminderLine.Line_no = ins.nextLineNo
		reminderLine.Type = gtables.ReminderLine_Type.GLAccount
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
			err = reminderLine.Validate("no", custPostingGroup.Additional_fee_account)
		}
		if err != nil {
			return err
		}
		lang := fcodeunits.CurrentLanguage()
		fee := reminderLevel.Additional_fee_lcy
		if rec.Currency_code == "" {
			reminderLine.Description = types.Text(i18n.GetInstance().FieldCaption(gtables.ReminderLevelTableName, "additional_fee_lcy", lang))
		} else {
			currExchRate := ExchangeRates(rec)
			rate, err := currExchRate.ExchangeRate(rec.Posting_date, rec.Currency_code)
			if err == nil {
				fee, err = currExchRate.ExchangeAmtLCYToFCY(rec.Posting_date, rec.Currency_code, reminderLevel.Additional_fee_lcy, rate)
			}
			if err != nil {
				return err
			}
			reminderLine.Description = types.Text(system.StrSubstNo("%1 (%2)", i18n.GetInstance().Message("REMINDER_ADDITIONAL_FEE", lang), rec.Currency_code))
		}
		err = reminderLine.Validate("amount", fee)
		if err == nil {
			err = reminderLine.InsertOrError(false)
		}
		if err != nil {
			return err
		}
	}
	err = insertReminderEndText(rec)
	if err != nil {
		return err
	}
	return rec.ModifyOrError(false)
}

// UpdateReminderLines (UpdateLines): the text lines at the beginning and the end again (report 187
// Update Reminder Text)
func UpdateReminderLines(rec *gtables.ReminderHeader) error {
	reminderLine := gtables.NewReminderLine(rec)
	reminderLine.SetRange("reminder_no", rec.No)
	for ok := reminderLine.FindFirst(); ok; {
		ok = reminderLine.Type == gtables.ReminderLine_Type.Blank && reminderLine.Attached_to_line_no == 0
		if ok {
			err := reminderLine.DeleteOrError(true)
			if err != nil {
				return err
			}
			ok = reminderLine.FindFirst()
		}
	}
	for ok := reminderLine.FindLast(); ok; {
		ok = reminderLine.Type == gtables.ReminderLine_Type.Blank && reminderLine.Attached_to_line_no == 0
		if ok {
			err := reminderLine.DeleteOrError(true)
			if err != nil {
				return err
			}
			ok = reminderLine.FindLast()
		}
	}
	err := insertReminderBeginText(rec)
	if err == nil {
		err = insertReminderEndText(rec)
	}
	if err != nil {
		return err
	}
	return rec.ModifyOrError(false)
}

// insertReminderBeginText (InsertBeginText): the beginning texts before the first line
func insertReminderBeginText(rec *gtables.ReminderHeader) error {
	reminderLevel := gtables.NewReminderLevel(rec)
	reminderLevel.SetRange("reminder_terms_code", rec.Reminder_terms_code)
	reminderLevel.SetRange("no", 1, rec.Reminder_level)
	if reminderLevel.FindLast() {
		reminderText := gtables.NewReminderText(rec)
		reminderText.SetRange("reminder_terms_code", rec.Reminder_terms_code)
		reminderText.SetRange("reminder_level", reminderLevel.No)
		reminderText.SetRange("position", gtables.ReminderText_Position.Beginning)
		ins := &reminderLineInserter{rec: rec, reminderLevel: reminderLevel, reminderText: reminderText}
		reminderLine := gtables.NewReminderLine(rec)
		reminderLine.SetRange("reminder_no", rec.No)
		if reminderLine.FindFirst() {
			ins.lineSpacing = reminderLine.Line_no / (reminderText.Count() + 2)
			if ins.lineSpacing == 0 {
				return apperrors.ReminderNoSpaceForText()
			}
		} else {
			ins.lineSpacing = 10000
		}
		ins.nextLineNo = 0
		return ins.insertTextLines()
	}
	return nil
}

// insertReminderEndText (InsertEndText): the ending texts after the last line
func insertReminderEndText(rec *gtables.ReminderHeader) error {
	reminderLevel := gtables.NewReminderLevel(rec)
	reminderLevel.SetRange("reminder_terms_code", rec.Reminder_terms_code)
	reminderLevel.SetRange("no", 1, rec.Reminder_level)
	if reminderLevel.FindLast() {
		reminderText := gtables.NewReminderText(rec)
		reminderText.SetRange("reminder_terms_code", rec.Reminder_terms_code)
		reminderText.SetRange("reminder_level", reminderLevel.No)
		reminderText.SetRange("position", gtables.ReminderText_Position.Ending)
		ins := &reminderLineInserter{rec: rec, reminderLevel: reminderLevel, reminderText: reminderText, lineSpacing: 10000}
		reminderLine := gtables.NewReminderLine(rec)
		reminderLine.SetRange("reminder_no", rec.No)
		if reminderLine.FindLast() {
			ins.nextLineNo = reminderLine.Line_no
		}
		return ins.insertTextLines()
	}
	return nil
}

// insertTextLines (InsertTextLines): the texts with their placeholders filled — %1 document
// date, %2 due date, %3 interest rate, %4 remaining amount, %5 interest, %6 additional fee, %7
// total, %8 reminder level, %9 currency code
func (ins *reminderLineInserter) insertTextLines() error {
	rec := ins.rec
	reminderText := ins.reminderText
	if !reminderText.FindFirst() {
		return nil
	}
	if reminderText.Position == gtables.ReminderText_Position.Ending {
		err := ins.insertBlankLine()
		if err != nil {
			return err
		}
	}
	finChrgTerms := gtables.NewFinanceChargeTerms(rec)
	if rec.Fin_charge_terms_code != "" {
		err := finChrgTerms.GetOrError(rec.Fin_charge_terms_code)
		if err != nil {
			return err
		}
	}
	if !ins.reminderLevel.Calculate_interest {
		finChrgTerms.Interest_rate = types.Decimal{}
	}
	rec.CalcFields("remaining_amount", "interest_amount", "additional_fee", "tax_amount")
	reminderTotal := rec.Remaining_amount.Add(rec.Interest_amount).Add(rec.Additional_fee).Add(rec.Tax_amount)
	lang := fcodeunits.CurrentLanguage()
	for found := reminderText.FindSet(); found; found = reminderText.Next() {
		ins.nextLineNo += ins.lineSpacing
		reminderLine := gtables.NewReminderLine(rec)
		reminderLine.Init()
		reminderLine.Reminder_no = rec.No
		reminderLine.Line_no = ins.nextLineNo
		reminderLine.Type = gtables.ReminderLine_Type.Blank
		reminderLine.Description = types.Text(system.CopyStr(system.StrSubstNo(reminderText.Text.String(),
			formatTextDate(rec.Document_date, lang), formatTextDate(rec.Due_date, lang), finChrgTerms.Interest_rate,
			formatTextAmount(rec.Remaining_amount, lang), formatTextAmount(rec.Interest_amount, lang), formatTextAmount(rec.Additional_fee, lang),
			formatTextAmount(reminderTotal, lang), rec.Reminder_level, rec.Currency_code), 1, reminderLine.MaxStrLen("description")))
		err := reminderLine.InsertOrError(false)
		if err != nil {
			return err
		}
	}
	if reminderText.Position == gtables.ReminderText_Position.Beginning {
		return ins.insertBlankLine()
	}
	return nil
}

// insertBlankLine (InsertBlankLine)
func (ins *reminderLineInserter) insertBlankLine() error {
	ins.nextLineNo += ins.lineSpacing
	reminderLine := gtables.NewReminderLine(ins.rec)
	reminderLine.Init()
	reminderLine.Reminder_no = ins.rec.No
	reminderLine.Line_no = ins.nextLineNo
	return reminderLine.InsertOrError(false)
}

// formatTextDate / formatTextAmount: a date or amount filled into a reminder or finance charge
// text, in the user's format
func formatTextDate(d types.Date, lang string) string {
	return format.Date(d, lang)
}

func formatTextAmount(d types.Decimal, lang string) string {
	return format.Decimal(d, "2:2", lang)
}
