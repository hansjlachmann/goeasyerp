package demodata

import (
	"fmt"
	"time"

	"github.com/hansjlachmann/openerp/backend/business-logic/fadepr"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The open journals of the demo (Options.Journals, codeunit 901): every journal of the main menu
// has lines ready to post, dated today — nothing is posted:
//   - General Journals: office supplies paid in cash; Recurring General Journals: the monthly
//     rent paid from the bank, allocated to the departments, and the salaries accrued at the
//     month's end (reversed the day after); Sales Journals: a service invoice to a
//     customer; Purchase Journals: a vendor's invoice; Cash Receipt Journals: a customer's payment
//     of an open invoice; Payment Journals: the payment of an open vendor invoice;
//   - Fixed Asset G/L Journals (template ASSETS): this month's depreciation of the assets, as
//     Calculate Depreciation would fill it (FA Journal Setup of the COMPANY book);
//   - Fixed Asset Journals: the acquisitions of three assets in the TAX book (they get the book);
//     FA Reclass. Journals: a share of the office furniture moved to the pallet racking; Insurance
//     Journals: the coverage of the laptops;
//   - Item Journals: adjustments (a carton of pens found); Transfer Journals: a transfer to the
//     Oslo store; Phys. Inventory Journals: counted quantities; Resource Journals: usage; BOM
//     Journals: an assembly of office kits (their pens are sold out: posted after the Item
//     Journal);
//   - a depreciation table and resource price changes;
//   - Bank Account Reconciliations: this month's statement of bank B010, its lines applied to the
//     open entries as Suggest Lines makes them (not posted).

// demoJournals fills the journals; the lines are dated today
type demoJournals struct {
	src   ftables.Source
	today types.Date
	lines int
}

func insertDemoJournals(src ftables.Source, today time.Time, counts *Counts, p *progress) error {
	j := &demoJournals{src: src, today: types.NewDateFromTime(today)}
	steps := []struct {
		name string
		fn   func() error
	}{
		{"general journals", j.generalJournals},
		{"recurring journal", j.recurringJournal},
		{"bank reconciliation", j.bankReconciliation},
		{"fixed asset journals", j.fixedAssetJournals},
		{"item journals", j.itemJournals},
		{"resource journals", j.resourceJournals},
		{"BOM journal", j.bomJournal},
		{"depreciation table", j.depreciationTable},
		{"resource price changes", j.resourcePriceChanges},
	}
	p.setTotal(len(steps))
	for _, step := range steps {
		err := step.fn()
		if err != nil {
			return fmt.Errorf("demo %s: %w", step.name, err)
		}
		p.step()
	}
	counts.JournalLines = j.lines
	return nil
}

// set validates fields one after another (the first error stops)
func set(rec interface {
	Validate(field string, value interface{}) error
}, values ...interface{}) error {
	for i := 0; i+1 < len(values); i += 2 {
		err := rec.Validate(values[i].(string), values[i+1])
		if err != nil {
			return fmt.Errorf("%s: %w", values[i], err)
		}
	}
	return nil
}

// genJnlLine: a new line of a general journal batch (SetUpNewLine), dated today
func (j *demoJournals) genJnlLine(template types.Code, lineNo int, last *gtables.GenJournalLine) (*gtables.GenJournalLine, error) {
	l := gtables.NewGenJournalLine(j.src)
	l.Journal_template_name = template
	l.Journal_batch_name = "DEFAULT"
	l.Line_no = lineNo
	if last == nil {
		last = gtables.NewGenJournalLine(j.src)
	}
	err := tables.SetUpNewGenJnlLine(l, last, types.Decimal{}, true)
	if err != nil {
		return nil, err
	}
	l.Posting_date = j.today
	l.Document_date = j.today
	return l, nil
}

func (j *demoJournals) insertGenJnlLine(l *gtables.GenJournalLine) error {
	err := l.InsertOrError(true)
	if err == nil {
		j.lines++
	}
	return err
}

// openCustInvoice / openVendInvoice: the first open invoice of a customer or vendor (a payment
// applies to it)
func (j *demoJournals) openCustInvoice() *gtables.CustomerLedgerEntry {
	e := gtables.NewCustomerLedgerEntry(j.src)
	e.SetRange("open", true)
	e.SetRange("document_type", gtables.CustomerLedgerEntry_Document_type.Invoice)
	if e.FindFirst() {
		return e
	}
	return nil
}

func (j *demoJournals) openVendInvoice() *gtables.VendorLedgerEntry {
	e := gtables.NewVendorLedgerEntry(j.src)
	e.SetRange("open", true)
	e.SetRange("document_type", gtables.VendorLedgerEntry_Document_type.Invoice)
	if e.FindFirst() {
		return e
	}
	return nil
}

func (j *demoJournals) generalJournals() error {
	// General: office supplies paid in cash
	l, err := j.genJnlLine("GENERAL", 10000, nil)
	if err != nil {
		return err
	}
	err = set(l, "account_type", gtables.GenJournalLine_Account_type.GLAccount, "account_no", types.Code("5410"),
		"amount", types.NewDecimalFromInt(850), "bal_account_type", gtables.GenJournalLine_Bal_account_type.GLAccount, "bal_account_no", types.Code("1510"))
	if err != nil {
		return err
	}
	l.Description = "Office supplies, cash"
	l.Department_code = "ADM"
	err = j.insertGenJnlLine(l)
	if err != nil {
		return err
	}

	// Sales: a service invoice to the first domestic customer
	cust := gtables.NewCustomer(j.src)
	cust.SetRange("customer_posting_group", types.Code("DOMESTIC"))
	if cust.FindFirst() {
		l, err = j.genJnlLine("SALES", 10000, nil)
		if err != nil {
			return err
		}
		err = set(l, "document_type", gtables.GenJournalLine_Document_type.Invoice, "account_type", gtables.GenJournalLine_Account_type.Customer,
			"account_no", cust.No, "amount", types.NewDecimalFromInt(12500), "bal_account_type", gtables.GenJournalLine_Bal_account_type.GLAccount,
			"bal_account_no", types.Code("3130"))
		if err != nil {
			return err
		}
		l.Description = "Consulting, workshop"
		err = j.insertGenJnlLine(l)
		if err != nil {
			return err
		}
	}

	// Purchases: an invoice from the furniture vendor
	l, err = j.genJnlLine("PURCHASES", 10000, nil)
	if err != nil {
		return err
	}
	err = set(l, "document_type", gtables.GenJournalLine_Document_type.Invoice, "account_type", gtables.GenJournalLine_Account_type.Vendor,
		"account_no", types.Code("V00030"), "amount", types.NewDecimalFromInt(-5600), "bal_account_type", gtables.GenJournalLine_Bal_account_type.GLAccount,
		"bal_account_no", types.Code("5410"))
	if err != nil {
		return err
	}
	l.External_document_no = "INV-88231"
	l.Description = "Office chairs"
	err = j.insertGenJnlLine(l)
	if err != nil {
		return err
	}

	// Cash receipts: a customer pays an open invoice into the bank (the batch's balancing account)
	custLedgEntry := j.openCustInvoice()
	if custLedgEntry != nil {
		custLedgEntry.CalcFields("remaining_amount")
		l, err = j.genJnlLine("CASHRCPT", 10000, nil)
		if err != nil {
			return err
		}
		err = set(l, "document_type", gtables.GenJournalLine_Document_type.Payment, "account_type", gtables.GenJournalLine_Account_type.Customer,
			"account_no", custLedgEntry.Customer_no, "applies_to_doc_type", gtables.GenJournalLine_Applies_to_doc_type.Invoice,
			"applies_to_doc_no", custLedgEntry.Document_no)
		if err != nil {
			return err
		}
		if l.Amount.IsZero() {
			err = set(l, "amount", custLedgEntry.Remaining_amount.Neg())
			if err != nil {
				return err
			}
		}
		err = j.insertGenJnlLine(l)
		if err != nil {
			return err
		}
	}

	// Payments: an open vendor invoice paid from the bank
	vendLedgEntry := j.openVendInvoice()
	if vendLedgEntry != nil {
		vendLedgEntry.CalcFields("remaining_amount")
		l, err = j.genJnlLine("PAYMENT", 10000, nil)
		if err != nil {
			return err
		}
		err = set(l, "document_type", gtables.GenJournalLine_Document_type.Payment, "account_type", gtables.GenJournalLine_Account_type.Vendor,
			"account_no", vendLedgEntry.Vendor_no, "applies_to_doc_type", gtables.GenJournalLine_Applies_to_doc_type.Invoice,
			"applies_to_doc_no", vendLedgEntry.Document_no)
		if err != nil {
			return err
		}
		if l.Amount.IsZero() {
			err = set(l, "amount", vendLedgEntry.Remaining_amount.Neg())
			if err != nil {
				return err
			}
		}
		err = j.insertGenJnlLine(l)
		if err != nil {
			return err
		}
	}
	return nil
}

func (j *demoJournals) fixedAssetJournals() error {
	// The Fixed Asset G/L Journal and the FA Journal Setup of both books
	tpl := gtables.NewGenJournalTemplate(j.src)
	tpl.Name = "ASSETS"
	tpl.Description = "Fixed Asset G/L Journal"
	err := tpl.Validate("type", gtables.GenJournalTemplate_Type.Assets)
	if err != nil {
		return err
	}
	err = tpl.InsertOrError(true)
	if err != nil {
		return err
	}
	batch := gtables.NewGenJournalBatch(j.src)
	batch.Journal_template_name = "ASSETS"
	batch.Name = "DEFAULT"
	batch.Description = "Default Journal"
	err = batch.InsertOrError(true)
	if err != nil {
		return err
	}
	for _, s := range []struct {
		book, faTemplate, genTemplate, insTemplate types.Code
	}{{"COMPANY", "ASSETS", "ASSETS", "INSURANCE"}, {"TAX", "ASSETS", "", ""}} {
		setup := gtables.NewFAJournalSetup(j.src)
		setup.Depreciation_book_code = s.book
		setup.Fa_jnl_template_name, setup.Fa_jnl_batch_name = s.faTemplate, "DEFAULT"
		if s.genTemplate != "" {
			setup.Gen_jnl_template_name, setup.Gen_jnl_batch_name = s.genTemplate, "DEFAULT"
		}
		if s.insTemplate != "" {
			setup.Insurance_jnl_template_name, setup.Insurance_jnl_batch_name = s.insTemplate, "DEFAULT"
		}
		err = setup.InsertOrError(true)
		if err != nil {
			return err
		}
	}

	// This month's depreciation of the COMPANY book (Calculate Depreciation)
	monthEnd := time.Date(j.today.Year(), j.today.Month()+1, 0, 0, 0, 0, 0, time.Local)
	faDeprBook := gtables.NewFADepreciationBook(j.src)
	faDeprBook.SetRange("depreciation_book_code", types.Code("COMPANY"))
	lineNo := 0
	var last *gtables.GenJournalLine
	for found := faDeprBook.FindSet(); found; found = faDeprBook.Next() {
		if !faDeprBook.Disposal_date.IsZero() || faDeprBook.Acquisition_date.IsZero() {
			continue
		}
		result, err := fadepr.Calculate(j.src, faDeprBook.Fa_no, "COMPANY", types.NewDateFromTime(monthEnd), fadepr.EntryAmounts{}, types.Date{}, 0)
		if err != nil {
			return err
		}
		if result.DeprAmount.IsZero() {
			continue
		}
		lineNo += 10000
		l, err := j.genJnlLine("ASSETS", lineNo, last)
		if err != nil {
			return err
		}
		l.Posting_date = types.NewDateFromTime(monthEnd)
		l.Document_date = l.Posting_date
		l.Document_no = types.NewCode(monthEnd.Format("DEPR2006-01"))
		err = set(l, "account_type", gtables.GenJournalLine_Account_type.FixedAsset, "account_no", faDeprBook.Fa_no,
			"depreciation_book_code", types.Code("COMPANY"), "fa_posting_type", gtables.GenJournalLine_Fa_posting_type.Depreciation,
			"amount", result.DeprAmount, "bal_account_type", gtables.GenJournalLine_Bal_account_type.GLAccount,
			"bal_account_no", depreciationExpenseAccount(j.src, faDeprBook.Fa_posting_group))
		if err != nil {
			return err
		}
		l.No_of_depreciation_days = result.NumberOfDays
		l.Description = types.NewText("Depreciation " + monthEnd.Format("01/2006"))
		err = j.insertGenJnlLine(l)
		if err != nil {
			return err
		}
		last = l
	}

	// FA Journal: the vehicles and the forklift acquired in the TAX book too
	lineNo = 0
	var lastFA *gtables.FAJournalLine
	for _, faNo := range []types.Code{"FA0010", "FA0020", "FA0060"} {
		company := gtables.NewFADepreciationBook(j.src)
		if !company.Get(map[string]interface{}{"fa_no": faNo, "depreciation_book_code": types.Code("COMPANY")}) {
			continue
		}
		company.CalcFields("acquisition_cost")
		tax := gtables.NewFADepreciationBook(j.src)
		tax.Fa_no = faNo
		tax.Depreciation_book_code = "TAX"
		err = tax.InsertOrError(true)
		if err == nil {
			err = set(tax, "fa_posting_group", company.Fa_posting_group, "depreciation_starting_date", company.Depreciation_starting_date,
				"no_of_depreciation_years", types.NewDecimalFromInt(4))
		}
		if err == nil {
			err = tax.ModifyOrError(true)
		}
		if err != nil {
			return err
		}
		lineNo += 10000
		l := gtables.NewFAJournalLine(j.src)
		l.Journal_template_name = "ASSETS"
		l.Journal_batch_name = "DEFAULT"
		l.Line_no = lineNo
		if lastFA == nil {
			lastFA = gtables.NewFAJournalLine(j.src)
		}
		err = tables.SetUpNewFAJnlLine(l, lastFA)
		if err != nil {
			return err
		}
		l.Fa_posting_date = company.Acquisition_date
		l.Document_no = types.NewCode("TAX-" + faNo.String())
		err = set(l, "fa_no", faNo, "depreciation_book_code", types.Code("TAX"), "fa_posting_type", gtables.FAJournalLine_Fa_posting_type.AcquisitionCost,
			"amount", company.Acquisition_cost)
		if err == nil {
			err = l.InsertOrError(true)
		}
		if err != nil {
			return err
		}
		j.lines++
		lastFA = l
	}

	// FA Reclass. Journal: a tenth of the office furniture to the pallet racking
	reclass := gtables.NewFAReclassJournalLine(j.src)
	reclass.Journal_template_name = "RECLASSIFY"
	reclass.Journal_batch_name = "DEFAULT"
	reclass.Line_no = 10000
	reclass.Fa_posting_date = j.today
	reclass.Document_no = "RECL-0001"
	err = set(reclass, "fa_no", types.Code("FA0050"), "new_fa_no", types.Code("FA0040"), "reclassify_acq_cost_percent", types.NewDecimalFromInt(10))
	if err != nil {
		return err
	}
	reclass.Reclassify_acquisition_cost = true
	reclass.Reclassify_depreciation = true
	reclass.Insert_bal_account = true
	reclass.Description = "Shelving moved to the warehouse"
	err = reclass.InsertOrError(true)
	if err != nil {
		return err
	}
	j.lines++

	// Insurance Journal: the laptops covered by the insurance
	ins := gtables.NewInsuranceJournalLine(j.src)
	ins.Journal_template_name = "INSURANCE"
	ins.Journal_batch_name = "DEFAULT"
	ins.Line_no = 10000
	err = tables.SetUpNewInsuranceJnlLine(ins, gtables.NewInsuranceJournalLine(j.src))
	if err != nil {
		return err
	}
	ins.Posting_date = j.today
	ins.Document_no = "INS-FA0060"
	err = set(ins, "insurance_no", types.Code("INS010"), "fa_no", types.Code("FA0060"), "amount", types.NewDecimalFromInt(96000))
	if err == nil {
		err = ins.InsertOrError(true)
	}
	if err != nil {
		return err
	}
	j.lines++
	return nil
}

// itemJnlLine: a new line of an item journal batch (SetUpNewLine), dated today
func (j *demoJournals) itemJnlLine(template types.Code, lineNo int, last *gtables.ItemJournalLine) (*gtables.ItemJournalLine, error) {
	l := gtables.NewItemJournalLine(j.src)
	l.Journal_template_name = template
	l.Journal_batch_name = "DEFAULT"
	l.Line_no = lineNo
	if last == nil {
		last = gtables.NewItemJournalLine(j.src)
	}
	err := tables.SetUpNewItemJnlLine(l, last)
	if err != nil {
		return nil, err
	}
	l.Posting_date = j.today
	l.Document_date = j.today
	return l, nil
}

func (j *demoJournals) itemJournals() error {
	// Item Journal: a found carton of pens, a damaged notebook pack
	var last *gtables.ItemJournalLine
	for i, adj := range []struct {
		entryType gtables.ItemJournalLineEntry_type
		item      types.Code
		quantity  int64
	}{{gtables.ItemJournalLine_Entry_type.PositiveAdjmt, "1100", 50}, {gtables.ItemJournalLine_Entry_type.NegativeAdjmt, "1120", 6}} {
		l, err := j.itemJnlLine("ITEM", (i+1)*10000, last)
		if err != nil {
			return err
		}
		err = set(l, "entry_type", adj.entryType, "item_no", adj.item, "location_code", types.Code("MAIN"), "quantity", types.NewDecimalFromInt(adj.quantity))
		if err == nil {
			err = l.InsertOrError(true)
		}
		if err != nil {
			return err
		}
		j.lines++
		last = l
	}

	// Transfer Journal: markers to the Oslo store
	l, err := j.itemJnlLine("TRANSFER", 10000, nil)
	if err != nil {
		return err
	}
	err = set(l, "entry_type", gtables.ItemJournalLine_Entry_type.Transfer, "item_no", types.Code("1110"), "location_code", types.Code("MAIN"),
		"new_location_code", types.Code("OSLO"), "quantity", types.NewDecimalFromInt(10))
	if err == nil {
		err = l.InsertOrError(true)
	}
	if err != nil {
		return err
	}
	j.lines++

	// Phys. Inventory Journal: the components counted in the main warehouse, one short
	last = nil
	for i, itemNo := range []types.Code{"1100", "1110", "1120"} {
		itemLedgEntry := gtables.NewItemLedgerEntry(j.src)
		itemLedgEntry.SetRange("item_no", itemNo)
		itemLedgEntry.SetRange("location_code", types.Code("MAIN"))
		err := itemLedgEntry.CalcSums("quantity")
		if err != nil {
			return err
		}
		l, err := j.itemJnlLine("PHYS. INV.", (i+1)*10000, last)
		if err != nil {
			return err
		}
		err = set(l, "item_no", itemNo, "location_code", types.Code("MAIN"))
		if err != nil {
			return err
		}
		l.Phys_inventory = true
		err = set(l, "qty_calculated", itemLedgEntry.Quantity, "qty_phys_inventory", itemLedgEntry.Quantity.Sub(types.NewDecimalFromInt(int64(i))))
		if err == nil {
			err = l.InsertOrError(true)
		}
		if err != nil {
			return err
		}
		j.lines++
		last = l
	}
	return nil
}

func (j *demoJournals) resourceJournals() error {
	var last *gtables.ResJournalLine
	for i, usage := range []struct {
		resource, workType types.Code
		hours              int64
	}{{"R0010", "NORMAL", 7}, {"R0020", "OVERTIME", 3}} {
		l := gtables.NewResJournalLine(j.src)
		l.Journal_template_name = "RESOURCES"
		l.Journal_batch_name = "DEFAULT"
		l.Line_no = (i + 1) * 10000
		if last == nil {
			last = gtables.NewResJournalLine(j.src)
		}
		err := tables.SetUpNewResJnlLine(l, last)
		if err != nil {
			return err
		}
		l.Posting_date = j.today
		if l.Document_no == "" {
			l.Document_no = types.NewCode(fmt.Sprintf("RES-%s", j.today.Format("20060102")))
		}
		err = set(l, "entry_type", gtables.ResJournalLine_Entry_type.Usage, "resource_no", usage.resource, "work_type_code", usage.workType,
			"quantity", types.NewDecimalFromInt(usage.hours))
		if err == nil {
			err = l.InsertOrError(true)
		}
		if err != nil {
			return err
		}
		j.lines++
		last = l
	}
	return nil
}

func (j *demoJournals) bomJournal() error {
	l := gtables.NewBOMJournalLine(j.src)
	l.Journal_template_name = "BOM"
	l.Journal_batch_name = "DEFAULT"
	l.Line_no = 10000
	err := tables.SetUpNewBOMJnlLine(l, gtables.NewBOMJournalLine(j.src))
	if err != nil {
		return err
	}
	l.Posting_date = j.today
	err = set(l, "item_no", types.Code("1500"), "location_code", types.Code("MAIN"), "quantity", types.NewDecimalFromInt(5))
	if err == nil {
		err = l.InsertOrError(true)
	}
	if err != nil {
		return err
	}
	j.lines++
	return nil
}

// depreciationTable: a declining table over five years (user-defined depreciation)
func (j *demoJournals) depreciationTable() error {
	header := gtables.NewDepreciationTableHeader(j.src)
	header.Code = "DECL-5Y"
	header.Description = "Declining over 5 years"
	header.Period_length = gtables.DepreciationTableHeader_Period_length.Year
	err := header.InsertOrError(true)
	if err != nil {
		return err
	}
	for i, percent := range []int64{30, 25, 20, 15, 10} {
		line := gtables.NewDepreciationTableLine(j.src)
		line.Depreciation_table_code = header.Code
		line.Period_no = i + 1
		err = set(line, "period_depreciation_percent", types.NewDecimalFromInt(percent))
		if err == nil {
			err = line.InsertOrError(true)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// resourcePriceChanges: the consultants' new prices (Suggest/Implement Res. Price Change)
func (j *demoJournals) resourcePriceChanges() error {
	for _, c := range []struct {
		resource types.Code
		current  int64
		newPrice int64
	}{{"R0010", 1250, 1350}, {"R0020", 1050, 1120}} {
		change := gtables.NewResourcePriceChange(j.src)
		change.Type = gtables.ResourcePriceChange_Type.Resource
		err := set(change, "code", c.resource)
		if err != nil {
			return err
		}
		change.Current_unit_price = types.NewDecimalFromInt(c.current)
		change.New_unit_price = types.NewDecimalFromInt(c.newPrice)
		err = change.InsertOrError(true)
		if err != nil {
			return err
		}
	}
	return nil
}

// recurringJournal: template RECURRING — the rent, fixed each month, paid from bank B010 and
// allocated to the departments by their floor space (5:3:2), and the salary accrual of the month,
// variable, reversed the day after
func (j *demoJournals) recurringJournal() error {
	l, err := j.genJnlLine("RECURRING", 10000, nil)
	if err != nil {
		return err
	}
	l.Document_no = "RENT-%3"
	err = set(l, "recurring_method", gtables.GenJournalLine_Recurring_method.FFixed, "account_type", gtables.GenJournalLine_Account_type.BankAccount,
		"account_no", types.Code("B010"))
	if err != nil {
		return err
	}
	l.Recurring_frequency = "1M"
	l.Description = "Office rent %4"
	err = j.insertGenJnlLine(l)
	if err == nil {
		err = l.Validate("amount", types.NewDecimalFromInt(-45000))
	}
	if err == nil {
		err = l.ModifyOrError(true)
	}
	if err != nil {
		return err
	}
	for i, d := range []struct {
		department types.Code
		quantity   int64
	}{{"ADM", 5}, {"SALES", 3}, {"PROD", 2}} {
		err = j.allocation(l, (i+1)*10000, "5210", d.department, func(a *gtables.GenJnlAllocation) error {
			return a.Validate("allocation_quantity", types.NewDecimalFromInt(d.quantity))
		})
		if err != nil {
			return err
		}
	}

	l2, err := j.genJnlLine("RECURRING", 20000, l)
	if err != nil {
		return err
	}
	l2.Document_no = "ACCR-%3"
	err = set(l2, "recurring_method", gtables.GenJournalLine_Recurring_method.RVReversingVariable, "account_type", gtables.GenJournalLine_Account_type.GLAccount,
		"account_no", types.Code("2360"))
	if err != nil {
		return err
	}
	l2.Recurring_frequency = "1M"
	l2.Description = "Accrued salaries %4"
	err = j.insertGenJnlLine(l2)
	if err == nil {
		err = l2.Validate("amount", types.NewDecimalFromInt(-38000))
	}
	if err == nil {
		err = l2.ModifyOrError(true)
	}
	if err != nil {
		return err
	}
	return j.allocation(l2, 10000, "5110", "", func(a *gtables.GenJnlAllocation) error {
		return a.Validate("allocation_percent", types.NewDecimalFromInt(100))
	})
}

// allocation: an allocation of a journal line to a G/L account (and department)
func (j *demoJournals) allocation(l *gtables.GenJournalLine, lineNo int, accNo, department types.Code, fn func(a *gtables.GenJnlAllocation) error) error {
	a := gtables.NewGenJnlAllocation(j.src)
	a.Journal_template_name = l.Journal_template_name
	a.Journal_batch_name = l.Journal_batch_name
	a.Journal_line_no = l.Line_no
	a.Line_no = lineNo
	err := a.Validate("account_no", accNo)
	if err != nil {
		return err
	}
	if department != "" {
		a.Department_code = department
	}
	err = a.InsertOrError(true)
	if err == nil {
		err = fn(a)
	}
	if err == nil {
		err = a.ModifyOrError(true)
	}
	return err
}

// bankReconciliation: the statement after B010's last one, dated today, a line applied to each
// open bank account ledger entry up to today (report 1496 Suggest Lines), the ending balance
// matching — ready to post
func (j *demoJournals) bankReconciliation() error {
	recon := gtables.NewBankAccReconciliation(j.src)
	err := recon.Validate("bank_account_no", types.Code("B010"))
	if err != nil {
		return err
	}
	recon.Statement_date = j.today
	err = recon.InsertOrError(true)
	if err != nil {
		return err
	}
	entry := gtables.NewBankAccountLedgerEntry(j.src)
	entry.SetCurrentKey("bank_account_no", "posting_date")
	entry.SetRange("bank_account_no", recon.Bank_account_no)
	entry.SetRange("posting_date", types.Date{}, j.today)
	entry.SetRange("open", true)
	entry.SetRange("statement_status", gtables.BankAccountLedgerEntry_Statement_status.Open)
	var entries []*gtables.BankAccountLedgerEntry
	for found := entry.FindSetBuffered(); found; found = entry.Next() {
		e := gtables.NewBankAccountLedgerEntry(j.src)
		e.Copy(entry)
		entries = append(entries, e)
	}
	total := types.Decimal{}
	for i, e := range entries {
		line := gtables.NewBankAccReconciliationLine(j.src)
		line.Bank_account_no = recon.Bank_account_no
		line.Statement_no = recon.Statement_no
		line.Statement_line_no = (i + 1) * 10000
		line.Transaction_date = e.Posting_date
		line.Description = e.Description
		line.Document_no = e.Document_no
		line.Statement_amount = e.Remaining_amount
		line.Applied_amount = e.Remaining_amount
		line.Type = gtables.BankAccReconciliationLine_Type.BankAccountLedgerEntry
		line.Applied_entries = 1
		err = tables.SetBankAccReconNo(e, line)
		if err == nil {
			err = line.InsertOrError(false)
		}
		if err != nil {
			return err
		}
		total = total.Add(e.Remaining_amount)
	}
	recon.Statement_ending_balance = recon.Balance_last_statement.Add(total)
	return recon.ModifyOrError(false)
}
