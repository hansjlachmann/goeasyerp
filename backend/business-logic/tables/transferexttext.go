package tables

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Codeunit 378 Transfer Extended Text: the extended text of a document line's standard text, G/L
// account, item or resource — the one valid on the document date in the document's language (or
// for all languages), for the document type — becomes text lines attached to the line (Attached
// to Line No.), replacing those it had. Insert Ext. Texts on the line subpages inserts it always
// (Unconditionally), entering No. only when the account, item or resource has Automatic Ext. Text
// (or for a standard text).

func init() {
	functions.Register("table", gtables.SalesLineTableName, "InsertExtendedText", functions.ForArgs(salesLineInsertExtendedText, "bool"))
	functions.Register("table", gtables.PurchaseLineTableName, "InsertExtendedText", functions.ForArgs(purchLineInsertExtendedText, "bool"))
	functions.Register("table", gtables.ReminderLineTableName, "InsertExtendedText", functions.ForArgs(reminderLineInsertExtendedText, "bool"))
	functions.Register("table", gtables.FinanceChargeMemoLineTableName, "InsertExtendedText", functions.ForArgs(finChrgMemoLineInsertExtendedText, "bool"))
}

// TransferExtendedText is the codeunit's state: the text lines read (TmpExtTextLine) and whether
// the page must show new lines (MakeUpdate)
type TransferExtendedText struct {
	tmpExtTextLines    []string
	makeUpdateRequired bool
}

// MakeUpdate (MakeUpdate): lines were deleted or inserted
func (t *TransferExtendedText) MakeUpdate() bool { return t.makeUpdateRequired }

// autoText: whether the account, item or resource inserts its text when entered
func autoText(src ftables.Source, tableName int, no types.Code) bool {
	switch tableName {
	case int(gtables.ExtendedTextHeader_Table_name.StandardText):
		return true
	case int(gtables.ExtendedTextHeader_Table_name.GLAccount):
		glAcc := gtables.NewGLAccount(src)
		return glAcc.Get(no) && glAcc.Automatic_ext_text
	case int(gtables.ExtendedTextHeader_Table_name.Item):
		item := gtables.NewItem(src)
		return item.Get(no) && item.Automatic_ext_text
	case int(gtables.ExtendedTextHeader_Table_name.Resource):
		res := gtables.NewResource(src)
		return res.Get(no) && res.Automatic_ext_text
	}
	return false
}

// readLines (ReadLines): the text valid on the document date — the latest text in the language,
// else (a language given) the latest for all languages — into the codeunit's lines
func (t *TransferExtendedText) readLines(extTextHeader *gtables.ExtendedTextHeader, docDate types.Date, languageCode types.Code) bool {
	extTextHeader.SetCurrentKey("table_name", "no", "language_code", "all_language_codes", "starting_date", "ending_date")
	extTextHeader.SetRange("starting_date", types.Date{}, docDate)
	extTextHeader.SetFilter("ending_date", docDate.String()+"..|''")
	if languageCode == "" {
		extTextHeader.SetRange("language_code", types.Code(""))
		if !extTextHeader.FindLast() {
			return false
		}
	} else {
		extTextHeader.SetRange("language_code", languageCode)
		if !extTextHeader.FindLast() {
			extTextHeader.SetRange("all_language_codes", true)
			extTextHeader.SetRange("language_code", types.Code(""))
			if !extTextHeader.FindLast() {
				return false
			}
		}
	}
	extTextLine := gtables.NewExtendedTextLine(extTextHeader)
	extTextLine.SetRange("table_name", gtables.ExtendedTextLineTable_name(extTextHeader.Table_name))
	extTextLine.SetRange("no", extTextHeader.No)
	extTextLine.SetRange("language_code", extTextHeader.Language_code)
	extTextLine.SetRange("text_no", extTextHeader.Text_no)
	t.tmpExtTextLines = nil
	for found := extTextLine.FindSetBuffered(); found; found = extTextLine.Next() {
		t.tmpExtTextLines = append(t.tmpExtTextLines, extTextLine.Text.String())
	}
	return len(t.tmpExtTextLines) > 0
}

