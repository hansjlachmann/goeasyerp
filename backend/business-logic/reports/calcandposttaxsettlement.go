package reports

import (
	"strconv"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/genjnl"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 20 Calc. and Post Tax Settlement: the open purchase and sales tax entries of the period
// are settled per tax posting setup — and for sales tax per jurisdiction and use tax: a journal
// line (posting type Settlement, Manual VAT Entry) takes their tax off the tax account (the
// setup's or the jurisdiction's; use tax also off the reverse charge account), its counterpart
// goes to the settlement account, and the tax entries are closed by the settlement's tax entry.
// Without Post it prints what would be posted (a test report). The lines carry the entries'
// additional-currency amounts (Source Currency Amount), so the settlement clears them in that
// currency too; Show Amounts in Add. Reporting Currency prints those amounts.
const CalcAndPostTaxSettlementID = 20

func init() {
	Register(CalcAndPostTaxSettlementID, "calc-and-post-tax-settlement", func() Report { return &calcAndPostTaxSettlement{} })
}

// taxSettlementGroup is a set of open tax entries settled by one journal line
type taxSettlementGroup struct {
	taxType      gtables.TaxEntryType
	jurisdiction types.Code
	useTax       bool
}

type calcAndPostTaxSettlement struct {
	startDate, endDate, postingDate types.Date
	docNo, settlementAccount        types.Code
	printTaxEntries, post           bool
	taxDateFilter                   string

	sourceCodeSetup  *gtables.SourceCodeSetup
	genJnlPostLine   *genjnl.GenJnlPostLine
	taxPostingSetup  *gtables.TaxPostingSetup
	groups           []taxSettlementGroup
	group            taxSettlementGroup
	taxEntry         *gtables.TaxEntry
	nextTaxEntryNo   int
	taxAmount        types.Decimal
	taxAmountAddCurr types.Decimal
	useAmtsInAddCurr bool
	glSetup          *gtables.GeneralLedgerSetup
}

func (r *calcAndPostTaxSettlement) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "starting_date", Type: "date"},
			{Name: "ending_date", Type: "date"},
			{Name: "posting_date", Type: "date", Required: true},
			{Name: "document_no", Type: "text", Required: true},
			{Name: "settlement_account", Type: "text", Required: true, Relation: &OptionRelation{Table: gtables.GLAccountTableName, Columns: []string{"no", "name"}}},
			{Name: "show_tax_entries", Type: "bool", Default: "false"},
			{Name: "post", Type: "bool", Default: "false"},
			{Name: "show_amounts_in_add_currency", Type: "bool", Default: "false"},
		},
		Transaction: true,
		OnPreReport: r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "Tax_Posting_Setup",
			Table:               gtables.TaxPostingSetupTableName,
			Key:                 []string{"tax_bus_posting_group", "tax_prod_posting_group"},
			RequestFilterFields: []string{"tax_bus_posting_group", "tax_prod_posting_group"},
			OnPreDataItem:       Trigger(r.setupOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.setupOnAfterGetRecord),
			OnPostDataItem:      Trigger(r.setupOnPostDataItem),
			DataItems: []*DataItem{{
				Name:             "Closing_GL_and_Tax_Entry",
				Table:            IntegerTable,
				OnPreDataItem:    Trigger(r.closingOnPreDataItem),
				OnAfterGetRecord: Trigger(r.closingOnAfterGetRecord),
				DataItems: []*DataItem{
					{
						Name:  "Tax_Entry",
						Table: gtables.TaxEntryTableName,
						Key:   []string{"type", "closed", "tax_bus_posting_group", "tax_prod_posting_group", "tax_jurisdiction_code", "use_tax", "posting_date"},
						TableView: []ftables.FieldFilter{{Field: "closed", Expression: "false"},
							{Field: "type", Expression: strconv.Itoa(int(gtables.TaxEntry_Type.Purchase)) + "|" + strconv.Itoa(int(gtables.TaxEntry_Type.Sale))}},
						OnPreDataItem: Trigger(r.taxEntryOnPreDataItem),
					},
					{
						Name:             "Close_Tax_Entries",
						Table:            IntegerTable,
						OnPreDataItem:    Trigger(func(rec *Integer, run *Run) error { rec.SetRange("number", 1); return nil }),
						OnAfterGetRecord: Trigger(r.closeOnAfterGetRecord),
					},
				},
			}},
		}},
	}
}

