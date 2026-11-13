package codeunits

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/glcheck"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchposting"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesposting"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/taxstatement"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/company"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The VAT checklist (steps 3–9) on the VAT demo company (routine 901 with VAT): purchases,
// general journal, the VAT return, the settlement, the EU reports, EU 3-party trade, unrealized
// VAT and the payment discount VAT adjustment. The steps run in order on one company, as a
// user goes through them: each step's documents are dated in the demo month and the VAT return
// and settlement see the steps before them.
func TestVATChecklist(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	runVATChecklist(t, db, database.DBTypeSQLite, "vatcheck")
}

// The VAT checklist on PostgreSQL (production's database)
func TestVATChecklistPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	name := fmt.Sprintf("vatcheck%d", time.Now().UnixNano()%1000000)
	t.Cleanup(func() {
		tx, err := db.Begin()
		if err == nil {
			_ = company.DropObjects(tx, database.DBTypePostgres, name)
			_ = tx.Commit()
		}
	})
	runVATChecklist(t, db, database.DBTypePostgres, name)
}

// the demo month: the demo's "today" and the dates the checklist posts on
var (
	vatDemoToday   = time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local)
	vatPeriod      = "2026-10-01..2026-10-31"
	vatPostingDate = types.NewDate(2026, 10, 20)
)

func runVATChecklist(t *testing.T, db *sql.DB, dbType database.DBType, companyName string) {
	if err := tables.CreateCompanyTables(db, companyName, dbType); err != nil {
		t.Fatal(err)
	}
	opts := demoDataOptions(demodata.Small, true, nil)
	opts.Today = vatDemoToday
	if _, err := demodata.Create(db, companyName, dbType, opts); err != nil {
		t.Fatal(err)
	}
	c := &vatCheck{t: t, src: ftables.DBSource(db, companyName, dbType)}

	steps := []struct {
		name string
		fn   func(c *vatCheck)
	}{
		{"3 purchases", (*vatCheck).purchases},
		{"4 general journal", (*vatCheck).generalJournal},
		{"7b EU 3-party trade", (*vatCheck).eu3PartyTrade},
		{"7 EU reports", (*vatCheck).euReports},
		{"5 VAT return", (*vatCheck).vatReturn},
		{"6 settlement", (*vatCheck).settlement},
		{"8 unrealized VAT", (*vatCheck).unrealizedVAT},
		{"9 payment discount VAT", (*vatCheck).paymentDiscountVAT},
	}
	for _, step := range steps {
		ok := t.Run(step.name, func(t *testing.T) {
			c.t = t
			step.fn(c)
		})
		c.t = t
		if !ok {
			return // the later steps build on this one
		}
	}
	if r := glcheck.Check(c.src, nil); !r.Balanced() {
		t.Errorf("G/L Balance Check after the checklist: %+v", r)
	}
}

type vatCheck struct {
	t   *testing.T
	src ftables.Source

	vatPayable types.Decimal // row 30 of the month's open entries (step 5), settled in step 6
}

func (c *vatCheck) must(err error) {
	c.t.Helper()
	if err != nil {
		c.t.Fatal(err)
	}
}

func (c *vatCheck) want(what string, got, want types.Decimal) {
	c.t.Helper()
	if !got.Equal(want) {
		c.t.Errorf("%s: %s, want %s", what, got, want)
	}
}

func vd(s string) types.Decimal { return types.MustDecimal(s) }

// gl: the net change of the accounts (a filter) on the documents (a filter, "" = all) in the
// demo month
func (c *vatCheck) gl(accounts, documents string) types.Decimal {
	c.t.Helper()
	glEntry := gtables.NewGLEntry(c.src)
	glEntry.SetFilter("gl_account_no", accounts)
	if documents != "" {
		glEntry.SetFilter("document_no", documents)
	}
	glEntry.SetFilter("posting_date", vatPeriod)
	c.must(glEntry.CalcSums("amount"))
	return glEntry.Amount.Round(2)
}

// historyVAT: the month's G/L amount on VAT accounts (a filter) of documents without tax
// entries — the demo's generated G/L history, inserted directly with its VAT on 2330/1440
func (c *vatCheck) historyVAT(accounts string) types.Decimal {
	c.t.Helper()
	total := types.Decimal{}
	glEntry := gtables.NewGLEntry(c.src)
	glEntry.SetFilter("gl_account_no", accounts)
	glEntry.SetFilter("posting_date", vatPeriod)
	for found := glEntry.FindSetBuffered(); found; found = glEntry.Next() {
		taxEntry := gtables.NewTaxEntry(c.src)
		taxEntry.SetRange("document_no", glEntry.Document_no)
		taxEntry.SetRange("posting_date", glEntry.Posting_date)
		if taxEntry.IsEmpty() {
			total = total.Add(glEntry.Amount)
		}
	}
	return total.Round(2)
}

// taxEntries: the tax entries of a document
func (c *vatCheck) taxEntries(documentNo types.Code) []*gtables.TaxEntry {
	taxEntry := gtables.NewTaxEntry(c.src)
	taxEntry.SetRange("document_no", documentNo)
	var entries []*gtables.TaxEntry
	for found := taxEntry.FindSet(); found; found = taxEntry.Next() {
		e := gtables.NewTaxEntry(c.src)
		e.Copy(taxEntry)
		entries = append(entries, e)
	}
	return entries
}

// oneTaxEntry: the one tax entry of a document
func (c *vatCheck) oneTaxEntry(documentNo types.Code) *gtables.TaxEntry {
	c.t.Helper()
	entries := c.taxEntries(documentNo)
	if len(entries) != 1 {
		c.t.Fatalf("%s: %d tax entries, want 1", documentNo, len(entries))
	}
	return entries[0]
}

// firstVendor / firstCustomer: a demo vendor/customer in the local currency with the tax
// business posting group
func (c *vatCheck) firstVendor(taxBus string) types.Code {
	c.t.Helper()
	vend := gtables.NewVendor(c.src)
	vend.SetRange("tax_bus_posting_group", types.Code(taxBus))
	vend.SetRange("currency_code", types.Code(""))
	vend.SetRange("blocked", false)
	if !vend.FindFirst() {
		c.t.Fatalf("no demo vendor in %s without currency", taxBus)
	}
	return vend.No
}

