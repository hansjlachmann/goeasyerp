package reports_test

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/bomjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/projectjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchposting"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reminderissue"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesposting"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pdf"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// fullDemoCompany: the demo data with every posting of codeunit 901 (sales, purchases, fixed
// assets, projects, reminders, journals) — what the printed reports print
func fullDemoCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	mustOK(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	mustOK(t, tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite))
	_, err = demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{
		Today:              time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local),
		ItemJnlPostLine:    func(src ftables.Source) demodata.ItemJnlPoster { return itemjnl.NewItemJnlPostLine(src) },
		BOMJnlPostLine:     func(src ftables.Source) demodata.BOMJnlPoster { return bomjnl.NewBOMJnlPostLine(src) },
		SalesPost:          func(src ftables.Source) demodata.SalesPoster { return salesposting.NewSalesPost(src) },
		PurchPost:          func(src ftables.Source) demodata.PurchPoster { return purchposting.NewPurchPost(src) },
		GenJnlPostLine:     func(src ftables.Source) demodata.GenJnlPoster { return genjnl.NewGenJnlPostLine(src) },
		ProjectJnlPostLine: func(src ftables.Source) demodata.ProjectJnlPoster { return projectjnl.NewProjectJnlPostLine(src) },
		ReminderIssue:      func(src ftables.Source) demodata.ReminderIssuer { return reminderissue.New(src) },
		FinChrgMemoIssue:   func(src ftables.Source) demodata.FinChrgMemoIssuer { return reminderissue.NewFinChrgMemoIssue(src) },
		AdjustExchRates: func(src ftables.Source, postingDate types.Date, documentNo types.Code) error {
			_, err := reports.Execute(reports.AdjustExchangeRatesID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
				Options: map[string]string{"posting_date": postingDate.String(), "ending_date": postingDate.String(), "document_no": documentNo.String()}})
			return err
		},
		Journals: true,
	})
	mustOK(t, err)
	return ftables.DBSource(db, "demo", database.DBTypeSQLite)
}

// layoutFiles: the layout files of backend/definitions/reports by report number
func layoutFiles(t *testing.T) map[int]string {
	t.Helper()
	files, err := filepath.Glob("../../definitions/reports/*.yaml")
	mustOK(t, err)
	byID := map[int]string{}
	for _, f := range files {
		id, err := strconv.Atoi(strings.SplitN(filepath.Base(f), "-", 2)[0])
		mustOK(t, err)
		byID[id] = f
	}
	return byID
}

// layoutFitOptions: the request page options some reports need to print (the others print
// the demo data with their defaults)
var layoutFitOptions = map[int]map[string]string{
	16:   {"starting_date": "2026-01-01", "ending_date": "2026-12-31", "journal_template": "GENERAL", "journal_batch": "DEFAULT"},
	17:   {"starting_date": "2026-01-01", "ending_date": "2026-12-31"},
	18:   {"starting_date": "2026-01-01", "ending_date": "2026-12-31"},
	20:   {"starting_date": "2026-07-01", "ending_date": "2026-09-30", "posting_date": "2026-10-06", "document_no": "SETTLE-TEST", "settlement_account": "2330", "show_tax_entries": "true"},
	1002: {"post_level": "1", "posting_date": "2026-10-06", "document_no": "INVT-FIT"},
	1401: {"bank_account": "B010", "last_check_no": "FIT1000", "test_print": "true"},
	5605: {"depreciation_book": "COMPANY", "starting_date": "2026-01-01", "ending_date": "2026-10-06"},
	5606: {"depreciation_book": "COMPANY", "starting_date": "2026-01-01", "ending_date": "2026-10-06"},
	5600: {"depreciation_book": "COMPANY", "starting_date": "2026-01-01", "ending_date": "2026-12-31", "date_field_1": "3", "date_field_2": "8", "amount_field_1": "2", "period_1": "1", "amount_field_2": "3", "period_2": "2", "amount_field_3": "10", "period_3": "3", "group_totals": "2", "print_per_fixed_asset": "true"},
	5610: {"depreciation_book": "COMPANY", "starting_date": "2026-01-01", "ending_date": "2026-12-31", "date_field_1": "2", "date_field_2": "8", "amount_field_1": "2", "period_1": "1", "amount_field_2": "3", "period_2": "1", "amount_field_3": "10", "period_3": "2", "group_totals": "2", "print_per_fixed_asset": "true"},
	5630: {"depreciation_book": "COMPANY", "starting_date": "2026-01-01", "ending_date": "2026-12-31", "period_1": "1", "period_2": "2", "period_3": "3", "group_totals": "2", "print_per_fixed_asset": "true"},
	5620: {"print_per_insurance_no": "true"},
	5607: {"depreciation_book": "COMPANY", "first_depreciation_date": "2026-10-31", "last_depreciation_date": "2027-12-31", "group_totals": "2", "print_per_fixed_asset": "true", "projected_disposal": "true", "print_amounts_per_date": "true", "posted_entries_from": "2026-01-01"},
}