// lineSpacing: the line numbers between a line and the next one for the text lines
func (t *TransferExtendedText) lineSpacing(lineNo int, nextLineNo int, hasNext bool) (int, error) {
	if !hasNext {
		return 10000, nil
	}
	spacing := (nextLineNo - lineNo) / (1 + len(t.tmpExtTextLines))
	if spacing == 0 {
		return 0, apperrors.ExtTextNoSpace()
	}
	return spacing, nil
}

// ---- Sales ----

// SalesCheckIfAnyExtText (SalesCheckIfAnyExtText): the line's attached text lines deleted, and
// whether there is a text to insert
func (t *TransferExtendedText) SalesCheckIfAnyExtText(salesLine *gtables.SalesLine, unconditionally bool) (bool, error) {
	t.makeUpdateRequired = false
	if salesLine.Line_no != 0 {
		t.makeUpdateRequired = deleteSalesLines(salesLine)
	}
	if !unconditionally && !autoText(salesLine, int(salesLine.Type), salesLine.No) {
		return false, nil
	}
	err := salesLine.TestField("document_no")
	if err != nil {
		return false, err
	}
	salesHeader := gtables.NewSalesHeader(salesLine)
	err = salesHeader.GetOrError(map[string]interface{}{"document_type": int(salesLine.Document_type), "no": salesLine.Document_no})
	if err != nil {
		return false, err
	}
	extTextHeader := gtables.NewExtendedTextHeader(salesLine)
	extTextHeader.SetRange("table_name", int(salesLine.Type))
	extTextHeader.SetRange("no", salesLine.No)
	switch salesLine.Document_type {
	case gtables.SalesLine_Document_type.Quote:
		extTextHeader.SetRange("sales_quote", true)
	case gtables.SalesLine_Document_type.Order:
		extTextHeader.SetRange("sales_order", true)
	case gtables.SalesLine_Document_type.Invoice:
		extTextHeader.SetRange("sales_invoice", true)
	case gtables.SalesLine_Document_type.CreditMemo:
		extTextHeader.SetRange("sales_credit_memo", true)
	}
	return t.readLines(extTextHeader, salesHeader.Document_date, salesHeader.Language_code), nil
}

// InsertSalesExtText (InsertSalesExtText): the text lines after the line, attached to it
func (t *TransferExtendedText) InsertSalesExtText(salesLine *gtables.SalesLine) error {
	toSalesLine := gtables.NewSalesLine(salesLine)
	toSalesLine.SetRange("document_type", salesLine.Document_type)
	toSalesLine.SetRange("document_no", salesLine.Document_no)
	toSalesLine.Document_type, toSalesLine.Document_no, toSalesLine.Line_no = salesLine.Document_type, salesLine.Document_no, salesLine.Line_no
	hasNext := toSalesLine.Find(">")
	lineSpacing, err := t.lineSpacing(salesLine.Line_no, toSalesLine.Line_no, hasNext)
	if err != nil {
		return err
	}
	nextLineNo := salesLine.Line_no + lineSpacing
	for _, text := range t.tmpExtTextLines {
		toSalesLine = gtables.NewSalesLine(salesLine)
		toSalesLine.Init()
		toSalesLine.Document_type = salesLine.Document_type
		toSalesLine.Document_no = salesLine.Document_no
		toSalesLine.Line_no = nextLineNo
		nextLineNo = nextLineNo + lineSpacing
		toSalesLine.Description = types.Text(text)
		toSalesLine.Attached_to_line_no = salesLine.Line_no
		err = toSalesLine.InsertOrError(false)
		if err != nil {
			return err
		}
	}
	if len(t.tmpExtTextLines) > 0 {
		t.makeUpdateRequired = true
	}
	t.tmpExtTextLines = nil
	return nil
}

// deleteSalesLines (DeleteSalesLines): the text lines attached to the line
func deleteSalesLines(salesLine *gtables.SalesLine) bool {
	salesLine2 := gtables.NewSalesLine(salesLine)
	salesLine2.SetRange("document_type", salesLine.Document_type)
	salesLine2.SetRange("document_no", salesLine.Document_no)
	salesLine2.SetRange("attached_to_line_no", salesLine.Line_no)
	if salesLine2.Count() == 0 {
		return false
	}
	salesLine2.DeleteAll()
	return true
}

// ---- Purchases ----

