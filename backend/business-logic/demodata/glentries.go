package demodata

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/sift"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The demo G/L entries post a year of business onto the demo chart of accounts (setup.yaml):
// opening balances, sales invoices with sales tax and their payments, purchases with purchase
// tax and their payments, payroll, running costs by dimension 1 and dimension 2, loan interest and
// installments, depreciation and the quarterly tax settlement. Every transaction balances.

// Accounts of the demo chart the transactions use
const (
	accLand, accAccDeprBuildings     = "1210", "1220"
	accMachinery, accAccDeprMachine  = "1230", "1240"
	accVehicles, accAccDeprVehicles  = "1250", "1260"
	accOffice, accAccDeprOffice      = "1270", "1280"
	accRawMaterials, accFinished     = "1320", "1330"
	accReceivableDom, accReceivableX = "1410", "1420"
	accPurchaseTax                   = "1440"
	accCash, accBank, accSavings     = "1510", "1520", "1530"
	accShareCapital, accRetained     = "2110", "2120"
	accBankLoans, accMortgage        = "2210", "2220"
	accPayableDom, accPayableX       = "2310", "2320"
	accSalesTax, accTaxSettlement    = "2330", "2340"
	accPayrollTaxes                  = "2350"
)

// Tax rates of the demo tax groups (sales and purchase tax lines)
const (
	taxStandard = 0.25
	taxReduced  = 0.15
)

// glLine is one entry of a transaction; the amounts of a transaction add up to zero
type glLine struct {
	account     string
	amount      float64 // positive: debit, negative: credit
	department  string
	project     string
	postingType gtables.GLEntryGen_posting_type
	genBus      string
	genProd     string
	taxBus      string
	taxProd     string
	taxAmount   float64
	balType     gtables.GLEntryBal_account_type
	balNo       string
	sourceType  gtables.GLEntrySource_type
	sourceNo    string
	system      bool
}

type glTx struct {
	date        time.Time
	docType     gtables.GLEntryDocument_type
	docNo       string
	description string
	sourceCode  string
	priorYear   bool
	lines       []glLine
}

// glGenerator builds the transactions with a fixed random seed (the same demo every time)
type glGenerator struct {
	rng       *rand.Rand
	customers []string
	txs       []glTx
	docNos    map[string]int
}

func (g *glGenerator) amount(min, max float64) float64 {
	return math.Round((min+g.rng.Float64()*(max-min))*100) / 100
}

func (g *glGenerator) docNo(prefix string, date time.Time) string {
	g.docNos[prefix]++
	return fmt.Sprintf("%s%s-%04d", prefix, date.Format("0601"), g.docNos[prefix])
}

func (g *glGenerator) add(tx glTx) {
	g.txs = append(g.txs, tx)
}

// salesInvoice: receivable against revenue and sales tax; paid into the bank later
func (g *glGenerator) salesInvoice(date time.Time, revenue string, foreign bool, taxRate float64, prod, taxProd, department, project string, net float64) {
	customer := ""
	if len(g.customers) > 0 {
		customer = g.customers[g.rng.Intn(len(g.customers))]
	}
	receivable, bus, taxBus := accReceivableDom, "DOMESTIC", "DOMESTIC"
	if foreign {
		receivable, bus, taxBus, taxRate, taxProd = accReceivableX, "EXPORT", "EXPORT", 0, "EXEMPT"
	}
	tax := math.Round(net*taxRate*100) / 100
	doc := g.docNo("SI", date)
	customerSide := glLine{balType: gtables.GLEntry_Bal_account_type.Customer, balNo: customer,
		sourceType: gtables.GLEntry_Source_type.Customer, sourceNo: customer}
	lines := []glLine{
		{account: revenue, amount: -net, department: department, project: project,
			postingType: gtables.GLEntry_Gen_posting_type.Sale, genBus: bus, genProd: prod, taxBus: taxBus, taxProd: taxProd, taxAmount: -tax,
			balType: customerSide.balType, balNo: customer, sourceType: customerSide.sourceType, sourceNo: customer},
		{account: receivable, amount: net + tax, balType: customerSide.balType, balNo: customer, sourceType: customerSide.sourceType, sourceNo: customer},
	}
	if tax != 0 {
		lines = append(lines, glLine{account: accSalesTax, amount: -tax, taxBus: taxBus, taxProd: taxProd,
			postingType: gtables.GLEntry_Gen_posting_type.Sale, sourceType: customerSide.sourceType, sourceNo: customer})
	}
	g.add(glTx{date: date, docType: gtables.GLEntry_Document_type.Invoice, docNo: doc, description: "Invoice " + doc, sourceCode: "SALES", lines: lines})

	// Paid 14–40 days later (if that is still in the demo period: the caller trims)
	paid := date.AddDate(0, 0, 14+g.rng.Intn(27))
	g.add(glTx{date: paid, docType: gtables.GLEntry_Document_type.Payment, docNo: g.docNo("PAY", paid), description: "Payment " + doc, sourceCode: "SALES",
		lines: []glLine{
			{account: accBank, amount: net + tax, balType: customerSide.balType, balNo: customer, sourceType: customerSide.sourceType, sourceNo: customer},
			{account: receivable, amount: -(net + tax), balType: customerSide.balType, balNo: customer, sourceType: customerSide.sourceType, sourceNo: customer},
		}})
}