func (c *vatCheck) firstCustomer(taxBus string) types.Code {
	c.t.Helper()
	cust := gtables.NewCustomer(c.src)
	cust.SetRange("tax_bus_posting_group", types.Code(taxBus))
	cust.SetRange("currency_code", types.Code(""))
	cust.SetRange("blocked", false)
	if !cust.FindFirst() {
		c.t.Fatalf("no demo customer in %s without currency", taxBus)
	}
	return cust.No
}

// purchaseInvoice posts a purchase invoice of one G/L account line (STANDARD) and returns the
// posted invoice no.
func (c *vatCheck) purchaseInvoice(vendorNo types.Code, account types.Code, cost, vendorInvoiceNo string) types.Code {
	c.t.Helper()
	purchHeader := gtables.NewPurchaseHeader(c.src)
	purchHeader.Document_type = gtables.PurchaseHeader_Document_type.Invoice
	c.must(purchHeader.InsertOrError(true))
	c.must(purchHeader.Validate("buy_from_vendor_no", vendorNo))
	c.must(purchHeader.Validate("posting_date", vatPostingDate))
	c.must(purchHeader.Validate("vendor_invoice_no", types.Code(vendorInvoiceNo)))
	c.must(purchHeader.ModifyOrError(true))
	purchLine := gtables.NewPurchaseLine(purchHeader)
	purchLine.Document_type = gtables.PurchaseLineDocument_type(purchHeader.Document_type)
	purchLine.Document_no = purchHeader.No
	purchLine.Line_no = 10000
	c.must(purchLine.Validate("type", gtables.PurchaseLine_Type.GLAccount))
	c.must(purchLine.Validate("no", account))
	c.must(purchLine.Validate("tax_prod_posting_group", types.Code("STANDARD")))
	c.must(purchLine.Validate("quantity", vd("1")))
	c.must(purchLine.Validate("direct_unit_cost", vd(cost)))
	c.must(purchLine.InsertOrError(true))
	purchPost := purchposting.NewPurchPost(purchHeader)
	c.must(purchPost.Run(purchHeader))
	if purchPost.InvoiceNo == "" {
		c.t.Fatal("no posted purchase invoice")
	}
	return purchPost.InvoiceNo
}

// vendorAmount: the vendor ledger entry's amount (LCY) of a document
func (c *vatCheck) vendorAmount(vendorNo, documentNo types.Code) types.Decimal {
	c.t.Helper()
	entry := gtables.NewVendorLedgerEntry(c.src)
	entry.SetRange("vendor_no", vendorNo)
	entry.SetRange("document_no", documentNo)
	if !entry.FindFirst() {
		c.t.Fatalf("no vendor entry %s", documentNo)
	}
	entry.CalcFields("amount_lcy")
	return entry.Amount_lcy
}

// Step 3: a domestic purchase of 100 has 25 VAT on 1440, the vendor 125; a purchase of 1000
// from abroad has no VAT from the vendor, the reverse charge 250 on 1440 and back on 2345
func (c *vatCheck) purchases() {
	domestic := c.firstVendor("DOMESTIC")
	invoiceNo := c.purchaseInvoice(domestic, "4130", "100", "VATCHK-3A")
	c.want("domestic: 1440", c.gl("1440", invoiceNo.String()), vd("25"))
	c.want("domestic: vendor", c.vendorAmount(domestic, invoiceNo), vd("-125"))
	taxEntry := c.oneTaxEntry(invoiceNo)
	if taxEntry.Type != gtables.TaxEntry_Type.Purchase || taxEntry.Tax_calculation_type != gtables.TaxEntry_Tax_calculation_type.NormalVAT {
		c.t.Errorf("domestic tax entry: type %v calculation %v", taxEntry.Type, taxEntry.Tax_calculation_type)
	}
	c.want("domestic: tax entry", taxEntry.Amount, vd("25"))
	c.want("domestic: tax entry base", taxEntry.Base, vd("100"))

	export := c.firstVendor("EXPORT")
	invoiceNo = c.purchaseInvoice(export, "4120", "1000", "VATCHK-3B")
	c.want("abroad: vendor", c.vendorAmount(export, invoiceNo), vd("-1000"))
	c.want("abroad: 1440", c.gl("1440", invoiceNo.String()), vd("250"))
	c.want("abroad: 2345", c.gl("2345", invoiceNo.String()), vd("-250"))
	taxEntry = c.oneTaxEntry(invoiceNo)
	if taxEntry.Tax_calculation_type != gtables.TaxEntry_Tax_calculation_type.ReverseChargeVAT {
		c.t.Errorf("abroad tax entry: calculation %v", taxEntry.Tax_calculation_type)
	}
	c.want("abroad: tax entry", taxEntry.Amount, vd("250"))
	c.want("abroad: tax entry base", taxEntry.Base, vd("1000"))
}

// glLine: a general journal line on a G/L account with VAT, balanced by the bank account
func (c *vatCheck) glLine(documentNo, account, taxBus, amount string, eu3PartyTrade bool) *gtables.GenJournalLine {
	c.t.Helper()
	line := gtables.NewGenJournalLine(c.src)
	line.Init()
	c.must(line.Validate("posting_date", vatPostingDate))
	line.Document_no = types.Code(documentNo)
	c.must(line.Validate("account_type", gtables.GenJournalLine_Account_type.GLAccount))
	c.must(line.Validate("account_no", types.Code(account)))
	c.must(line.Validate("gen_posting_type", gtables.GenJournalLine_Gen_posting_type.Sale))
	c.must(line.Validate("tax_bus_posting_group", types.Code(taxBus)))
	c.must(line.Validate("tax_prod_posting_group", types.Code("STANDARD")))
	c.must(line.Validate("bal_account_type", gtables.GenJournalLine_Bal_account_type.BankAccount))
	c.must(line.Validate("bal_account_no", types.Code("B010")))
	c.must(line.Validate("amount", vd(amount)))
	line.Eu_3_party_trade = eu3PartyTrade
	return line
}

// postLine posts a journal line through codeunit 12 in a transaction of its own
func (c *vatCheck) postLine(line *gtables.GenJournalLine) {
	c.t.Helper()
	c.inTx(func(src ftables.Source) error {
		return genjnl.NewGenJnlPostLine(src).Run(line)
	})
}

func (c *vatCheck) inTx(fn func(src ftables.Source) error) {
	c.t.Helper()
	tx, err := ftables.BeginTransaction(c.src.GetDB())
	c.must(err)
	err = fn(ftables.DBSource(tx, c.src.GetCompany(), c.src.GetDBType()))
	if err != nil {
		_ = ftables.Rollback(tx)
		c.t.Fatal(err)
	}
	c.must(ftables.Commit(tx))
}

