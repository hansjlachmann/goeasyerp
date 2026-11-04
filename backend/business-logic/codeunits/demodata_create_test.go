package codeunits

import (
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/hansjlachmann/openerp/backend/business-logic/bomjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/openerp/backend/business-logic/fajnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/fareclass"
	"github.com/hansjlachmann/openerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/glcheck"
	"github.com/hansjlachmann/openerp/backend/business-logic/insurancejnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/projectjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/resjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Tables of active main menu entries that the company's demo data does not fill: system tables
// (global), temporary pages (filled when opened) and per-user setups
var demoDataNotNeeded = map[string]bool{
	"Company": true, "Installed_App": true, "Language": true, "Menu": true, "Permission": true, "Theme": true,
	"User": true, "User_Member": true, "User_Role": true, "Job_Queue": true, "SMTP_Setup": true,
	"Document_Entry":       true, // Navigate (temporary)
	"User_Setup":           true, // per user: the users are not part of a company's demo data
	"Printer_Selection":    true, // per user and global, like User Setup
	"Object":               true, // written at startup
	"Date_Compr_Register":  true, // written by date compressions, which need a closed year with entries
	"User_Time_Register":   true, // written when users log out
	"Setup_Checklist_Line": true, // made when the Setup Checklist opens
	"Business_Unit":        true, // consolidation needs a second company
	"Refused_Posting":      true, // written only when the balance check refuses a posting
}

type menuEntry struct {
	Name   string      `yaml:"name"`
	PageID int         `yaml:"page_id"`
	Items  []menuEntry `yaml:"items"`
}

// menuPages: the pages the main menu opens, with the entry's path
func menuPages(t *testing.T) map[int]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "definitions", "menus", "admin.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var menu struct {
		Items []menuEntry `yaml:"menu"`
	}
	if err := yaml.Unmarshal(data, &menu); err != nil {
		t.Fatal(err)
	}
	pages := map[int]string{}
	var walk func(entries []menuEntry, path string)
	walk = func(entries []menuEntry, path string) {
		for _, e := range entries {
			if e.PageID != 0 {
				pages[e.PageID] = path + e.Name
			}
			walk(e.Items, path+e.Name+" > ")
		}
	}
	walk(menu.Items, "")
	return pages
}

// pageSourceTables: page id → its source table
func pageSourceTables(t *testing.T) map[int]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "definitions", "pages", "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	tables := map[int]string{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var page struct {
			Page struct {
				ID          int    `yaml:"id"`
				SourceTable string `yaml:"source_table"`
			} `yaml:"page"`
		}
		if yaml.Unmarshal(data, &page) == nil && page.Page.ID != 0 {
			tables[page.Page.ID] = page.Page.SourceTable
		}
	}
	return tables
}