func (r *calcAndPostTaxSettlement) onPreReport(run *Run) error {
	r.startDate = run.OptionDate("starting_date")
	r.endDate = run.OptionDate("ending_date")
	r.postingDate = run.OptionDate("posting_date")
	if r.postingDate.IsZero() {
		return system.Error(run.Text("ENTER_POSTING_DATE"))
	}
	r.docNo = types.Code(system.UpperCase(run.Option("document_no")))
	if r.docNo == "" {
		return system.Error(run.Text("ENTER_DOCUMENT_NO"))
	}
	r.settlementAccount = types.Code(system.UpperCase(run.Option("settlement_account")))
	if r.settlementAccount == "" {
		return system.Error(run.Text("ENTER_SETTLEMENT_ACCOUNT"))
	}
	err := gtables.NewGLAccount(run).GetOrError(r.settlementAccount)
	if err != nil {
		return err
	}
	r.printTaxEntries = run.OptionBool("show_tax_entries")
	r.post = run.OptionBool("post")
	r.useAmtsInAddCurr = run.OptionBool("show_amounts_in_add_currency")

	switch {
	case r.endDate.IsZero() && r.startDate.IsZero():
		r.taxDateFilter = ""
	case r.endDate.IsZero():
		r.taxDateFilter = r.startDate.String() + ".."
	default:
		r.taxDateFilter = r.startDate.String() + ".." + r.endDate.String()
	}

	setReportHeadValues(run)
	setPeriodText(run, r.taxDateFilter)
	run.SetValue("posting_date", r.postingDate)
	run.SetValue("document_no", r.docNo.String())
	run.SetValue("settlement_account", r.settlementAccount.String())
	run.SetValue("not_posted", !r.post)
	run.SetValue("use_add_curr", r.useAmtsInAddCurr)
	r.glSetup = gtables.GetGeneralLedgerSetup(run)
	if r.useAmtsInAddCurr {
		run.SetValue("amounts_in", run.Text("AMOUNTS_IN", r.glSetup.Additional_reporting_currency))
	} else {
		err = r.glSetup.TestField("lcy_code")
		if err != nil {
			return err
		}
		run.SetValue("amounts_in", run.Text("AMOUNTS_IN", r.glSetup.Lcy_code))
	}
	return nil
}

func (r *calcAndPostTaxSettlement) setupOnPreDataItem(setup *gtables.TaxPostingSetup, run *Run) error {
	gtables.NewGLEntry(run).LockTable() // Avoid deadlock with codeunit 12
	taxEntry := gtables.NewTaxEntry(run)
	taxEntry.LockTable()
	if taxEntry.FindLast() {
		r.nextTaxEntryNo = taxEntry.Entry_no
	}
	r.sourceCodeSetup = gtables.GetSourceCodeSetup(run)
	r.genJnlPostLine = genjnl.NewGenJnlPostLine(run)
	r.taxAmount = types.Decimal{}
	r.taxAmountAddCurr = types.Decimal{}
	run.SetValue("tax_amount", r.taxAmount)
	setFilterText(run, setup)

	if r.post && system.GuiAllowed() && !system.Confirm(run.Text("CONFIRM")) {
		return Break()
	}
	return nil
}

func (r *calcAndPostTaxSettlement) setupOnAfterGetRecord(setup *gtables.TaxPostingSetup, run *Run) error {
	r.taxPostingSetup = setup
	return nil
}