// purchase: cost and purchase tax against a payable; paid from the bank later
func (g *glGenerator) purchase(date time.Time, cost string, foreign bool, prod, department, description string, net float64) {
	payable, bus, taxBus, taxProd, taxRate := accPayableDom, "DOMESTIC", "DOMESTIC", "STANDARD", taxStandard
	if foreign {
		payable, bus, taxBus, taxProd, taxRate = accPayableX, "EXPORT", "EXPORT", "EXEMPT", 0
	}
	tax := math.Round(net*taxRate*100) / 100
	doc := g.docNo("PI", date)
	lines := []glLine{
		{account: cost, amount: net, department: department, postingType: gtables.GLEntry_Gen_posting_type.Purchase,
			genBus: bus, genProd: prod, taxBus: taxBus, taxProd: taxProd, taxAmount: tax, balType: gtables.GLEntry_Bal_account_type.GLAccount, balNo: payable},
		{account: payable, amount: -(net + tax)},
	}
	if tax != 0 {
		lines = append(lines, glLine{account: accPurchaseTax, amount: tax, taxBus: taxBus, taxProd: taxProd, postingType: gtables.GLEntry_Gen_posting_type.Purchase})
	}
	g.add(glTx{date: date, docType: gtables.GLEntry_Document_type.Invoice, docNo: doc, description: description, sourceCode: "PURCHASES", lines: lines})
	paid := date.AddDate(0, 0, 20+g.rng.Intn(15))
	g.add(glTx{date: paid, docType: gtables.GLEntry_Document_type.Payment, docNo: g.docNo("PP", paid), description: "Payment " + doc, sourceCode: "PURCHASES",
		lines: []glLine{{account: payable, amount: net + tax}, {account: accBank, amount: -(net + tax)}}})
}

// expense: a cost paid from the bank at once (dimension 1, dimension 2)
func (g *glGenerator) expense(date time.Time, account, department, project, description string, amount float64) {
	g.add(glTx{date: date, docNo: g.docNo("GJ", date), description: description, sourceCode: "GENJNL",
		lines: []glLine{
			{account: account, amount: amount, department: department, project: project, balType: gtables.GLEntry_Bal_account_type.GLAccount, balNo: accBank},
			{account: accBank, amount: -amount, balType: gtables.GLEntry_Bal_account_type.GLAccount, balNo: account},
		}})
}