// PurchCheckIfAnyExtText (PurchCheckIfAnyExtText)
func (t *TransferExtendedText) PurchCheckIfAnyExtText(purchLine *gtables.PurchaseLine, unconditionally bool) (bool, error) {
	t.makeUpdateRequired = false
	if purchLine.Line_no != 0 {
		t.makeUpdateRequired = deletePurchLines(purchLine)
	}
	if purchLine.Type == gtables.PurchaseLine_Type.FixedAsset {
		return false, nil
	}
	if !unconditionally && !autoText(purchLine, int(purchLine.Type), purchLine.No) {
		return false, nil
	}
	err := purchLine.TestField("document_no")
	if err != nil {
		return false, err
	}
	purchHeader := gtables.NewPurchaseHeader(purchLine)
	err = purchHeader.GetOrError(map[string]interface{}{"document_type": int(purchLine.Document_type), "no": purchLine.Document_no})
	if err != nil {
		return false, err
	}
	extTextHeader := gtables.NewExtendedTextHeader(purchLine)
	extTextHeader.SetRange("table_name", int(purchLine.Type))
	extTextHeader.SetRange("no", purchLine.No)
	switch purchLine.Document_type {
	case gtables.PurchaseLine_Document_type.Quote:
		extTextHeader.SetRange("purchase_quote", true)
	case gtables.PurchaseLine_Document_type.Order:
		extTextHeader.SetRange("purchase_order", true)
	case gtables.PurchaseLine_Document_type.Invoice:
		extTextHeader.SetRange("purchase_invoice", true)
	case gtables.PurchaseLine_Document_type.CreditMemo:
		extTextHeader.SetRange("purchase_credit_memo", true)
	}
	return t.readLines(extTextHeader, purchHeader.Document_date, purchHeader.Language_code), nil
}

// InsertPurchExtText (InsertPurchExtText)
func (t *TransferExtendedText) InsertPurchExtText(purchLine *gtables.PurchaseLine) error {
	toPurchLine := gtables.NewPurchaseLine(purchLine)
	toPurchLine.SetRange("document_type", purchLine.Document_type)
	toPurchLine.SetRange("document_no", purchLine.Document_no)
	toPurchLine.Document_type, toPurchLine.Document_no, toPurchLine.Line_no = purchLine.Document_type, purchLine.Document_no, purchLine.Line_no
	hasNext := toPurchLine.Find(">")
	lineSpacing, err := t.lineSpacing(purchLine.Line_no, toPurchLine.Line_no, hasNext)
	if err != nil {
		return err
	}
	nextLineNo := purchLine.Line_no + lineSpacing
	for _, text := range t.tmpExtTextLines {
		toPurchLine = gtables.NewPurchaseLine(purchLine)
		toPurchLine.Init()
		toPurchLine.Document_type = purchLine.Document_type
		toPurchLine.Document_no = purchLine.Document_no
		toPurchLine.Line_no = nextLineNo
		nextLineNo = nextLineNo + lineSpacing
		toPurchLine.Description = types.Text(text)
		toPurchLine.Attached_to_line_no = purchLine.Line_no
		err = toPurchLine.InsertOrError(false)
		if err != nil {
			return err
		}
	}
	if len(t.tmpExtTextLines) > 0 {
		t.makeUpdateRequired = true
	}
	t.tmpExtTextLines = nil
	return nil
}

// deletePurchLines (DeletePurchLines)
func deletePurchLines(purchLine *gtables.PurchaseLine) bool {
	purchLine2 := gtables.NewPurchaseLine(purchLine)
	purchLine2.SetRange("document_type", purchLine.Document_type)
	purchLine2.SetRange("document_no", purchLine.Document_no)
	purchLine2.SetRange("attached_to_line_no", purchLine.Line_no)
	if purchLine2.Count() == 0 {
		return false
	}
	purchLine2.DeleteAll()
	return true
}

// ---- Reminders and finance charge memos ----