// setupOnPostDataItem: the settled tax goes to the settlement account
func (r *calcAndPostTaxSettlement) setupOnPostDataItem(setup *gtables.TaxPostingSetup, run *Run) error {
	if r.taxAmount.IsZero() {
		return nil
	}
	genJnlLine := gtables.NewGenJournalLine(run)
	genJnlLine.Init()
	genJnlLine.System_created_entry = true
	genJnlLine.Account_type = gtables.GenJournalLine_Account_type.GLAccount
	err := genJnlLine.Validate("account_no", r.settlementAccount)
	if err != nil {
		return err
	}
	genJnlLine.Posting_date = r.postingDate
	genJnlLine.Document_type = gtables.GenJournalLine_Document_type.Blank
	genJnlLine.Document_no = r.docNo
	genJnlLine.Description = types.Text(run.Text("TAX_SETTLEMENT"))
	err = genJnlLine.Validate("amount", r.taxAmount)
	if err != nil {
		return err
	}
	genJnlLine.Source_currency_code = r.glSetup.Additional_reporting_currency
	genJnlLine.Source_currency_amount = r.taxAmountAddCurr
	genJnlLine.Source_code = r.sourceCodeSetup.Tax_settlement
	genJnlLine.Vat_posting = gtables.GenJournalLine_Vat_posting.ManualVATEntry
	if r.post {
		return r.genJnlPostLine.Run(genJnlLine)
	}
	return nil
}

// closingOnPreDataItem: the open tax entries of the setup and period in groups — purchase, then
// sale; for sales tax per jurisdiction and use tax
func (r *calcAndPostTaxSettlement) closingOnPreDataItem(rec *Integer, run *Run) error {
	r.groups = nil
	salesTax := r.taxPostingSetup.Tax_calculation_type == gtables.TaxPostingSetup_Tax_calculation_type.SalesTax
	taxEntry := r.newTaxEntryFilter(run)
	taxEntry.SetRange("type", gtables.TaxEntry_Type.Purchase, gtables.TaxEntry_Type.Sale)
	for found := taxEntry.FindSet(); found; found = taxEntry.Next() {
		g := taxSettlementGroup{taxType: taxEntry.Type}
		if salesTax {
			g.jurisdiction = taxEntry.Tax_jurisdiction_code
			g.useTax = taxEntry.Use_tax
		}
		if len(r.groups) == 0 || r.groups[len(r.groups)-1] != g {
			r.groups = append(r.groups, g)
		}
	}
	if len(r.groups) == 0 {
		return Break()
	}
	rec.SetRange("number", 1, len(r.groups))
	return nil
}

// closingOnAfterGetRecord: the tax entries of the next group
func (r *calcAndPostTaxSettlement) closingOnAfterGetRecord(rec *Integer, run *Run) error {
	r.group = r.groups[rec.Number-1]
	r.taxEntry = r.newTaxEntryFilter(run)
	r.taxEntry.SetRange("type", r.group.taxType)
	if r.taxPostingSetup.Tax_calculation_type == gtables.TaxPostingSetup_Tax_calculation_type.SalesTax {
		r.taxEntry.SetRange("tax_jurisdiction_code", r.group.jurisdiction)
		r.taxEntry.SetRange("use_tax", r.group.useTax)
	}
	return nil
}

// newTaxEntryFilter: the open tax entries of the setup in the period
func (r *calcAndPostTaxSettlement) newTaxEntryFilter(run *Run) *gtables.TaxEntry {
	taxEntry := gtables.NewTaxEntry(run)
	taxEntry.SetCurrentKey("type", "closed", "tax_bus_posting_group", "tax_prod_posting_group", "tax_jurisdiction_code", "use_tax", "posting_date")
	taxEntry.SetRange("closed", false)
	if r.taxDateFilter != "" {
		taxEntry.SetFilter("posting_date", r.taxDateFilter)
	}
	taxEntry.SetRange("tax_bus_posting_group", r.taxPostingSetup.Tax_bus_posting_group)
	taxEntry.SetRange("tax_prod_posting_group", r.taxPostingSetup.Tax_prod_posting_group)
	return taxEntry
}

// taxEntryOnPreDataItem: the group's entries, printed with Show Tax Entries
func (r *calcAndPostTaxSettlement) taxEntryOnPreDataItem(taxEntry *gtables.TaxEntry, run *Run) error {
	if !r.printTaxEntries {
		return Break()
	}
	taxEntry.CopyFilters(r.taxEntry)
	return nil
}

