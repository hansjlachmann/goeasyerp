package tables

import (
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// financeChargeMemoLineLogic is the business logic of table 303 Finance Charge Memo Line: a text,
// the additional fee (G/L account) or a customer entry with its interest.
type financeChargeMemoLineLogic struct{}

func init() { gtables.RegisterFinanceChargeMemoLineLogic(financeChargeMemoLineLogic{}) }

// OnInsert: the memo exists
func (financeChargeMemoLineLogic) OnInsert(rec *gtables.FinanceChargeMemoLine) error {
	finChrgMemoHeader := gtables.NewFinanceChargeMemoHeader(rec)
	err := finChrgMemoHeader.GetOrError(rec.Finance_charge_memo_no)
	if err != nil {
		return err
	}
	rec.Attached_to_line_no = 0
	return nil
}

// OnDelete: the lines attached to it
func (financeChargeMemoLineLogic) OnDelete(rec *gtables.FinanceChargeMemoLine) error {
	finChrgMemoLine := gtables.NewFinanceChargeMemoLine(rec)
	finChrgMemoLine.SetRange("finance_charge_memo_no", rec.Finance_charge_memo_no)
	finChrgMemoLine.SetRange("attached_to_line_no", rec.Line_no)
	finChrgMemoLine.DeleteAll()
	return nil
}

// getFinChrgMemoHeader (GetFinChrgMemoHeader, ProcessFinChrgMemoHeader): the memo — with a
// customer, document date, posting group and terms —, its terms and its currency's rounding
func getFinChrgMemoHeader(src ftables.Source, finChrgMemoNo types.Code) (*gtables.FinanceChargeMemoHeader, *gtables.FinanceChargeTerms, *gtables.Currency, error) {
	finChrgMemoHeader := gtables.NewFinanceChargeMemoHeader(src)
	err := finChrgMemoHeader.GetOrError(finChrgMemoNo)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, field := range []string{"customer_no", "document_date", "customer_posting_group", "fin_charge_terms_code"} {
		err = finChrgMemoHeader.TestField(field)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	finChrgTerms := gtables.NewFinanceChargeTerms(src)
	err = finChrgTerms.GetOrError(finChrgMemoHeader.Fin_charge_terms_code)
	if err != nil {
		return nil, nil, nil, err
	}
	currency := gtables.NewCurrency(src)
	if finChrgMemoHeader.Currency_code == "" {
		InitRoundingPrecision(currency)
	} else {
		err = currency.GetOrError(finChrgMemoHeader.Currency_code)
		if err == nil {
			err = currency.TestField("amount_rounding_precision")
		}
		if err != nil {
			return nil, nil, nil, err
		}
	}
	return finChrgMemoHeader, finChrgTerms, currency, nil
}

// OnValidate_Type: a new type clears the line
func (financeChargeMemoLineLogic) OnValidate_Type(rec *gtables.FinanceChargeMemoLine) error {
	xRec := rec.XRec()
	if rec.Type != xRec.Type {
		lineType := rec.Type
		rec.Init()
		rec.Type = lineType
		_, _, _, err := getFinChrgMemoHeader(rec, rec.Finance_charge_memo_no)
		return err
	}
	return nil
}

// OnValidate_Entry_no: an entry of the memo's customer (open or closed by the terms' Interest
// Calculation), not on hold, in its currency — its dates, document, amounts and interest
func (financeChargeMemoLineLogic) OnValidate_Entry_no(rec *gtables.FinanceChargeMemoLine) error {
	err := rec.TestField("type", gtables.FinanceChargeMemoLine_Type.CustomerLedgerEntry)
	if err == nil {
		err = rec.TestField("attached_to_line_no", 0)
	}
	if err != nil {
		return err
	}
	finChrgMemoHeader, finChrgTerms, _, err := getFinChrgMemoHeader(rec, rec.Finance_charge_memo_no)
	if err != nil {
		return err
	}
	custLedgEntry := gtables.NewCustomerLedgerEntry(rec)
	err = custLedgEntry.GetOrError(rec.Entry_no)
	if err != nil {
		return err
	}
	switch finChrgTerms.Interest_calculation {
	case gtables.FinanceChargeTerms_Interest_calculation.OpenEntries:
		err = custLedgEntry.TestField("open", true)
	case gtables.FinanceChargeTerms_Interest_calculation.ClosedEntries:
		err = custLedgEntry.TestField("open", false)
	}
	if err == nil {
		err = custLedgEntry.TestField("customer_no", finChrgMemoHeader.Customer_no)
	}
	if err == nil {
		err = custLedgEntry.TestField("on_hold", types.Code(""))
	}
	if err != nil {
		return err
	}
	if custLedgEntry.Currency_code != finChrgMemoHeader.Currency_code {
		return apperrors.ReminderCurrencyMismatch(gtables.FinanceChargeMemoHeaderTableName)
	}
	rec.Posting_date = custLedgEntry.Posting_date
	rec.Document_date = custLedgEntry.Document_date
	rec.Due_date = custLedgEntry.Due_date
	rec.Document_type = gtables.FinanceChargeMemoLineDocument_type(custLedgEntry.Document_type)
	rec.Document_no = custLedgEntry.Document_no
	rec.Description = custLedgEntry.Description
	rec.Original_amount = custLedgEntry.Amount
	rec.Remaining_amount = custLedgEntry.Remaining_amount
	return calcFinChrgMemoFinChrg(rec, finChrgMemoHeader, finChrgTerms)
}

// setFinChrgCustLedgEntryView (SetCustLedgEntryView): the customer's entries the terms charge —
// open ones, or all
func setFinChrgCustLedgEntryView(custLedgEntry *gtables.CustomerLedgerEntry, finChrgMemoHeader *gtables.FinanceChargeMemoHeader, finChrgTerms *gtables.FinanceChargeTerms) {
	custLedgEntry.SetRange("customer_no", finChrgMemoHeader.Customer_no)
	if finChrgTerms.Interest_calculation == gtables.FinanceChargeTerms_Interest_calculation.OpenEntries {
		custLedgEntry.SetCurrentKey("customer_no", "open")
		custLedgEntry.SetRange("open", true)
	}
}

// OnValidate_Document_type: the entry again by the document
func (financeChargeMemoLineLogic) OnValidate_Document_type(rec *gtables.FinanceChargeMemoLine) error {
	err := rec.TestField("type", gtables.FinanceChargeMemoLine_Type.CustomerLedgerEntry)
	if err != nil {
		return err
	}
	return rec.Validate("document_no", rec.Document_no)
}

// OnValidate_Document_no: the customer's entry of the document
func (financeChargeMemoLineLogic) OnValidate_Document_no(rec *gtables.FinanceChargeMemoLine) error {
	err := rec.TestField("type", gtables.FinanceChargeMemoLine_Type.CustomerLedgerEntry)
	if err != nil {
		return err
	}
	rec.Entry_no = 0
	if rec.Document_no != "" {
		finChrgMemoHeader, finChrgTerms, _, err := getFinChrgMemoHeader(rec, rec.Finance_charge_memo_no)
		if err != nil {
			return err
		}
		custLedgEntry := gtables.NewCustomerLedgerEntry(rec)
		setFinChrgCustLedgEntryView(custLedgEntry, finChrgMemoHeader, finChrgTerms)
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

// OnValidate_No: a standard text's description, or a G/L account with its tax groups
func (financeChargeMemoLineLogic) OnValidate_No(rec *gtables.FinanceChargeMemoLine) error {
	if rec.No == "" {
		return nil
	}
	switch rec.Type {
	case gtables.FinanceChargeMemoLine_Type.Blank:
		standardText := gtables.NewStandardText(rec)
		err := standardText.GetOrError(rec.No)
		if err != nil {
			return err
		}
		rec.Description = standardText.Description
	case gtables.FinanceChargeMemoLine_Type.CustomerLedgerEntry:
		return apperrors.ReminderLineTypeMustBe(gtables.FinanceChargeMemoLineTableName, "type", gtables.FinanceChargeMemoLine_Type.Blank, gtables.FinanceChargeMemoLine_Type.GLAccount)
	case gtables.FinanceChargeMemoLine_Type.GLAccount:
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
		_, _, _, err = getFinChrgMemoHeader(rec, rec.Finance_charge_memo_no)
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

// OnValidate_Amount: not on text lines; rounded, with its tax
func (financeChargeMemoLineLogic) OnValidate_Amount(rec *gtables.FinanceChargeMemoLine) error {
	if rec.Type == gtables.FinanceChargeMemoLine_Type.Blank {
		return apperrors.ReminderLineTypeMustBe(gtables.FinanceChargeMemoLineTableName, "type", gtables.FinanceChargeMemoLine_Type.GLAccount, gtables.FinanceChargeMemoLine_Type.CustomerLedgerEntry)
	}
	finChrgMemoHeader, _, currency, err := getFinChrgMemoHeader(rec, rec.Finance_charge_memo_no)
	if err != nil {
		return err
	}
	rec.Amount = system.Round(rec.Amount, currency.Amount_rounding_precision)
	rec.Tax_amount, rec.Tax_percent, err = reminderLineTax(rec, finChrgMemoHeader.Tax_area_code, rec.Tax_group_code, finChrgMemoHeader.Tax_liable,
		finChrgMemoHeader.Posting_date, rec.Amount, rec.Tax_percent, int(rec.Tax_calculation_type))
	if err != nil {
		return err
	}
	rec.Tax_amount = system.Round(rec.Tax_amount, currency.Amount_rounding_precision)
	return nil
}

// OnValidate_Interest_rate: the interest again (CalcFinChrg takes the terms' rate)
func (financeChargeMemoLineLogic) OnValidate_Interest_rate(rec *gtables.FinanceChargeMemoLine) error {
	err := rec.TestField("type", gtables.FinanceChargeMemoLine_Type.CustomerLedgerEntry)
	if err == nil {
		err = rec.TestField("entry_no")
	}
	if err != nil {
		return err
	}
	finChrgMemoHeader, finChrgTerms, _, err := getFinChrgMemoHeader(rec, rec.Finance_charge_memo_no)
	if err != nil {
		return err
	}
	return calcFinChrgMemoFinChrg(rec, finChrgMemoHeader, finChrgTerms)
}

// OnValidate_Gen_prod_posting_group: the group's default tax product posting group
func (financeChargeMemoLineLogic) OnValidate_Gen_prod_posting_group(rec *gtables.FinanceChargeMemoLine) error {
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
func (financeChargeMemoLineLogic) OnValidate_Tax_group_code(rec *gtables.FinanceChargeMemoLine) error {
	return rec.Validate("tax_prod_posting_group", rec.Tax_prod_posting_group)
}

// OnValidate_Tax_prod_posting_group: the Tax Posting Setup's Tax % and calculation type, the tax
// again
func (financeChargeMemoLineLogic) OnValidate_Tax_prod_posting_group(rec *gtables.FinanceChargeMemoLine) error {
	finChrgMemoHeader, _, _, err := getFinChrgMemoHeader(rec, rec.Finance_charge_memo_no)
	if err != nil {
		return err
	}
	taxPostingSetup := gtables.NewTaxPostingSetup(rec)
	if !taxPostingSetup.Get(map[string]interface{}{"tax_bus_posting_group": finChrgMemoHeader.Tax_bus_posting_group, "tax_prod_posting_group": rec.Tax_prod_posting_group}) {
		taxPostingSetup.Init()
	}
	rec.Tax_percent = taxPostingSetup.Vat_percent
	rec.Tax_calculation_type = gtables.FinanceChargeMemoLineTax_calculation_type(taxPostingSetup.Tax_calculation_type)
	if rec.Tax_calculation_type == gtables.FinanceChargeMemoLine_Tax_calculation_type.ReverseChargeVAT {
		rec.Tax_percent = types.Decimal{}
	}
	if rec.Type == gtables.FinanceChargeMemoLine_Type.Blank {
		return nil
	}
	return rec.Validate("amount", rec.Amount)
}

// calcFinChrgMemoFinChrg (CalcFinChrg): the entry's interest by the memo's terms (not on hold),
// the terms' Line Description (%1 entry description, %2 document type, %3 document no., %4
// interest rate, %5 original amount, %6 remaining amount, %7 due date, %8 currency code), the
// interest account's posting groups
func calcFinChrgMemoFinChrg(rec *gtables.FinanceChargeMemoLine, finChrgMemoHeader *gtables.FinanceChargeMemoHeader, finChrgTerms *gtables.FinanceChargeTerms) error {
	rec.Amount = types.Decimal{}
	rec.Tax_amount = types.Decimal{}
	rec.Tax_calculation_type = gtables.FinanceChargeMemoLine_Tax_calculation_type.NormalVAT
	rec.Gen_prod_posting_group = ""
	rec.Tax_prod_posting_group = ""
	rec.Interest_rate = types.Decimal{}
	custLedgEntry := gtables.NewCustomerLedgerEntry(rec)
	err := custLedgEntry.GetOrError(rec.Entry_no)
	if err != nil {
		return err
	}
	if custLedgEntry.On_hold != "" {
		return nil
	}
	amount, interestRate, err := CalcEntryInterest(rec, custLedgEntry, finChrgMemoHeader.Fin_charge_terms_code, rec.Due_date, rec.Remaining_amount, finChrgMemoHeader.Document_date, true)
	if err != nil {
		return err
	}
	rec.Interest_rate = interestRate
	rec.Amount = amount
	if finChrgTerms.Line_description != "" {
		lang := fcodeunits.CurrentLanguage()
		docTypeText := ""
		if rec.Document_type != 0 {
			docTypeText = i18n.GetInstance().OptionCaption(gtables.FinanceChargeMemoLineTableName, "document_type", i18n.Key(rec.Document_type.String()), lang)
		}
		if docTypeText == "" {
			docTypeText = i18n.GetInstance().Message("FIN_CHRG_DOCUMENT", lang)
		}
		rec.Description = types.Text(system.CopyStr(system.StrSubstNo(finChrgTerms.Line_description.String(),
			custLedgEntry.Description, docTypeText, rec.Document_no, rec.Interest_rate,
			formatTextAmount(rec.Original_amount, lang), formatTextAmount(rec.Remaining_amount, lang), formatTextDate(rec.Due_date, lang),
			finChrgMemoHeader.Currency_code), 1, rec.MaxStrLen("description")))
	}
	if !rec.Amount.IsZero() {
		return setInterestPostingGroups(rec, finChrgMemoHeader.Customer_posting_group, func(genProd, taxProd types.Code) error {
			err := rec.Validate("gen_prod_posting_group", genProd)
			if err == nil {
				err = rec.Validate("tax_prod_posting_group", taxProd)
			}
			return err
		})
	}
	return nil
}