// generateGLEntries creates the demo G/L entries for the 12 months up to today: SMALL posts 12
// sales invoices a month, LARGE 30, HEAVY 60 (and a quarter as many purchases). With
// fixedAssetsPosted the machinery, vehicle and office equipment accounts get no opening balance
// and no depreciation here: the fixed asset postings write them (buildings stay)
func generateGLEntries(today time.Time, size Size, customers []string, fixedAssetsPosted bool, newEntry func() *gtables.GLEntry) []*gtables.GLEntry {
	g := &glGenerator{rng: rand.New(rand.NewSource(42)), customers: customers, docNos: map[string]int{}}
	invoicesPerMonth := 12
	switch size {
	case Large:
		invoicesPerMonth = 30
	case Heavy:
		invoicesPerMonth = 60
	}
	first := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.Local).AddDate(0, -11, 0)

	// Opening balances on the first day (entries of the prior year's closing)
	opening := []glLine{
		{account: accLand, amount: 2500000}, {account: accAccDeprBuildings, amount: -250000},
		{account: accMachinery, amount: 800000}, {account: accAccDeprMachine, amount: -160000},
		{account: accVehicles, amount: 450000}, {account: accAccDeprVehicles, amount: -90000},
		{account: accOffice, amount: 120000}, {account: accAccDeprOffice, amount: -40000},
		{account: accRawMaterials, amount: 180000}, {account: accFinished, amount: 95000},
		{account: accBank, amount: 640000}, {account: accSavings, amount: 300000}, {account: accCash, amount: 15000},
		{account: accShareCapital, amount: -1000000}, {account: accBankLoans, amount: -1200000},
		{account: accMortgage, amount: -1100000}, {account: accRetained, amount: -1260000},
	}
	depreciation := []glLine{
		{account: "5610", amount: 6250, department: "ADM", system: true}, {account: accAccDeprBuildings, amount: -6250, system: true},
		{account: "5620", amount: 13333.33, department: "PROD", system: true}, {account: accAccDeprMachine, amount: -13333.33, system: true},
		{account: "5630", amount: 7500, department: "SALES", system: true}, {account: accAccDeprVehicles, amount: -7500, system: true},
		{account: "5640", amount: 2000, department: "ADM", system: true}, {account: accAccDeprOffice, amount: -2000, system: true},
	}
	if fixedAssetsPosted {
		opening = withoutFixedAssetAccounts(opening, accRetained)
		depreciation = depreciation[:2]
	}
	g.add(glTx{date: first, docNo: "OPENING", description: "Opening balance", sourceCode: "GENJNL", priorYear: true, lines: opening})

	for m := 0; m < 12; m++ {
		month := first.AddDate(0, m, 0)
		day := func(d int) time.Time { return month.AddDate(0, 0, d-1) }

		// Sales
		for range invoicesPerMonth {
			date := day(1 + g.rng.Intn(27))
			switch g.rng.Intn(10) {
			case 0, 1, 2, 3:
				project := ""
				if g.rng.Intn(3) == 0 {
					project = "WEBSHOP"
				}
				g.salesInvoice(date, "3110", false, taxStandard, "RETAIL", "STANDARD", "SALES", project, g.amount(8000, 45000))
			case 4, 5:
				g.salesInvoice(date, "3120", true, 0, "RETAIL", "EXEMPT", "SALES", "EXPANSION", g.amount(15000, 80000))
			case 6, 7:
				g.salesInvoice(date, "3130", false, taxStandard, "SERVICES", "STANDARD", "PROD", "", g.amount(5000, 25000))
			default:
				g.salesInvoice(date, "3140", false, taxReduced, "FOOD", "REDUCED", "SALES", "", g.amount(3000, 15000))
			}
		}
		// Cash sales
		cash := g.amount(2000, 6000)
		cashTax := math.Round(cash*taxStandard*100) / 100
		g.add(glTx{date: day(28), docNo: g.docNo("CS", day(28)), description: "Cash sales", sourceCode: "SALES",
			lines: []glLine{
				{account: accCash, amount: cash + cashTax},
				{account: "3110", amount: -cash, department: "SALES", postingType: gtables.GLEntry_Gen_posting_type.Sale,
					genBus: "DOMESTIC", genProd: "RETAIL", taxBus: "DOMESTIC", taxProd: "STANDARD", taxAmount: -cashTax},
				{account: accSalesTax, amount: -cashTax, postingType: gtables.GLEntry_Gen_posting_type.Sale, taxBus: "DOMESTIC", taxProd: "STANDARD"},
			}})

		// Purchases
		for range max(2, invoicesPerMonth/4) {
			date := day(1 + g.rng.Intn(25))
			if g.rng.Intn(4) == 0 {
				g.purchase(date, "4120", true, "RETAIL", "PROD", "Retail goods (import)", g.amount(10000, 40000))
			} else {
				g.purchase(date, "4110", false, "RETAIL", "PROD", "Materials", g.amount(6000, 30000))
			}
		}
		g.purchase(day(8), "4130", false, "SERVICES", "PROD", "Freight", g.amount(1500, 5000))

		// Payroll on the 25th: salaries and payroll taxes; the taxes are paid the next month
		gross := g.amount(95000, 105000)
		payrollTax := math.Round(gross*0.141*100) / 100
		pension := math.Round(gross*0.02*100) / 100
		adm, sales := math.Round(gross*55)/100, math.Round(gross*30)/100 // in cents, the rest to PROD
		g.add(glTx{date: day(25), docNo: g.docNo("PAYROLL", day(25)), description: "Salaries", sourceCode: "GENJNL",
			lines: []glLine{
				{account: "5110", amount: adm, department: "ADM"},
				{account: "5110", amount: sales, department: "SALES"},
				{account: "5110", amount: math.Round((gross-adm-sales)*100) / 100, department: "PROD"},
				{account: "5120", amount: payrollTax, department: "ADM"},
				{account: "5130", amount: pension, department: "ADM"},
				{account: accPayrollTaxes, amount: -payrollTax},
				{account: accBank, amount: -(gross + pension)},
			}})
		if m > 0 {
			prev := math.Round(gross*0.141*100) / 100 // about last month's
			g.add(glTx{date: day(15), docNo: g.docNo("GJ", day(15)), description: "Payroll taxes", sourceCode: "GENJNL",
				lines: []glLine{{account: accPayrollTaxes, amount: prev}, {account: accBank, amount: -prev}}})
		}

		// Running costs
		g.expense(day(1), "5210", "ADM", "", "Rent", 25000)
		g.expense(day(10), "5220", "ADM", "", "Electricity and heating", g.amount(4000, 9000))
		g.expense(day(12), "5230", "ADM", "", "Cleaning", g.amount(1800, 2200))
		g.expense(day(6), "5310", "SALES", "", "Fuel", g.amount(3000, 6000))
		g.expense(day(9), "5410", "ADM", "", "Office supplies", g.amount(500, 2500))
		g.expense(day(3), "5420", "ADM", "", "Phone and internet", g.amount(2500, 3200))
		g.expense(day(2), "5430", "ADM", "WEBSHOP", "Software licences", 6500)
		g.expense(day(14), "5510", "SALES", "WEBSHOP", "Advertising", g.amount(4000, 15000))
		g.expense(day(18), "5520", "SALES", "EXPANSION", "Travel", g.amount(1000, 8000))
		g.expense(day(20), "6130", "ADM", "", "Bank fees", g.amount(150, 400))
		if m%3 == 1 {
			g.expense(day(16), "5320", "PROD", "", "Repairs and maintenance", g.amount(3000, 12000))
		}
		if m%6 == 0 {
			g.expense(day(5), "5450", "ADM", "", "Insurance", 18000)
			g.expense(day(5), "5330", "SALES", "", "Vehicle insurance", 7200)
		}
		if m == 2 || m == 8 {
			g.expense(day(22), "5440", "ADM", "", "Accounting and audit", g.amount(15000, 25000))
		}

		// Loan: interest and installment; interest on the savings
		g.add(glTx{date: day(28), docNo: g.docNo("GJ", day(28)), description: "Loan interest and installment", sourceCode: "GENJNL",
			lines: []glLine{{account: "6120", amount: 6500, department: "ADM"}, {account: accBankLoans, amount: 25000}, {account: accBank, amount: -31500}}})
		g.add(glTx{date: day(28), docNo: g.docNo("GJ", day(28)), description: "Interest on savings", sourceCode: "GENJNL",
			lines: []glLine{{account: accSavings, amount: 450}, {account: "6110", amount: -450}}})

		// Depreciation at month end (system-created)
		g.add(glTx{date: day(28), docNo: g.docNo("DEPR", day(28)), description: "Depreciation", sourceCode: "GENJNL",
			lines: append([]glLine(nil), depreciation...)})

		// Quarterly tax settlement: output tax less input tax to the tax settlement account
		if m%3 == 2 {
			var out, in float64
			for _, tx := range g.txs {
				for _, l := range tx.lines {
					switch l.account {
					case accSalesTax:
						out -= l.amount
					case accPurchaseTax:
						in += l.amount
					}
				}
			}
			out, in = math.Round(out*100)/100, math.Round(in*100)/100
			date := day(28)
			g.add(glTx{date: date, docNo: g.docNo("TAX", date), description: "Tax settlement", sourceCode: "GENJNL",
				lines: []glLine{{account: accSalesTax, amount: out}, {account: accPurchaseTax, amount: -in}, {account: accTaxSettlement, amount: in - out}}})
			// Paid the next month
			g.add(glTx{date: date.AddDate(0, 0, 10), docNo: g.docNo("TAX", date), description: "Tax payment", sourceCode: "GENJNL",
				lines: []glLine{{account: accTaxSettlement, amount: out - in}, {account: accBank, amount: in - out}}})
		}
	}

	// The entries: transactions up to today, in date order, numbered as one transaction each
	var entries []*gtables.GLEntry
	transaction := 0
	sortTxsByDate(g.txs)
	for _, tx := range g.txs {
		if tx.date.After(today) {
			continue
		}
		transaction++
		for _, l := range tx.lines {
			e := newEntry()
			e.Gl_account_no = types.NewCode(l.account)
			e.Posting_date = types.NewDate(tx.date.Year(), tx.date.Month(), tx.date.Day())
			e.Document_date = e.Posting_date
			e.Document_type = tx.docType
			e.Document_no = types.NewCode(tx.docNo)
			e.Description = types.NewText(tx.description)
			amount := math.Round(l.amount*100) / 100
			e.Amount = types.NewDecimal(amount)
			if amount >= 0 {
				e.Debit_amount = types.NewDecimal(amount)
			} else {
				e.Credit_amount = types.NewDecimal(-amount)
			}
			e.Dimension_1_code = types.NewCode(l.department)
			e.Dimension_2_code = types.NewCode(l.project)
			e.Gen_posting_type = l.postingType
			e.Gen_bus_posting_group = types.NewCode(l.genBus)
			e.Gen_prod_posting_group = types.NewCode(l.genProd)
			e.Tax_bus_posting_group = types.NewCode(l.taxBus)
			e.Tax_prod_posting_group = types.NewCode(l.taxProd)
			e.Tax_amount = types.NewDecimal(math.Round(l.taxAmount*100) / 100)
			e.Bal_account_type = l.balType
			e.Bal_account_no = types.NewCode(l.balNo)
			e.Source_type = l.sourceType
			e.Source_no = types.NewCode(l.sourceNo)
			e.Source_code = types.NewCode(tx.sourceCode)
			e.System_created_entry = l.system
			e.Prior_year_entry = tx.priorYear
			e.User_id = "ADMIN"
			e.Transaction_no = transaction
			entries = append(entries, e)
		}
	}
	return entries
}