// Step 4: the demo's GENERAL journal posts its import VAT line (Full VAT 2500 on 1440, base
// 0); a sale of 125 on a G/L account with DOMESTIC/STANDARD splits into 100 and 25 VAT
func (c *vatCheck) generalJournal() {
	fullVAT := gtables.NewGenJournalLine(c.src)
	fullVAT.SetRange("journal_template_name", types.Code("GENERAL"))
	fullVAT.SetRange("tax_prod_posting_group", types.Code("FULL"))
	if !fullVAT.FindFirst() {
		c.t.Fatal("no Full VAT line in the GENERAL journal")
	}
	documentNo := fullVAT.Document_no
	c.inTx(func(src ftables.Source) error {
		line := gtables.NewGenJournalLine(src)
		line.SetRange("journal_template_name", types.Code("GENERAL"))
		line.SetRange("journal_batch_name", fullVAT.Journal_batch_name)
		line.FindFirst()
		_, err := genjnl.NewGenJnlPostBatch(src).Run(line)
		return err
	})
	taxEntry := gtables.NewTaxEntry(c.src)
	taxEntry.SetRange("document_no", documentNo)
	taxEntry.SetRange("tax_calculation_type", gtables.TaxEntry_Tax_calculation_type.FullVAT)
	if taxEntry.Count() != 1 || !taxEntry.FindFirst() {
		c.t.Fatalf("%d Full VAT entries of %s, want 1", taxEntry.Count(), documentNo)
	}
	c.want("Full VAT entry", taxEntry.Amount, vd("2500"))
	c.want("Full VAT entry base", taxEntry.Base, vd("0"))
	glEntry := gtables.NewGLEntry(c.src)
	glEntry.SetRange("document_no", documentNo)
	glEntry.SetRange("gl_account_no", types.Code("1440"))
	c.must(glEntry.CalcSums("amount"))
	c.want("import VAT on 1440", glEntry.Amount, vd("2500"))

	c.postLine(c.glLine("VATCHK-4", "3110", "DOMESTIC", "-125", false))
	c.want("sale: 3110", c.gl("3110", "VATCHK-4"), vd("-100"))
	c.want("sale: 2330", c.gl("2330", "VATCHK-4"), vd("-25"))
	sale := c.oneTaxEntry("VATCHK-4")
	if sale.Type != gtables.TaxEntry_Type.Sale || sale.Tax_calculation_type != gtables.TaxEntry_Tax_calculation_type.NormalVAT {
		c.t.Errorf("sale tax entry: type %v calculation %v", sale.Type, sale.Tax_calculation_type)
	}
	c.want("sale: tax entry", sale.Amount, vd("-25"))
	c.want("sale: tax entry base", sale.Base, vd("-100"))
}

// Step 7b: the demo's two sales to the Danish customer this month, the second an EU 3-party
// trade; a sales order and a journal line entered with EU 3-Party Trade carry it onto the
// posted invoice and the tax entries
func (c *vatCheck) eu3PartyTrade() {
	invoices := gtables.NewSalesInvoiceHeader(c.src)
	invoices.SetRange("sell_to_customer_no", types.Code("C00080"))
	invoices.SetFilter("posting_date", vatPeriod)
	var with, without []types.Code
	for found := invoices.FindSet(); found; found = invoices.Next() {
		if invoices.Eu_3_party_trade {
			with = append(with, invoices.No)
		} else {
			without = append(without, invoices.No)
		}
	}
	if len(with) != 1 || len(without) != 1 {
		c.t.Fatalf("Danish invoices this month: %v with and %v without EU 3-Party Trade, want one each", with, without)
	}
	c.checkEU3PartyEntries(with[0], true)
	c.checkEU3PartyEntries(without[0], false)

	// a sales order entered with EU 3-Party Trade
	salesHeader := gtables.NewSalesHeader(c.src)
	salesHeader.Document_type = gtables.SalesHeader_Document_type.Order
	c.must(salesHeader.InsertOrError(true))
	c.must(salesHeader.Validate("sell_to_customer_no", types.Code("C00080")))
	c.must(salesHeader.Validate("posting_date", vatPostingDate))
	c.must(salesHeader.Validate("eu_3_party_trade", true))
	c.must(salesHeader.ModifyOrError(true))
	salesLine := gtables.NewSalesLine(salesHeader)
	salesLine.Document_type = gtables.SalesLineDocument_type(salesHeader.Document_type)
	salesLine.Document_no = salesHeader.No
	salesLine.Line_no = 10000
	c.must(salesLine.Validate("type", gtables.SalesLine_Type.GLAccount))
	c.must(salesLine.Validate("no", types.Code("3110")))
	c.must(salesLine.Validate("quantity", vd("1")))
	c.must(salesLine.Validate("unit_price", vd("1000")))
	c.must(salesLine.InsertOrError(true))
	salesHeader.Ship = true
	salesHeader.Invoice = true
	salesPost := salesposting.NewSalesPost(salesHeader)
	c.must(salesPost.Run(salesHeader))
	posted := gtables.NewSalesInvoiceHeader(c.src)
	c.must(posted.GetOrError(salesPost.InvoiceNo))
	if !posted.Eu_3_party_trade {
		c.t.Errorf("posted invoice %s without EU 3-Party Trade", posted.No)
	}
	c.checkEU3PartyEntries(posted.No, true)

	// a general journal sale with EU 3-Party Trade
	c.postLine(c.glLine("VATCHK-7B", "3110", "EXPORT", "-400", true))
	c.checkEU3PartyEntries("VATCHK-7B", true)
	c.want("journal sale abroad: tax entry base", c.oneTaxEntry("VATCHK-7B").Base, vd("-400"))
}

// checkEU3PartyEntries: a document's sales tax entries — reverse charge, base only — carry EU
// 3-Party Trade or not
func (c *vatCheck) checkEU3PartyEntries(documentNo types.Code, eu3PartyTrade bool) {
	c.t.Helper()
	entries := c.taxEntries(documentNo)
	if len(entries) == 0 {
		c.t.Errorf("%s: no tax entries", documentNo)
	}
	for _, e := range entries {
		if e.Eu_3_party_trade != eu3PartyTrade || e.Type != gtables.TaxEntry_Type.Sale ||
			e.Tax_calculation_type != gtables.TaxEntry_Tax_calculation_type.ReverseChargeVAT || !e.Amount.IsZero() || e.Base.IsZero() {
			c.t.Errorf("%s tax entry %d: EU 3-party %v (want %v), type %v, calculation %v, amount %s, base %s", documentNo, e.Entry_no,
				e.Eu_3_party_trade, eu3PartyTrade, e.Type, e.Tax_calculation_type, e.Amount, e.Base)
		}
	}
}