// layoutFitFilters: the request filters some reports need (Check: one payment journal — the
// demo journals also hold fixed asset lines, which a check refuses)
var layoutFitFilters = map[int]map[string][]ftables.FieldFilter{
	1401: {"GenJnlLine": {{Field: "journal_template_name", Expression: "PAYMENT"}}},
}

// layoutFitSkipped: reports that cannot print without special input
var layoutFitSkipped = map[int]string{15: "Consolidation - Test File needs a consolidation file"}

// freeText: elements printing a free text (descriptions, names, addresses) — such a text may
// be cut when it is longer than its (reasonably wide) column. Everything else — amounts,
// quantities, dates, numbers, codes, options, captions, titles — must be printed whole.
var freeText = map[string]bool{
	"field:description":    true,
	"field:description_2":  true,
	"value:description":    true,
	"value:fa_description": true,
	"field:name":           true,
	"value:account_name":   true,
}

// brokenAllowed: column headings whose one long compound word (Norwegian, Danish) does not
// fit its column even at 80 % of the size — the heading prints whole over two lines, broken
// inside the word. The fixed asset and maintenance reports' rows are full: a column wide enough for
// "Anleggsbokføringskategori" (about 30 mm) over an 18 mm date or category would take it from
// the amounts.
var brokenAllowed = map[string]bool{
	"5603 text:FA_POSTING_DATE":     true,
	"5603 text:FA_POSTING_CATEGORY": true,
	"5604 text:FA_POSTING_DATE":     true,
	"5604 text:FA_POSTING_CATEGORY": true,
	"5604 text:G_L_ENTRY_NO":        true,
	"5633 text:FA_POSTING_DATE":     true,
	"5633 text:G_L_ENTRY_NO":        true,
	"5634 text:FA_POSTING_DATE":     true,
	"5634 text:G_L_ENTRY_NO":        true,
}