// Codeunit 901's demo data (all postings and the open journals): every page of the main menu
// shows records of the demo company, the G/L passes the balance check, and every journal's lines
// post
func TestDemoDataFillsTheMainMenu(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	counts, err := demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{Today: time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local),
		ItemJnlPostLine: openItemJnlPostLine, BOMJnlPostLine: openBOMJnlPostLine, SalesPost: openSalesPost, PurchPost: openPurchPost,
		GenJnlPostLine: openGenJnlPostLine, ProjectJnlPostLine: openProjectJnlPostLine, Journals: true,
		ReminderIssue: openReminderIssue, FinChrgMemoIssue: openFinChrgMemoIssue, AdjustExchRates: adjustExchRates})
	if err != nil {
		t.Fatal(err)
	}
	if counts.JournalLines < 15 {
		t.Errorf("%d journal lines", counts.JournalLines)
	}
	// the bank entries before this month are reconciled; the last statement's balance is the
	// balance at last month's end (Last Statement No. is the open reconciliation's, ST2026-10)
	src0 := ftables.DBSource(db, "demo", database.DBTypeSQLite)
	openBank := gtables.NewBankAccountLedgerEntry(src0)
	openBank.SetRange("open", true)
	openBank.SetRange("posting_date", types.Date{}, types.NewDate(2026, 9, 30))
	if n := openBank.Count(); n != 0 {
		t.Errorf("%d bank entries before October still open", n)
	}
	// an open sales order reserves from the inventory: its lines and the item entries say so
	reservEntry := gtables.NewReservationEntry(src0)
	reservEntry.SetRange("reservation_status", gtables.ReservationEntry_Reservation_status.Open)
	reservedLines := gtables.NewSalesLine(src0)
	reservedLines.SetFilter("reserved_qty_base", ">0")
	if reservEntry.Count() == 0 || reservedLines.Count() == 0 {
		t.Errorf("%d open reservations, %d reserved sales lines", reservEntry.Count(), reservedLines.Count())
	}
	b010 := gtables.NewBankAccount(src0)
	if !b010.Get(types.Code("B010")) {
		t.Fatal("no bank account B010")
	}
	if err := b010.SetFlowFilter("date_filter", "..2026-09-30"); err != nil {
		t.Fatal(err)
	}
	b010.CalcFields("balance_at_date")
	if b010.Last_statement_no != "ST2026-10" || !b010.Balance_last_statement.Equal(b010.Balance_at_date) {
		t.Errorf("B010: last statement %s balance %s, want ST2026-10 and the balance at 30 September %s",
			b010.Last_statement_no, b010.Balance_last_statement, b010.Balance_at_date)
	}
	// the demo bank is not overdrawn (a loan finances the fixed assets); the GBP account's
	// entries are in pounds, so its exchange rate adjustment is a small part of its balance
	bankGL := gtables.NewGLAccount(src0)
	if !bankGL.Get(types.Code("1520")) {
		t.Fatal("no G/L account 1520")
	}
	bankGL.CalcFields("balance")
	if err := b010.SetFlowFilter("date_filter", ""); err != nil {
		t.Fatal(err)
	}
	b010.CalcFields("balance")
	if !bankGL.Balance.IsPositive() || !b010.Balance.IsPositive() {
		t.Errorf("bank overdrawn: G/L 1520 %s, B010 %s", bankGL.Balance, b010.Balance)
	}
	gbp := gtables.NewBankAccountLedgerEntry(src0)
	gbp.SetRange("bank_account_no", types.Code("B030"))
	gbp.SetRange("source_code", types.Code("GENJNL"))
	for found := gbp.FindSetBuffered(); found; found = gbp.Next() {
		if gbp.Currency_code != "GBP" || !gbp.Amount_lcy.Abs().GreaterThan(gbp.Amount.Abs()) {
			t.Errorf("B030 entry %d: %s %s, LCY %s", gbp.Entry_no, gbp.Amount, gbp.Currency_code, gbp.Amount_lcy)
			break
		}
	}
	adjReg := gtables.NewExchRateAdjmtReg(src0)
	adjReg.SetRange("account_type", gtables.ExchRateAdjmtReg_Account_type.BankAccount)
	for found := adjReg.FindSetBuffered(); found; found = adjReg.Next() {
		if adjReg.Adjusted_amt_lcy.Abs().GreaterThan(adjReg.Adjusted_base_lcy.Abs().Mul(types.MustDecimal("0.05"))) {
			t.Errorf("exchange rate adjustment %s of a balance of %s LCY", adjReg.Adjusted_amt_lcy, adjReg.Adjusted_base_lcy)
		}
	}
	if counts.ExchRateAdjmtRegs == 0 {
		t.Error("no exchange rate adjustment registers")
	}
	if counts.Reminders != 2 || counts.IssuedReminders != 1 || counts.FinChrgMemos != 1 || counts.IssuedFinChrgMemos != 1 {
		t.Errorf("reminders %d (issued %d), finance charge memos %d (issued %d), want 2 (1), 1 (1)",
			counts.Reminders, counts.IssuedReminders, counts.FinChrgMemos, counts.IssuedFinChrgMemos)
	}
	sources := pageSourceTables(t)
	var empty []string
	for pageID, path := range menuPages(t) {
		table := sources[pageID]
		if table == "" || demoDataNotNeeded[table] {
			continue
		}
		factory, ok := gtables.GetTableFactory(table)
		if !ok {
			t.Errorf("%s: page %d's table %s not found", path, pageID, table)
			continue
		}
		rec := factory()
		rec.InitWithDBType(db, "demo", database.DBTypeSQLite)
		counter, ok := rec.(interface{ Count() int })
		if ok && counter.Count() == 0 {
			empty = append(empty, path+" ("+table+")")
		}
	}
	sort.Strings(empty)
	for _, e := range empty {
		t.Errorf("no demo data: %s", e)
	}
	result := glcheck.Check(ftables.DBSource(db, "demo", database.DBTypeSQLite), nil)
	if !result.Balanced() {
		t.Errorf("G/L balance check: %+v", result)
	}

	// every journal's lines post (each batch in a transaction rolled back)
	inTx := func(name string, post func(src ftables.Source) error) {
		tx, err := ftables.BeginTransaction(db)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = ftables.Rollback(tx) }()
		err = post(ftables.DBSource(tx, "demo", database.DBTypeSQLite))
		if err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for _, template := range []string{"GENERAL", "SALES", "PURCHASES", "CASHRCPT", "PAYMENT", "ASSETS"} {
		inTx("general journal "+template, func(src ftables.Source) error {
			l := gtables.NewGenJournalLine(src)
			l.SetRange("journal_template_name", template)
			if !l.FindFirst() {
				t.Errorf("general journal %s has no lines", template)
				return nil
			}
			result, err := genjnl.NewGenJnlPostBatch(src).Run(l)
			if err == nil && result.RegisterNo == 0 {
				t.Errorf("general journal %s: nothing posted", template)
			}
			return err
		})
	}
	for _, template := range []string{"ITEM", "TRANSFER", "PHYS. INV."} {
		inTx("item journal "+template, func(src ftables.Source) error {
			l := gtables.NewItemJournalLine(src)
			l.SetRange("journal_template_name", template)
			if !l.FindFirst() {
				t.Errorf("item journal %s has no lines", template)
				return nil
			}
			_, err := itemjnl.NewItemJnlPostBatch(src).Run(l)
			return err
		})
	}
	inTx("project journal", func(src ftables.Source) error {
		l := gtables.NewProjectJournalLine(src)
		l.SetRange("journal_template_name", "PROJECT")
		l.FindFirst()
		_, err := projectjnl.NewProjectJnlPostBatch(src).Run(l)
		return err
	})
	inTx("resource journal", func(src ftables.Source) error {
		l := gtables.NewResJournalLine(src)
		l.FindFirst()
		_, err := resjnl.NewResJnlPostBatch(src).Run(l)
		return err
	})
	// the kits' pens are short: the Item Journal's found pens are posted first
	inTx("BOM journal", func(src ftables.Source) error {
		itemJnlLine := gtables.NewItemJournalLine(src)
		itemJnlLine.SetRange("journal_template_name", "ITEM")
		itemJnlLine.FindFirst()
		_, err := itemjnl.NewItemJnlPostBatch(src).Run(itemJnlLine)
		if err != nil {
			return err
		}
		l := gtables.NewBOMJournalLine(src)
		l.FindFirst()
		_, err = bomjnl.NewBOMJnlPostBatch(src).Run(l)
		return err
	})
	inTx("FA journal", func(src ftables.Source) error {
		l := gtables.NewFAJournalLine(src)
		l.FindFirst()
		_, err := fajnl.NewFAJnlPostBatch(src).Run(l)
		return err
	})
	inTx("FA reclass. journal", func(src ftables.Source) error {
		l := gtables.NewFAReclassJournalLine(src)
		l.FindFirst()
		result, err := fareclass.TransferBatch(src, l)
		if err == nil && !result.Done {
			t.Error("FA reclass. journal: nothing reclassified")
		}
		return err
	})
	inTx("insurance journal", func(src ftables.Source) error {
		l := gtables.NewInsuranceJournalLine(src)
		l.FindFirst()
		_, err := insurancejnl.PostBatch(src, l)
		return err
	})
}