// sortTxsByDate orders the transactions by date, keeping the order of a day's transactions
func sortTxsByDate(txs []glTx) {
	for i := 1; i < len(txs); i++ {
		for j := i; j > 0 && txs[j].date.Before(txs[j-1].date); j-- {
			txs[j], txs[j-1] = txs[j-1], txs[j]
		}
	}
}

// insertGLEntries loads the demo G/L entries (InsertAll in chunks) with their SIFT totals
// suspended and built once afterwards, and a G/L register over them
func insertGLEntries(exec database.Executor, company string, dbType database.DBType, entries []*gtables.GLEntry, counts *Counts) error {
	if len(entries) == 0 {
		return nil
	}
	err := sift.Suspend(exec, dbType, company, gtables.GLEntryTableName)
	if err != nil {
		return err
	}
	var last gtables.GLEntry
	last.InitWithDBType(exec, company, dbType)
	firstNo := 1
	if last.FindLast() {
		firstNo = last.Entry_no + 1
	}
	src := ftables.DBSource(exec, company, dbType)
	glReg := gtables.NewGLRegister(src)
	nextRegNo := 1
	if glReg.FindLast() {
		nextRegNo = glReg.No + 1
	}
	// Customer, vendor and bank entries take G/L entry numbers when they are posted. The
	// generated ones are numbered on their own; where they reach past the G/L entries (LARGE,
	// HEAVY: more customer entries than G/L entries) the first posting's number would be
	// taken: the G/L entries start after them instead. The numbers skipped are history without
	// G/L detail — a G/L register over them, marked as compressed like a date compression's,
	// which the G/L Balance Check leaves out.
	lastNo := lastGLNumberedEntryNo(src)
	if lastNo >= firstNo+len(entries) {
		err = insertCompressedHistory(src, nextRegNo, firstNo, lastNo, entries[0].Posting_date)
		if err != nil {
			return err
		}
		firstNo = lastNo + 1
		nextRegNo++
	}
	for i, e := range entries {
		e.Entry_no = firstNo + i
	}
	const chunk = 1000
	for start := 0; start < len(entries); start += chunk {
		batch := entries[start:min(start+chunk, len(entries))]
		err := last.InsertAll(batch, true)
		if err != nil {
			var batchErr *ftables.BatchInsertError
			key, cause := "", err
			if errors.As(err, &batchErr) {
				key, cause = fmt.Sprint(batch[batchErr.Index].Entry_no), batchErr.Err
			}
			return &InsertError{Table: gtables.GLEntryTableName, Key: key, Err: cause}
		}
		counts.GLEntries += len(batch)
	}
	// a G/L register over them, as a posting writes one (the G/L check finds entries without)
	glReg = gtables.NewGLRegister(src)
	glReg.No = nextRegNo
	glReg.From_entry_no = firstNo
	glReg.To_entry_no = firstNo + len(entries) - 1
	glReg.Creation_date = entries[len(entries)-1].Posting_date
	err = glReg.InsertOrError(false)
	if err != nil {
		return err
	}
	return (&gtables.GLEntry{}).SyncKeys(exec, company, dbType)
}