// Every printed report prints the demo data in every language without cutting a text that
// must be read whole (wider than its element: pdf.PlacedText.Full) and without breaking a
// word of a wrapped text (Result.Broken)
func TestPrintedLayoutsFit(t *testing.T) {
	src := fullDemoCompany(t)
	db := src.GetDB().(*sql.DB)
	files := layoutFiles(t)
	ids := make([]int, 0, len(files))
	for id := range files {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	type problem struct {
		report   int
		language string
		text     pdf.PlacedText
		broken   bool
	}
	var problems []problem
	printed := 0
	for _, language := range []string{"en-US", "nb-NO", "da-DK"} {
		fcodeunits.SetCurrentContext("", "", "demo", language)
		for _, id := range ids {
			if layoutFitSkipped[id] != "" {
				continue
			}
			tx, err := ftables.BeginTransaction(db)
			mustOK(t, err)
			result, err := reports.Execute(id, tx, "demo", database.DBTypeSQLite, reports.RunOptions{Output: reports.OutputPDF, Options: layoutFitOptions[id], Filters: layoutFitFilters[id]})
			_ = ftables.Rollback(tx)
			if err != nil {
				t.Errorf("report %d (%s) does not print the demo data: %v", id, language, err)
				continue
			}
			printed++
			for _, text := range result.Texts {
				if text.Full != "" && !freeText[text.Tag] {
					problems = append(problems, problem{id, language, text, false})
				}
			}
			for _, text := range result.Broken {
				if !freeText[text.Tag] && !brokenAllowed[fmt.Sprintf("%d %s", id, text.Tag)] {
					problems = append(problems, problem{id, language, text, true})
				}
			}
		}
		fcodeunits.ClearCurrentContext()
	}
	if printed == 0 {
		t.Fatal("no report printed")
	}
	seen := map[string]bool{}
	for _, p := range problems {
		key := fmt.Sprintf("%d %s %s", p.report, p.language, p.text.Tag)
		if seen[key] {
			continue
		}
		seen[key] = true
		if p.broken {
			t.Errorf("report %d (%s): %s breaks the word %q of %q (width %.1f mm)", p.report, p.language, p.text.Tag, p.text.Text, p.text.Full, p.text.Width)
		} else {
			t.Errorf("report %d (%s): %s cuts %q to %q (width %.1f mm)", p.report, p.language, p.text.Tag, p.text.Full, p.text.Text, p.text.Width)
		}
	}
}

// layoutFile is what TestLayoutElementsDoNotOverlap reads of a layout file
type layoutFile struct {
	Report struct {
		DataItems []layoutItem `yaml:"data_items"`
	} `yaml:"report"`
}

type layoutItem struct {
	Name     string `yaml:"name"`
	Sections []struct {
		Type     string          `yaml:"type"`
		Elements []layoutElement `yaml:"elements"`
	} `yaml:"sections"`
	DataItems []layoutItem `yaml:"data_items"`
}

type layoutElement map[string]interface{}

func (e layoutElement) number(key string) float64 {
	switch v := e[key].(type) {
	case int:
		return float64(v)
	case float64:
		return v
	}
	return 0
}

func (e layoutElement) String() string {
	for _, k := range []string{"text", "caption", "field", "value", "total", "total_value", "lines", "image"} {
		if v, ok := e[k]; ok {
			return fmt.Sprintf("%s:%v", k, v)
		}
	}
	if e["page_no"] == true {
		return "page_no"
	}
	return "?"
}

// Within a section, a text element ends before the next one on the same line starts (texts
// shown under conditions — print_if/print_unless — may share the room of others)
func TestLayoutElementsDoNotOverlap(t *testing.T) {
	for id, file := range layoutFiles(t) {
		raw, err := os.ReadFile(file)
		mustOK(t, err)
		var layout layoutFile
		mustOK(t, yaml.Unmarshal(raw, &layout))
		var walk func(items []layoutItem)
		walk = func(items []layoutItem) {
			for _, item := range items {
				for _, section := range item.Sections {
					rows := map[float64][]layoutElement{}
					for _, e := range section.Elements {
						if e["line"] == true || e["box"] == true || e["image"] != nil || e["print_if"] != nil || e["print_unless"] != nil || e.number("width") <= 0 {
							continue
						}
						rows[e.number("y")] = append(rows[e.number("y")], e)
					}
					for y, row := range rows {
						sort.Slice(row, func(i, j int) bool { return row[i].number("x") < row[j].number("x") })
						for i := 1; i < len(row); i++ {
							end := row[i-1].number("x") + row[i-1].number("width")
							if end > row[i].number("x")+0.01 {
								t.Errorf("report %d, %s %s y %g: %s (to %g mm) overlaps %s (from %g mm)", id, item.Name, section.Type, y, row[i-1], end, row[i], row[i].number("x"))
							}
						}
					}
				}
				walk(item.DataItems)
			}
		}
		walk(layout.Report.DataItems)
	}
}