// ReminderCheckIfAnyExtText (ReminderCheckIfAnyExtText)
func (t *TransferExtendedText) ReminderCheckIfAnyExtText(reminderLine *gtables.ReminderLine, unconditionally bool) (bool, error) {
	t.makeUpdateRequired = false
	if reminderLine.Line_no != 0 {
		reminderLine2 := gtables.NewReminderLine(reminderLine)
		reminderLine2.SetRange("reminder_no", reminderLine.Reminder_no)
		reminderLine2.SetRange("attached_to_line_no", reminderLine.Line_no)
		if reminderLine2.Count() > 0 {
			reminderLine2.DeleteAll()
			t.makeUpdateRequired = true
		}
	}
	auto := unconditionally
	switch reminderLine.Type {
	case gtables.ReminderLine_Type.Blank:
		auto = true
	case gtables.ReminderLine_Type.GLAccount:
		auto = auto || autoText(reminderLine, int(gtables.ExtendedTextHeader_Table_name.GLAccount), reminderLine.No)
	}
	if !auto {
		return false, nil
	}
	err := reminderLine.TestField("reminder_no")
	if err != nil {
		return false, err
	}
	reminderHeader := gtables.NewReminderHeader(reminderLine)
	err = reminderHeader.GetOrError(reminderLine.Reminder_no)
	if err != nil {
		return false, err
	}
	extTextHeader := gtables.NewExtendedTextHeader(reminderLine)
	extTextHeader.SetRange("table_name", int(reminderLine.Type))
	extTextHeader.SetRange("no", reminderLine.No)
	extTextHeader.SetRange("reminder", true)
	return t.readLines(extTextHeader, reminderHeader.Document_date, reminderHeader.Language_code), nil
}

// InsertReminderExtText (InsertReminderExtText)
func (t *TransferExtendedText) InsertReminderExtText(reminderLine *gtables.ReminderLine) error {
	toReminderLine := gtables.NewReminderLine(reminderLine)
	toReminderLine.SetRange("reminder_no", reminderLine.Reminder_no)
	toReminderLine.Reminder_no, toReminderLine.Line_no = reminderLine.Reminder_no, reminderLine.Line_no
	hasNext := toReminderLine.Find(">")
	lineSpacing, err := t.lineSpacing(reminderLine.Line_no, toReminderLine.Line_no, hasNext)
	if err != nil {
		return err
	}
	nextLineNo := reminderLine.Line_no + lineSpacing
	for _, text := range t.tmpExtTextLines {
		toReminderLine = gtables.NewReminderLine(reminderLine)
		toReminderLine.Init()
		toReminderLine.Reminder_no = reminderLine.Reminder_no
		toReminderLine.Line_no = nextLineNo
		nextLineNo = nextLineNo + lineSpacing
		toReminderLine.Description = types.Text(text)
		toReminderLine.Attached_to_line_no = reminderLine.Line_no
		err = toReminderLine.InsertOrError(false)
		if err != nil {
			return err
		}
	}
	if len(t.tmpExtTextLines) > 0 {
		t.makeUpdateRequired = true
	}
	t.tmpExtTextLines = nil
	return nil
}

// FinChrgMemoCheckIfAnyExtText (FinChrgMemoCheckIfAnyExtText)
func (t *TransferExtendedText) FinChrgMemoCheckIfAnyExtText(finChrgMemoLine *gtables.FinanceChargeMemoLine, unconditionally bool) (bool, error) {
	t.makeUpdateRequired = false
	if finChrgMemoLine.Line_no != 0 {
		finChrgMemoLine2 := gtables.NewFinanceChargeMemoLine(finChrgMemoLine)
		finChrgMemoLine2.SetRange("finance_charge_memo_no", finChrgMemoLine.Finance_charge_memo_no)
		finChrgMemoLine2.SetRange("attached_to_line_no", finChrgMemoLine.Line_no)
		if finChrgMemoLine2.Count() > 0 {
			finChrgMemoLine2.DeleteAll()
			t.makeUpdateRequired = true
		}
	}
	auto := unconditionally
	switch finChrgMemoLine.Type {
	case gtables.FinanceChargeMemoLine_Type.Blank:
		auto = true
	case gtables.FinanceChargeMemoLine_Type.GLAccount:
		auto = auto || autoText(finChrgMemoLine, int(gtables.ExtendedTextHeader_Table_name.GLAccount), finChrgMemoLine.No)
	}
	if !auto {
		return false, nil
	}
	err := finChrgMemoLine.TestField("finance_charge_memo_no")
	if err != nil {
		return false, err
	}
	finChrgMemoHeader := gtables.NewFinanceChargeMemoHeader(finChrgMemoLine)
	err = finChrgMemoHeader.GetOrError(finChrgMemoLine.Finance_charge_memo_no)
	if err != nil {
		return false, err
	}
	extTextHeader := gtables.NewExtendedTextHeader(finChrgMemoLine)
	extTextHeader.SetRange("table_name", int(finChrgMemoLine.Type))
	extTextHeader.SetRange("no", finChrgMemoLine.No)
	extTextHeader.SetRange("finance_charge_memo", true)
	return t.readLines(extTextHeader, finChrgMemoHeader.Document_date, finChrgMemoHeader.Language_code), nil
}