// withoutFixedAssetAccounts: the opening balance without the accounts of the fixed asset posting
// groups (machinery, vehicles, office equipment and their depreciation) — their amount goes to the
// balancing account, so the transaction still balances
func withoutFixedAssetAccounts(lines []glLine, balancing string) []glLine {
	faAccounts := map[string]bool{accMachinery: true, accAccDeprMachine: true, accVehicles: true, accAccDeprVehicles: true, accOffice: true, accAccDeprOffice: true}
	var removed float64
	var kept []glLine
	for _, l := range lines {
		if faAccounts[l.account] {
			removed += l.amount
			continue
		}
		kept = append(kept, l)
	}
	for i := range kept {
		if kept[i].account == balancing {
			kept[i].amount += removed
		}
	}
	return kept
}

// lastGLNumberedEntryNo is the highest entry number of the ledgers whose entries take G/L
// entry numbers when posted and that the demo fills itself: customer, vendor and bank account
// ledger entries
func lastGLNumberedEntryNo(src ftables.Source) int {
	last := 0
	custLedgEntry := gtables.NewCustomerLedgerEntry(src)
	if custLedgEntry.FindLast() && custLedgEntry.Entry_no > last {
		last = custLedgEntry.Entry_no
	}
	vendLedgEntry := gtables.NewVendorLedgerEntry(src)
	if vendLedgEntry.FindLast() && vendLedgEntry.Entry_no > last {
		last = vendLedgEntry.Entry_no
	}
	bankAccLedgEntry := gtables.NewBankAccountLedgerEntry(src)
	if bankAccLedgEntry.FindLast() && bankAccLedgEntry.Entry_no > last {
		last = bankAccLedgEntry.Entry_no
	}
	return last
}

// insertCompressedHistory: a G/L register over the entry numbers fromNo..toNo, which have no
// G/L entries, and a Date Compr. Register row on it (Customer Ledger Entry), as a date
// compression of G/L-numbered entries leaves them
func insertCompressedHistory(src ftables.Source, regNo, fromNo, toNo int, date types.Date) error {
	glReg := gtables.NewGLRegister(src)
	glReg.No = regNo
	glReg.From_entry_no = fromNo
	glReg.To_entry_no = toNo
	glReg.Creation_date = date
	err := glReg.InsertOrError(false)
	if err != nil {
		return err
	}
	dateComprReg := gtables.NewDateComprRegister(src)
	nextNo := 1
	if dateComprReg.FindLast() {
		nextNo = dateComprReg.No + 1
	}
	dateComprReg = gtables.NewDateComprRegister(src)
	dateComprReg.No = nextNo
	dateComprReg.Creation_date = date
	dateComprReg.Table_id = gtables.CustomerLedgerEntryTableID
	dateComprReg.Register_no = regNo
	dateComprReg.Ending_date = date
	return dateComprReg.InsertOrError(false)
}