// euSales: the month's sales tax entries per customer in an EU country other than the
// company's — the sales (the base, positive) and the EU 3-party trade part
type euSales struct {
	sales, eu3Party types.Decimal
	country         types.Code
}

func (c *vatCheck) euSales() map[types.Code]*euSales {
	companyInfo := gtables.GetCompanyInformation(c.src)
	taxEntry := gtables.NewTaxEntry(c.src)
	taxEntry.SetRange("type", gtables.TaxEntry_Type.Sale)
	taxEntry.SetFilter("posting_date", vatPeriod)
	result := map[types.Code]*euSales{}
	for found := taxEntry.FindSetBuffered(); found; found = taxEntry.Next() {
		country := gtables.NewCountryRegion(c.src)
		if taxEntry.Country_code == "" || taxEntry.Country_code == companyInfo.Country_code || !country.Get(taxEntry.Country_code) || country.Eu_country_code == "" {
			continue
		}
		s := result[taxEntry.Sell_to_buy_from_no]
		if s == nil {
			s = &euSales{country: taxEntry.Country_code}
			result[taxEntry.Sell_to_buy_from_no] = s
		}
		s.sales = s.sales.Sub(taxEntry.Base)
		if taxEntry.Eu_3_party_trade {
			s.eu3Party = s.eu3Party.Sub(taxEntry.Base)
		}
	}
	return result
}

// Step 7: VIES 19 (printed) and 88 (file) list the month's EU sales per customer with its VAT
// registration no. and the EU 3-party trade amount
func (c *vatCheck) euReports() {
	companyInfo := gtables.GetCompanyInformation(c.src)
	if companyInfo.Vat_registration_no == "" {
		companyInfo.Vat_registration_no = "NO123456789"
		c.must(companyInfo.ModifyOrError(false))
	}
	sales := c.euSales()
	dk := sales["C00080"]
	if dk == nil || !dk.eu3Party.IsPositive() || !dk.sales.GreaterThan(dk.eu3Party) {
		c.t.Fatalf("the Danish customer's EU sales this month: %+v", dk)
	}
	vatRegNos := map[types.Code]string{}
	for custNo := range sales {
		if custNo == "" {
			c.t.Fatal("an EU sale without a customer")
		}
		cust := gtables.NewCustomer(c.src)
		c.must(cust.GetOrError(custNo))
		vatRegNos[custNo] = fmt.Sprintf("%s%08d", sales[custNo].country, 12345670+len(vatRegNos))
		cust.Vat_registration_no = types.Text(vatRegNos[custNo])
		c.must(cust.ModifyOrError(false))
	}
	filters := map[string][]ftables.FieldFilter{"TaxEntry": {{Field: "posting_date", Expression: vatPeriod}}}

	result, err := reports.Execute(reports.VIESDeclarationTaxAuthID, c.src.GetDB(), c.src.GetCompany(), c.src.GetDBType(),
		reports.RunOptions{Output: reports.OutputPDF, Filters: filters})
	c.must(err)
	var texts []string
	for _, text := range result.Texts {
		texts = append(texts, text.Text)
	}
	printed := "|" + strings.Join(texts, "|") + "|"
	for custNo, s := range sales {
		for _, want := range []string{vatRegNos[custNo], amountText(s.sales)} {
			if !strings.Contains(printed, "|"+want+"|") {
				c.t.Errorf("VIES 19 does not print %q for %s:\n%s", want, custNo, printed)
			}
		}
	}
	if !strings.Contains(printed, "|"+amountText(dk.eu3Party)+"|") {
		c.t.Errorf("VIES 19 does not print the EU 3-party trade amount %s:\n%s", amountText(dk.eu3Party), printed)
	}

	result, err = reports.Execute(reports.VIESDeclarationDiskID, c.src.GetDB(), c.src.GetCompany(), c.src.GetDBType(),
		reports.RunOptions{Filters: filters})
	c.must(err)
	file := string(result.File)
	for custNo, s := range sales {
		var record string
		for _, line := range strings.Split(file, "\r\n") {
			if strings.HasPrefix(line, "02") && strings.Contains(line, vatRegNos[custNo]) {
				record = line
			}
		}
		// the sales, then the EU 3-party trade amount: whole units, 15 digits, a sign
		want := fmt.Sprintf("%015d+0%015d+", s.sales.IntPart(), s.eu3Party.IntPart())
		if !strings.HasSuffix(strings.TrimRight(record, " "), want) {
			c.t.Errorf("VIES 88 record of %s: %q, want it to end with %q\n%s", custNo, record, want, file)
		}
		taxEntry := gtables.NewTaxEntry(c.src)
		taxEntry.SetRange("sell_to_buy_from_no", custNo)
		taxEntry.SetFilter("posting_date", vatPeriod)
		taxEntry.SetRange("internal_ref_no", types.Text(""))
		if n := taxEntry.Count(); n != 0 {
			c.t.Errorf("%s: %d tax entries without Internal Ref. No. after VIES 88", custNo, n)
		}
	}
}

// amountText: an amount as the reports print it for en-US (1,234.50)
func amountText(d types.Decimal) string {
	s := d.Abs().StringFixed(2)
	whole, frac := s[:len(s)-3], s[len(s)-3:]
	var b strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if d.IsNegative() {
		return "-" + b.String() + frac
	}
	return b.String() + frac
}

// statementLine: a line of the demo VAT return TAX/DEFAULT
func (c *vatCheck) statementLine(rowNo string) *gtables.TaxStatementLine {
	c.t.Helper()
	line := gtables.NewTaxStatementLine(c.src)
	line.SetRange("statement_template_name", types.Code("TAX"))
	line.SetRange("statement_name", types.Code("DEFAULT"))
	line.SetRange("row_no", types.Code(rowNo))
	if !line.FindFirst() {
		c.t.Fatalf("no row %s in the VAT return", rowNo)
	}
	return line
}