// InsertFinChrgMemoExtText (InsertFinChrgMemoExtText)
func (t *TransferExtendedText) InsertFinChrgMemoExtText(finChrgMemoLine *gtables.FinanceChargeMemoLine) error {
	toFinChrgMemoLine := gtables.NewFinanceChargeMemoLine(finChrgMemoLine)
	toFinChrgMemoLine.SetRange("finance_charge_memo_no", finChrgMemoLine.Finance_charge_memo_no)
	toFinChrgMemoLine.Finance_charge_memo_no, toFinChrgMemoLine.Line_no = finChrgMemoLine.Finance_charge_memo_no, finChrgMemoLine.Line_no
	hasNext := toFinChrgMemoLine.Find(">")
	lineSpacing, err := t.lineSpacing(finChrgMemoLine.Line_no, toFinChrgMemoLine.Line_no, hasNext)
	if err != nil {
		return err
	}
	nextLineNo := finChrgMemoLine.Line_no + lineSpacing
	for _, text := range t.tmpExtTextLines {
		toFinChrgMemoLine = gtables.NewFinanceChargeMemoLine(finChrgMemoLine)
		toFinChrgMemoLine.Init()
		toFinChrgMemoLine.Finance_charge_memo_no = finChrgMemoLine.Finance_charge_memo_no
		toFinChrgMemoLine.Line_no = nextLineNo
		nextLineNo = nextLineNo + lineSpacing
		toFinChrgMemoLine.Description = types.Text(text)
		toFinChrgMemoLine.Attached_to_line_no = finChrgMemoLine.Line_no
		err = toFinChrgMemoLine.InsertOrError(false)
		if err != nil {
			return err
		}
	}
	if len(t.tmpExtTextLines) > 0 {
		t.makeUpdateRequired = true
	}
	t.tmpExtTextLines = nil
	return nil
}

// ---- The subpages' InsertExtendedText (Insert Ext. Texts: unconditionally; entering No.: the
// automatic texts) ----

func extTextResult(t *TransferExtendedText) fcodeunits.Result {
	return fcodeunits.Result{Success: true, Refresh: t.MakeUpdate()}
}

func salesLineInsertExtendedText(rec *gtables.SalesLine, args functions.Args) (fcodeunits.Result, error) {
	t := &TransferExtendedText{}
	found, err := t.SalesCheckIfAnyExtText(rec, args.Bool(0))
	if err == nil && found {
		err = t.InsertSalesExtText(rec)
	}
	return extTextResult(t), err
}

func purchLineInsertExtendedText(rec *gtables.PurchaseLine, args functions.Args) (fcodeunits.Result, error) {
	t := &TransferExtendedText{}
	found, err := t.PurchCheckIfAnyExtText(rec, args.Bool(0))
	if err == nil && found {
		err = t.InsertPurchExtText(rec)
	}
	return extTextResult(t), err
}

func reminderLineInsertExtendedText(rec *gtables.ReminderLine, args functions.Args) (fcodeunits.Result, error) {
	t := &TransferExtendedText{}
	found, err := t.ReminderCheckIfAnyExtText(rec, args.Bool(0))
	if err == nil && found {
		err = t.InsertReminderExtText(rec)
	}
	return extTextResult(t), err
}

func finChrgMemoLineInsertExtendedText(rec *gtables.FinanceChargeMemoLine, args functions.Args) (fcodeunits.Result, error) {
	t := &TransferExtendedText{}
	found, err := t.FinChrgMemoCheckIfAnyExtText(rec, args.Bool(0))
	if err == nil && found {
		err = t.InsertFinChrgMemoExtText(rec)
	}
	return extTextResult(t), err
}
