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
	"github.com/hansjlachmann/openerp/backend/business-logic/resjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Tables of active main menu entries that the company's demo data does not fill: system tables
// (global), temporary pages (filled when opened) and per-user setups
var demoDataNotNeeded = map[string]bool{
	"Company": true, "Installed_App": true, "Language": true, "Menu": true, "Permission": true, "Theme": true,
	"User": true, "User_Member": true, "User_Role": true, "Job_Queue": true, "SMTP_Setup": true,
	"Document_Entry": true, // Navigate (temporary)
	"User_Setup":     true, // per user: the users are not part of a company's demo data
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
		Items []menuEntry `yaml:"items"`
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
		GenJnlPostLine: openGenJnlPostLine, Journals: true})
	if err != nil {
		t.Fatal(err)
	}
	if counts.JournalLines < 15 {
		t.Errorf("%d journal lines", counts.JournalLines)
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