// row: a row of the VAT return for the demo month
func (c *vatCheck) row(rowNo string, selection taxstatement.Selection) types.Decimal {
	c.t.Helper()
	calc := &taxstatement.Calculator{Src: c.src, DateFilter: vatPeriod, Selection: selection, PeriodSelection: taxstatement.WithinPeriod}
	total, err := calc.LineTotal(c.statementLine(rowNo))
	c.must(err)
	return total
}

// taxSum: the month's open tax entries of a type and tax posting groups — amount or base
func (c *vatCheck) taxSum(entryType gtables.TaxEntryType, bus, prod, field string) types.Decimal {
	c.t.Helper()
	taxEntry := gtables.NewTaxEntry(c.src)
	taxEntry.SetRange("type", entryType)
	taxEntry.SetRange("tax_bus_posting_group", types.Code(bus))
	taxEntry.SetRange("tax_prod_posting_group", types.Code(prod))
	taxEntry.SetRange("closed", false)
	taxEntry.SetFilter("posting_date", vatPeriod)
	c.must(taxEntry.CalcSums(field))
	if field == "base" {
		return taxEntry.Base
	}
	return taxEntry.Amount
}

// Step 5: the VAT return of the month — rows 10–15 the sales base and VAT per rate, 21–24 the
// input VAT, row 30 the VAT payable (output less input; the reverse charge's input VAT, row 23,
// left out, as it nets to 0 against 2345), row 40 the VAT accounts agreeing with it; report 12
// prints it with Open / Closed / Open and Closed entries
func (c *vatCheck) vatReturn() {
	sale, purchase := gtables.TaxEntry_Type.Sale, gtables.TaxEntry_Type.Purchase
	for _, r := range []struct {
		row       string
		entryType gtables.TaxEntryType
		bus, prod string
		field     string
	}{
		{"10", sale, "DOMESTIC", "STANDARD", "base"}, {"11", sale, "DOMESTIC", "STANDARD", "amount"},
		{"12", sale, "DOMESTIC", "REDUCED", "base"}, {"13", sale, "DOMESTIC", "REDUCED", "amount"},
		{"14", sale, "DOMESTIC", "EXEMPT", "base"}, {"15", sale, "EXPORT", "STANDARD", "base"},
		{"21", purchase, "DOMESTIC", "STANDARD", "amount"}, {"22", purchase, "DOMESTIC", "REDUCED", "amount"},
		{"23", purchase, "EXPORT", "STANDARD", "amount"}, {"24", purchase, "DOMESTIC", "FULL", "amount"},
	} {
		c.want("row "+r.row, c.row(r.row, taxstatement.SelectionOpen), c.taxSum(r.entryType, r.bus, r.prod, r.field))
	}
	for _, row := range []string{"10", "11", "15", "21", "23", "24"} {
		if c.row(row, taxstatement.SelectionOpen).IsZero() {
			c.t.Errorf("row %s is 0 in the demo month", row)
		}
	}
	payable := c.row("30", taxstatement.SelectionOpen)
	sum := types.Decimal{}
	for _, row := range []string{"11", "13", "21", "22", "24"} {
		sum = sum.Add(c.row(row, taxstatement.SelectionOpen))
	}
	c.want("row 30 = rows 11+13+21+22+24", payable, sum)
	if payable.IsZero() {
		c.t.Error("VAT payable is 0 in the demo month")
	}
	c.want("reverse charge: row 23 against 2345", c.row("23", taxstatement.SelectionOpen), c.gl("2345", "").Neg())
	// the VAT accounts agree with the VAT payable, apart from the demo's generated G/L history:
	// it posts VAT on 2330/1440 without tax entries
	history := c.historyVAT("1440|2330|2345")
	c.want("row 40 (1440, 2330, 2345) = row 30 + the generated history's VAT", c.row("40", taxstatement.SelectionOpen), payable.Add(history))

	c.want("row 30, closed entries", c.row("30", taxstatement.SelectionClosed), vd("0"))
	c.want("row 30, open and closed entries", c.row("30", taxstatement.SelectionOpenAndClosed), payable)
	c.vatPayable = payable

	for selection := 1; selection <= 3; selection++ {
		result, err := reports.Execute(reports.TaxStatementID, c.src.GetDB(), c.src.GetCompany(), c.src.GetDBType(), reports.RunOptions{
			Output: reports.OutputPDF, Options: map[string]string{"include_tax_entries": fmt.Sprint(selection), "period": "2"},
			Filters: map[string][]ftables.FieldFilter{
				"TaxStatementName": {{Field: "statement_template_name", Expression: "TAX"}, {Field: "name", Expression: "DEFAULT"}},
				"TaxStatementLine": {{Field: "date_filter", Expression: vatPeriod}}}})
		c.must(err)
		var texts []string
		for _, text := range result.Texts {
			texts = append(texts, text.Text)
		}
		printed := "|" + strings.Join(texts, "|") + "|"
		want := amountText(payable.Neg()) // printed with the opposite sign
		if selection == 2 {
			want = "VAT payable" // closed entries: nothing yet
		}
		if !strings.Contains(printed, "|"+want+"|") {
			c.t.Errorf("report 12 (selection %d) does not print %q:\n%s", selection, want, printed)
		}
	}
}

