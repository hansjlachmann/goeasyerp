package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// extendedTextHeaderLogic is the business logic of table 279 Extended_Text_Header: an extended
// text of a standard text, G/L account, item or resource in a language (or all languages),
// valid between two dates, for the document types it is checked for.
type extendedTextHeaderLogic struct{}

func init() { gtables.RegisterExtendedTextHeaderLogic(extendedTextHeaderLogic{}) }

// OnInsert: the next text number of the record and language; a resource's text is never for
// purchases
func (extendedTextHeaderLogic) OnInsert(rec *gtables.ExtendedTextHeader) error {
	if rec.Text_no == 0 {
		extTxt2 := gtables.NewExtendedTextHeader(rec)
		extTxt2.SetRange("table_name", rec.Table_name)
		extTxt2.SetRange("no", rec.No)
		extTxt2.SetRange("language_code", rec.Language_code)
		if extTxt2.FindLast() {
			rec.Text_no = extTxt2.Text_no + 1
		} else {
			rec.Text_no = 1
		}
	} else {
		rec.Text_no = rec.Text_no + 1
	}
	if rec.Table_name == gtables.ExtendedTextHeader_Table_name.Resource {
		rec.Purchase_quote = false
		rec.Purchase_invoice = false
		rec.Purchase_order = false
		rec.Purchase_credit_memo = false
	}
	return nil
}

// OnDelete: the text's lines go with it
func (extendedTextHeaderLogic) OnDelete(rec *gtables.ExtendedTextHeader) error {
	txtLin := gtables.NewExtendedTextLine(rec)
	txtLin.SetRange("table_name", rec.Table_name)
	txtLin.SetRange("no", rec.No)
	txtLin.SetRange("language_code", rec.Language_code)
	txtLin.SetRange("text_no", rec.Text_no)
	txtLin.DeleteAll()
	return nil
}

// OnRename: an extended text is never renamed
func (extendedTextHeaderLogic) OnRename(rec *gtables.ExtendedTextHeader) error {
	return apperrors.RenameNotAllowed(gtables.ExtendedTextHeaderTableName)
}

// OnValidate_Language_code: a text in one language is not for all languages
func (extendedTextHeaderLogic) OnValidate_Language_code(rec *gtables.ExtendedTextHeader) error {
	if rec.Language_code != "" {
		return rec.TestField("all_language_codes", false)
	}
	return nil
}

// OnValidate_All_language_codes: a text for all languages has no language
func (extendedTextHeaderLogic) OnValidate_All_language_codes(rec *gtables.ExtendedTextHeader) error {
	if rec.All_language_codes {
		return rec.TestField("language_code", types.Code(""))
	}
	return nil
}

func (extendedTextHeaderLogic) OnValidate_Purchase_quote(rec *gtables.ExtendedTextHeader) error {
	return noResourcePurch(rec, rec.Purchase_quote, "purchase_quote")
}

func (extendedTextHeaderLogic) OnValidate_Purchase_invoice(rec *gtables.ExtendedTextHeader) error {
	return noResourcePurch(rec, rec.Purchase_invoice, "purchase_invoice")
}

func (extendedTextHeaderLogic) OnValidate_Purchase_order(rec *gtables.ExtendedTextHeader) error {
	return noResourcePurch(rec, rec.Purchase_order, "purchase_order")
}

func (extendedTextHeaderLogic) OnValidate_Purchase_credit_memo(rec *gtables.ExtendedTextHeader) error {
	return noResourcePurch(rec, rec.Purchase_credit_memo, "purchase_credit_memo")
}

// noResourcePurch (NoResourcePurch): resources are not purchased
func noResourcePurch(rec *gtables.ExtendedTextHeader, on bool, field string) error {
	if on && rec.Table_name == gtables.ExtendedTextHeader_Table_name.Resource {
		return apperrors.ExtTextNoResourcePurch(gtables.ExtendedTextHeaderTableName, field)
	}
	return nil
}

// DeleteExtendedTexts: the extended texts of a record that is deleted (OnDelete of Standard Text,
// G/L Account, Item and Resource: ExtTextHeader.DELETEALL(TRUE), each with its lines)
func DeleteExtendedTexts(src ftables.Source, tableName gtables.ExtendedTextHeaderTable_name, no types.Code) error {
	extTextHeader := gtables.NewExtendedTextHeader(src)
	extTextHeader.SetRange("table_name", tableName)
	extTextHeader.SetRange("no", no)
	for found := extTextHeader.FindSetBuffered(); found; found = extTextHeader.Next() {
		err := extTextHeader.DeleteOrError(true)
		if err != nil {
			return err
		}
	}
	return nil
}