// closeOnAfterGetRecord: the settlement of the group — its tax off the tax account(s) — and the
// group's entries closed
func (r *calcAndPostTaxSettlement) closeOnAfterGetRecord(rec *Integer, run *Run) error {
	err := r.taxEntry.CalcSums("base", "amount", "additional_currency_base", "additional_currency_amount")
	if err != nil {
		return err
	}
	setup := r.taxPostingSetup
	run.SetValue("reversing_entry", false)

	// Balancing entries to tax accounts
	genJnlLine := r.newSettlementLine(run)
	switch r.group.taxType {
	case gtables.TaxEntry_Type.Purchase:
		genJnlLine.Description = types.Text(run.Text("PURCHASE_SETTLEMENT", setup.Tax_bus_posting_group, setup.Tax_prod_posting_group))
	case gtables.TaxEntry_Type.Sale:
		genJnlLine.Description = types.Text(run.Text("SALES_SETTLEMENT", setup.Tax_bus_posting_group, setup.Tax_prod_posting_group))
	}
	genJnlLine.Tax_bus_posting_group = setup.Tax_bus_posting_group
	genJnlLine.Tax_prod_posting_group = setup.Tax_prod_posting_group
	genJnlLine.Tax_calculation_type = gtables.GenJournalLineTax_calculation_type(setup.Tax_calculation_type)
	genJnlLine.Gen_posting_type = gtables.GenJournalLine_Gen_posting_type.Settlement

	purchase := r.group.taxType == gtables.TaxEntry_Type.Purchase
	var account types.Code
	var reverseAccount types.Code // the counterpart of use tax or reverse charge
	addToSettlement := true
	switch setup.Tax_calculation_type {
	case gtables.TaxPostingSetup_Tax_calculation_type.NormalVAT, gtables.TaxPostingSetup_Tax_calculation_type.FullVAT,
		gtables.TaxPostingSetup_Tax_calculation_type.ReverseChargeVAT:
		field := "sales_vat_account"
		account = setup.Sales_vat_account
		if purchase {
			field = "purchase_vat_account"
			account = setup.Purchase_vat_account
		}
		err = setup.TestField(field)
		if err != nil {
			return err
		}
		if setup.Tax_calculation_type == gtables.TaxPostingSetup_Tax_calculation_type.ReverseChargeVAT {
			addToSettlement = false
			if purchase {
				err = setup.TestField("reverse_chrg_vat_acc")
				if err != nil {
					return err
				}
				reverseAccount = setup.Reverse_chrg_vat_acc
			}
		}
	case gtables.TaxPostingSetup_Tax_calculation_type.SalesTax:
		taxJurisdiction := gtables.NewTaxJurisdiction(run)
		err = taxJurisdiction.GetOrError(r.group.jurisdiction)
		if err != nil {
			return err
		}
		genJnlLine.Tax_area_code = taxJurisdiction.Code
		genJnlLine.Use_tax = r.group.useTax
		field := "tax_account_sales"
		account = taxJurisdiction.Tax_account_sales
		if purchase {
			field = "tax_account_purchases"
			account = taxJurisdiction.Tax_account_purchases
		}
		err = taxJurisdiction.TestField(field)
		if err != nil {
			return err
		}
		if purchase && r.group.useTax {
			addToSettlement = false
			err = taxJurisdiction.TestField("reverse_charge_purchases")
			if err != nil {
				return err
			}
			reverseAccount = taxJurisdiction.Reverse_charge_purchases
		}
	}
	genJnlLine.Account_no = account
	err = genJnlLine.Validate("amount", r.taxEntry.Amount.Neg())
	if err != nil {
		return err
	}
	genJnlLine.Tax_amount = r.taxEntry.Amount.Neg()
	genJnlLine.Tax_base_amount = r.taxEntry.Base.Neg()
	genJnlLine.Source_currency_code = r.glSetup.Additional_reporting_currency
	genJnlLine.Source_currency_amount = r.taxEntry.Additional_currency_amount.Neg()
	genJnlLine.Source_curr_tax_amount = r.taxEntry.Additional_currency_amount.Neg()
	genJnlLine.Source_curr_tax_base_amount = r.taxEntry.Additional_currency_base.Neg()
	if r.useAmtsInAddCurr {
		run.SetValue("settlement_base", genJnlLine.Source_curr_tax_base_amount)
		run.SetValue("settlement_amount", genJnlLine.Source_curr_tax_amount)
	} else {
		run.SetValue("settlement_base", genJnlLine.Tax_base_amount)
		run.SetValue("settlement_amount", genJnlLine.Tax_amount)
	}
	run.SetValue("tax_calculation_type", setup.GetOptionCaption("tax_calculation_type", i18n.Key(setup.Tax_calculation_type.String()), fcodeunits.CurrentLanguage()))
	if r.post {
		err = r.genJnlPostLine.Run(genJnlLine)
		if err != nil {
			return err
		}
	}
	if reverseAccount != "" {
		genJnlLine2 := r.newSettlementLine(run)
		genJnlLine2.Description = genJnlLine.Description
		if setup.Tax_calculation_type == gtables.TaxPostingSetup_Tax_calculation_type.SalesTax {
			genJnlLine2.Tax_area_code = genJnlLine.Tax_area_code
			genJnlLine2.Use_tax = genJnlLine.Use_tax
		}
		genJnlLine2.Account_no = reverseAccount
		err = genJnlLine2.Validate("amount", r.taxEntry.Amount)
		if err != nil {
			return err
		}
		genJnlLine2.Source_currency_code = r.glSetup.Additional_reporting_currency
		genJnlLine2.Source_currency_amount = r.taxEntry.Additional_currency_amount
		if r.useAmtsInAddCurr {
			run.SetValue("reversing_amount", genJnlLine2.Source_currency_amount)
		} else {
			run.SetValue("reversing_amount", genJnlLine2.Amount)
		}
		run.SetValue("reversing_entry", true)
		if r.post {
			err = r.genJnlPostLine.Run(genJnlLine2)
			if err != nil {
				return err
			}
		}
	}
	if addToSettlement {
		r.taxAmount = r.taxAmount.Add(r.taxEntry.Amount)
		r.taxAmountAddCurr = r.taxAmountAddCurr.Add(r.taxEntry.Additional_currency_amount)
		if r.useAmtsInAddCurr {
			run.SetValue("tax_amount", r.taxAmountAddCurr)
		} else {
			run.SetValue("tax_amount", r.taxAmount)
		}
	}
	r.nextTaxEntryNo++
	run.SetValue("next_tax_entry_no", r.nextTaxEntryNo)

	// Close current tax entries
	if !r.post {
		return nil
	}
	var entryNos []int
	for found := r.taxEntry.FindSet(); found; found = r.taxEntry.Next() {
		entryNos = append(entryNos, r.taxEntry.Entry_no)
	}
	taxEntry2 := gtables.NewTaxEntry(run)
	for _, entryNo := range entryNos {
		err = taxEntry2.GetOrError(entryNo)
		if err != nil {
			return err
		}
		taxEntry2.Closed_by_entry_no = r.nextTaxEntryNo
		taxEntry2.Closed = true
		err = taxEntry2.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}

// newSettlementLine: a system-created G/L line of the settlement (Manual VAT Entry)
func (r *calcAndPostTaxSettlement) newSettlementLine(run *Run) *gtables.GenJournalLine {
	genJnlLine := gtables.NewGenJournalLine(run)
	genJnlLine.Init()
	genJnlLine.System_created_entry = true
	genJnlLine.Account_type = gtables.GenJournalLine_Account_type.GLAccount
	genJnlLine.Posting_date = r.postingDate
	genJnlLine.Document_type = gtables.GenJournalLine_Document_type.Blank
	genJnlLine.Document_no = r.docNo
	genJnlLine.Source_code = r.sourceCodeSetup.Tax_settlement
	genJnlLine.Vat_posting = gtables.GenJournalLine_Vat_posting.ManualVATEntry
	return genJnlLine
}