// Step 6: report 20 first as a test report (nothing posted), then posted to 2340: no open VAT
// entries in the month, a closed settlement entry per tax posting setup and type, 2330 and 1440
// at 0, 2345 settled against 1440, 2340 the VAT payable; the VAT return on closed entries the
// same as before on open ones
func (c *vatCheck) settlement() {
	countEntries := func() (int, int) {
		glEntry := gtables.NewGLEntry(c.src)
		open := gtables.NewTaxEntry(c.src)
		open.SetRange("closed", false)
		open.SetFilter("type", "1|2")
		open.SetFilter("posting_date", vatPeriod)
		return glEntry.Count(), open.Count()
	}
	// the groups to settle: tax posting setup and type of the month's open entries
	groups := map[string]bool{}
	open := gtables.NewTaxEntry(c.src)
	open.SetRange("closed", false)
	open.SetFilter("type", "1|2")
	open.SetFilter("posting_date", vatPeriod)
	for found := open.FindSet(); found; found = open.Next() {
		groups[fmt.Sprintf("%s/%s/%d", open.Tax_bus_posting_group, open.Tax_prod_posting_group, open.Type)] = true
	}
	glBefore, openBefore := countEntries()
	if openBefore == 0 {
		c.t.Fatal("no open VAT entries to settle")
	}
	options := map[string]string{"starting_date": "2026-10-01", "ending_date": "2026-10-31", "posting_date": "2026-10-31",
		"document_no": "VATCHK-SET", "settlement_account": "2340"}
	_, err := reports.Execute(reports.CalcAndPostTaxSettlementID, c.src.GetDB(), c.src.GetCompany(), c.src.GetDBType(),
		reports.RunOptions{Output: reports.OutputPDF, Options: options})
	c.must(err)
	if glAfter, openAfter := countEntries(); glAfter != glBefore || openAfter != openBefore {
		c.t.Fatalf("the test report posted: G/L entries %d → %d, open VAT entries %d → %d", glBefore, glAfter, openBefore, openAfter)
	}

	options["post"] = "true"
	_, err = reports.Execute(reports.CalcAndPostTaxSettlementID, c.src.GetDB(), c.src.GetCompany(), c.src.GetDBType(),
		reports.RunOptions{Output: reports.OutputPDF, Options: options})
	c.must(err)
	if _, openAfter := countEntries(); openAfter != 0 {
		c.t.Errorf("%d VAT entries of the month still open", openAfter)
	}
	settlement := gtables.NewTaxEntry(c.src)
	settlement.SetRange("type", gtables.TaxEntry_Type.Settlement)
	settlement.SetRange("document_no", types.Code("VATCHK-SET"))
	settled := map[string]int{}
	for found := settlement.FindSet(); found; found = settlement.Next() {
		if !settlement.Closed {
			c.t.Errorf("settlement entry %d is open", settlement.Entry_no)
		}
		settled[fmt.Sprintf("%s/%s", settlement.Tax_bus_posting_group, settlement.Tax_prod_posting_group)]++
	}
	if settlement.Count() == 0 || settlement.Count() > len(groups) {
		c.t.Errorf("%d settlement entries for %d tax posting setups and types: %v", settlement.Count(), len(groups), groups)
	}
	for setup, n := range settled {
		if n > 2 {
			c.t.Errorf("tax posting setup %s: %d settlement entries (at most one per type)", setup, n)
		}
	}
	c.want("2330 after the settlement (the generated history's VAT left)", c.gl("2330", ""), c.historyVAT("2330"))
	c.want("1440 after the settlement (the generated history's VAT left)", c.gl("1440", ""), c.historyVAT("1440"))
	c.want("2345 after the settlement", c.gl("2345", ""), vd("0"))
	c.want("2340 = the VAT payable", c.gl("2340", "VATCHK-SET"), c.vatPayable)
	c.want("row 30, closed entries", c.row("30", taxstatement.SelectionClosed), c.vatPayable)
	c.want("row 30, open entries", c.row("30", taxstatement.SelectionOpen), vd("0"))
}

// custLine: a customer line (invoice or payment) balanced by an account
func (c *vatCheck) custLine(documentNo string, docType gtables.GenJournalLineDocument_type, custNo types.Code, amount string,
	balType gtables.GenJournalLineBal_account_type, balNo string, postingDate types.Date) *gtables.GenJournalLine {
	c.t.Helper()
	line := gtables.NewGenJournalLine(c.src)
	line.Init()
	c.must(line.Validate("posting_date", postingDate))
	line.Document_no = types.Code(documentNo)
	c.must(line.Validate("document_type", docType))
	c.must(line.Validate("account_type", gtables.GenJournalLine_Account_type.Customer))
	c.must(line.Validate("account_no", custNo))
	c.must(line.Validate("bal_account_type", balType))
	c.must(line.Validate("bal_account_no", types.Code(balNo)))
	c.must(line.Validate("amount", vd(amount)))
	return line
}

func (c *vatCheck) payment(documentNo string, custNo types.Code, amount, appliesTo string, postingDate types.Date) {
	c.t.Helper()
	line := c.custLine(documentNo, gtables.GenJournalLine_Document_type.Payment, custNo, amount,
		gtables.GenJournalLine_Bal_account_type.BankAccount, "B010", postingDate)
	line.Applies_to_doc_type = gtables.GenJournalLine_Applies_to_doc_type.Invoice
	line.Applies_to_doc_no = types.Code(appliesTo)
	c.postLine(line)
}

func (c *vatCheck) custEntry(custNo types.Code, documentNo string) *gtables.CustomerLedgerEntry {
	c.t.Helper()
	entry := gtables.NewCustomerLedgerEntry(c.src)
	entry.SetRange("customer_no", custNo)
	entry.SetRange("document_no", types.Code(documentNo))
	if !entry.FindFirst() {
		c.t.Fatalf("no customer entry %s", documentNo)
	}
	return entry
}

// glAccount adds a posting account when it does not exist
func (c *vatCheck) glAccount(no, name string) {
	c.t.Helper()
	glAcc := gtables.NewGLAccount(c.src)
	if glAcc.Get(types.Code(no)) {
		c.t.Fatalf("G/L account %s exists in the demo", no)
	}
	glAcc.No = types.Code(no)
	glAcc.Name = types.Text(name)
	c.must(glAcc.InsertOrError(false))
}

func (c *vatCheck) unrealizedType(unrealizedTaxType gtables.TaxPostingSetupUnrealized_tax_type) {
	c.t.Helper()
	setup := gtables.NewTaxPostingSetup(c.src)
	c.must(setup.GetOrError(map[string]interface{}{"tax_bus_posting_group": "DOMESTIC", "tax_prod_posting_group": "STANDARD"}))
	c.must(setup.Validate("unrealized_tax_type", unrealizedTaxType))
	c.must(setup.ModifyOrError(false))
}

// Step 8: unrealized VAT on DOMESTIC/STANDARD — Percentage: a sales invoice of 1250 puts its
// VAT on the unrealized account (tax entry: Unrealized Amount −250, Amount 0); a payment of 500
// realizes 100 (−100 / base −400), the rest the remaining 150; unapplying the last payment makes
// its 150 unrealized again; First on a purchase invoice, Last on a sales invoice; the VAT
// return's Unrealized Amount/Base
func (c *vatCheck) unrealizedVAT() {
	c.glAccount("2336", "Sales VAT, unrealized")
	c.glAccount("1446", "Purchase VAT, unrealized")
	glSetup := gtables.GetGeneralLedgerSetup(c.src)
	c.must(glSetup.Validate("unrealized_tax", true))
	c.must(glSetup.ModifyOrError(false))
	setup := gtables.NewTaxPostingSetup(c.src)
	c.must(setup.GetOrError(map[string]interface{}{"tax_bus_posting_group": "DOMESTIC", "tax_prod_posting_group": "STANDARD"}))
	c.must(setup.Validate("sales_vat_unreal_account", types.Code("2336")))
	c.must(setup.Validate("purch_vat_unreal_account", types.Code("1446")))
	c.must(setup.ModifyOrError(false))
	c.unrealizedType(gtables.TaxPostingSetup_Unrealized_tax_type.Percentage)

	custNo := c.firstCustomer("DOMESTIC")
	day := func(d int) types.Date { return types.NewDate(2026, 10, d) }
	c.postLine(c.custLine("VATCHK-U1", gtables.GenJournalLine_Document_type.Invoice, custNo, "1250",
		gtables.GenJournalLine_Bal_account_type.GLAccount, "3110", day(21)))
	c.want("invoice: unrealized VAT 2336", c.gl("2336", "VATCHK-U1"), vd("-250"))
	c.want("invoice: VAT 2330", c.gl("2330", "VATCHK-U1"), vd("0"))
	invTax := c.oneTaxEntry("VATCHK-U1")
	c.want("invoice tax entry: Unrealized Amount", invTax.Unrealized_amount, vd("-250"))
	c.want("invoice tax entry: Unrealized Base", invTax.Unrealized_base, vd("-1000"))
	c.want("invoice tax entry: Amount", invTax.Amount, vd("0"))

	c.payment("VATCHK-UP1", custNo, "-500", "VATCHK-U1", day(22))
	c.want("first payment: 2336", c.gl("2336", "VATCHK-UP1"), vd("100"))
	c.want("first payment: 2330", c.gl("2330", "VATCHK-UP1"), vd("-100"))
	realized := c.oneTaxEntry("VATCHK-UP1")
	c.want("realized tax entry", realized.Amount, vd("-100"))
	c.want("realized tax entry base", realized.Base, vd("-400"))
	if realized.Unrealized_tax_entry_no != invTax.Entry_no {
		c.t.Errorf("realized tax entry of %d, want the invoice's %d", realized.Unrealized_tax_entry_no, invTax.Entry_no)
	}
	c.want("remaining unrealized", c.oneTaxEntry("VATCHK-U1").Remaining_unrealized_amount, vd("-150"))

	c.payment("VATCHK-UP2", custNo, "-750", "VATCHK-U1", day(23))
	c.want("second payment: 2330", c.gl("2330", "VATCHK-UP2"), vd("-150"))
	c.want("remaining unrealized after the second payment", c.oneTaxEntry("VATCHK-U1").Remaining_unrealized_amount, vd("0"))
	c.want("2336 after the invoice is paid", c.gl("2336", "VATCHK-U1|VATCHK-UP1|VATCHK-UP2"), vd("0"))

	paymentEntry := c.custEntry(custNo, "VATCHK-UP2") // read before the transaction: SQLite has one connection
	c.inTx(func(src ftables.Source) error {
		entry := gtables.NewCustomerLedgerEntry(src)
		entry.Copy(paymentEntry)
		_, err := genjnl.UnapplyCustEntry(src, entry)
		return err
	})
	c.want("remaining unrealized after Unapply", c.oneTaxEntry("VATCHK-U1").Remaining_unrealized_amount, vd("-150"))
	c.want("2330 of the unapplied payment", c.gl("2330", "VATCHK-UP2"), vd("0"))
	c.want("2336 of the unapplied payment", c.gl("2336", "VATCHK-UP2"), vd("0"))
	c.payment("VATCHK-UP3", custNo, "-750", "VATCHK-U1", day(23)) // paid again
	c.want("payment after Unapply: 2330", c.gl("2330", "VATCHK-UP3"), vd("-150"))

	// the VAT return's Unrealized Amount and Base of the domestic sales
	for amountType, want := range map[gtables.TaxStatementLineAmount_type]string{
		gtables.TaxStatementLine_Amount_type.UnrealizedAmount: "-250", gtables.TaxStatementLine_Amount_type.UnrealizedBase: "-1000"} {
		line := gtables.NewTaxStatementLine(c.src)
		line.Statement_template_name = "TAX"
		line.Statement_name = "DEFAULT"
		line.Type = gtables.TaxStatementLine_Type.TaxEntryTotaling
		line.Gen_posting_type = gtables.TaxStatementLine_Gen_posting_type.Sale
		line.Tax_bus_posting_group = "DOMESTIC"
		line.Tax_prod_posting_group = "STANDARD"
		line.Amount_type = amountType
		calc := &taxstatement.Calculator{Src: c.src, DateFilter: "2026-10-21..2026-10-31", Selection: taxstatement.SelectionOpenAndClosed,
			PeriodSelection: taxstatement.WithinPeriod}
		total, err := calc.LineTotal(line)
		c.must(err)
		c.want(fmt.Sprintf("VAT return unrealized (%v)", amountType), total, vd(want))
	}

	// First, a purchase: 100 of 1250 paid realizes 100 of the 250 VAT
	c.unrealizedType(gtables.TaxPostingSetup_Unrealized_tax_type.First)
	vendNo := c.firstVendor("DOMESTIC")
	inv := gtables.NewGenJournalLine(c.src)
	inv.Init()
	c.must(inv.Validate("posting_date", day(24)))
	inv.Document_no = "VATCHK-U2"
	inv.External_document_no = "VATCHK-U2-EXT"
	c.must(inv.Validate("document_type", gtables.GenJournalLine_Document_type.Invoice))
	c.must(inv.Validate("account_type", gtables.GenJournalLine_Account_type.Vendor))
	c.must(inv.Validate("account_no", vendNo))
	c.must(inv.Validate("bal_account_type", gtables.GenJournalLine_Bal_account_type.GLAccount))
	c.must(inv.Validate("bal_account_no", types.Code("4110")))
	c.must(inv.Validate("amount", vd("-1250")))
	c.postLine(inv)
	c.want("purchase invoice: unrealized VAT 1446", c.gl("1446", "VATCHK-U2"), vd("250"))
	pay := gtables.NewGenJournalLine(c.src)
	pay.Init()
	c.must(pay.Validate("posting_date", day(25)))
	pay.Document_no = "VATCHK-UP4"
	c.must(pay.Validate("document_type", gtables.GenJournalLine_Document_type.Payment))
	c.must(pay.Validate("account_type", gtables.GenJournalLine_Account_type.Vendor))
	c.must(pay.Validate("account_no", vendNo))
	c.must(pay.Validate("bal_account_type", gtables.GenJournalLine_Bal_account_type.BankAccount))
	c.must(pay.Validate("bal_account_no", types.Code("B010")))
	c.must(pay.Validate("amount", vd("100")))
	pay.Applies_to_doc_type = gtables.GenJournalLine_Applies_to_doc_type.Invoice
	pay.Applies_to_doc_no = "VATCHK-U2"
	c.postLine(pay)
	c.want("First: realized 1440", c.gl("1440", "VATCHK-UP4"), vd("100"))
	c.want("First: remaining unrealized", c.oneTaxEntry("VATCHK-U2").Remaining_unrealized_amount, vd("150"))

	// Last, a sale: 500 of 1250 paid realizes nothing (the base first), the rest the 250
	c.unrealizedType(gtables.TaxPostingSetup_Unrealized_tax_type.Last)
	c.postLine(c.custLine("VATCHK-U3", gtables.GenJournalLine_Document_type.Invoice, custNo, "1250",
		gtables.GenJournalLine_Bal_account_type.GLAccount, "3110", day(26)))
	c.payment("VATCHK-UP5", custNo, "-500", "VATCHK-U3", day(27))
	c.want("Last: first payment realizes", c.gl("2330", "VATCHK-UP5"), vd("0"))
	c.payment("VATCHK-UP6", custNo, "-750", "VATCHK-U3", day(28))
	c.want("Last: second payment realizes", c.gl("2330", "VATCHK-UP6"), vd("-250"))
	c.want("Last: remaining unrealized", c.oneTaxEntry("VATCHK-U3").Remaining_unrealized_amount, vd("0"))

	if err := glSetup.Validate("unrealized_tax", false); err == nil {
		c.t.Error("Unrealized Tax turned off while a setup has an Unrealized Tax Type")
	}
	// back to realized VAT for the next step
	c.unrealizedType(gtables.TaxPostingSetup_Unrealized_tax_type.Blank)
	glSetup = gtables.GetGeneralLedgerSetup(c.src)
	c.must(glSetup.Validate("unrealized_tax", false))
	c.must(glSetup.ModifyOrError(false))
}

// Step 9: Adjust for Payment Disc. — an invoice of 1250 with 2 % discount paid with 1225: 20 on
// the Sales Payment Disc. Account, 5 off 2330, a tax entry of the payment (5 / base 20);
// Unapply Entries takes it back; the setup's options cannot be turned off while used
func (c *vatCheck) paymentDiscountVAT() {
	c.glAccount("3156", "Payment discounts granted, net")
	custNo := c.firstCustomer("DOMESTIC")
	cust := gtables.NewCustomer(c.src)
	c.must(cust.GetOrError(custNo))
	genPostingSetup := gtables.NewGeneralPostingSetup(c.src)
	c.must(genPostingSetup.GetOrError(map[string]interface{}{"gen_bus_posting_group": cust.Gen_bus_posting_group, "gen_prod_posting_group": "RETAIL"}))
	genPostingSetup.Sales_payment_disc_account = "3156"
	c.must(genPostingSetup.ModifyOrError(false))
	glSetup := gtables.GetGeneralLedgerSetup(c.src)
	c.must(glSetup.Validate("adjust_for_payment_disc", true))
	c.must(glSetup.ModifyOrError(false))
	setup := gtables.NewTaxPostingSetup(c.src)
	c.must(setup.GetOrError(map[string]interface{}{"tax_bus_posting_group": "DOMESTIC", "tax_prod_posting_group": "STANDARD"}))
	c.must(setup.Validate("adjust_for_payment_discount", true))
	c.must(setup.ModifyOrError(false))

	day := func(d int) types.Date { return types.NewDate(2026, 10, d) }
	inv := c.custLine("VATCHK-D1", gtables.GenJournalLine_Document_type.Invoice, custNo, "1250",
		gtables.GenJournalLine_Bal_account_type.GLAccount, "3110", day(21))
	inv.Payment_discount_percent = vd("2")
	inv.Pmt_discount_date = day(29)
	c.postLine(inv)
	c.payment("VATCHK-DP1", custNo, "-1225", "VATCHK-D1", day(28))
	c.want("discount on 3156", c.gl("3156", "VATCHK-DP1"), vd("20"))
	c.want("VAT off 2330", c.gl("2330", "VATCHK-DP1"), vd("5"))
	taxEntry := c.oneTaxEntry("VATCHK-DP1")
	c.want("discount tax entry", taxEntry.Amount, vd("5"))
	c.want("discount tax entry base", taxEntry.Base, vd("20"))
	if taxEntry.Type != gtables.TaxEntry_Type.Sale {
		c.t.Errorf("discount tax entry type %v", taxEntry.Type)
	}
	if invEntry := c.custEntry(custNo, "VATCHK-D1"); invEntry.Open {
		c.t.Error("the invoice is still open after the payment with discount")
	}

	paymentEntry := c.custEntry(custNo, "VATCHK-DP1") // read before the transaction: SQLite has one connection
	c.inTx(func(src ftables.Source) error {
		entry := gtables.NewCustomerLedgerEntry(src)
		entry.Copy(paymentEntry)
		_, err := genjnl.UnapplyCustEntry(src, entry)
		return err
	})
	var total types.Decimal
	for _, e := range c.taxEntries("VATCHK-DP1") {
		total = total.Add(e.Amount)
	}
	c.want("tax entries of the unapplied payment", total, vd("0"))
	c.want("2330 of the unapplied payment", c.gl("2330", "VATCHK-DP1"), vd("0"))
	c.want("3156 of the unapplied payment", c.gl("3156", "VATCHK-DP1"), vd("0"))
	invEntry := c.custEntry(custNo, "VATCHK-D1")
	invEntry.CalcFields("remaining_amount")
	if !invEntry.Open {
		c.t.Error("the invoice is not open again after Unapply")
	}
	c.want("invoice remaining after Unapply", invEntry.Remaining_amount, vd("1250"))

	glSetup = gtables.GetGeneralLedgerSetup(c.src)
	if err := glSetup.Validate("adjust_for_payment_disc", false); err == nil {
		c.t.Error("Adjust for Payment Disc. turned off while a setup uses it")
	}
}
