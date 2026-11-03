package reports

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/pdf"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
)

// printTestID is the test's printed report: copies of a document with lines (Integer loops)
const printTestID = 99950

type printTest struct {
	copies int
	lines  int
}

func (r *printTest) Definition() *Definition {
	return &Definition{
		DataItems: []*DataItem{{
			Name: "CopyLoop", Table: IntegerTable,
			OnPreDataItem: Trigger(func(rec *Integer, run *Run) error {
				rec.SetRange("number", 1, r.copies)
				return nil
			}),
			OnAfterGetRecord: Trigger(func(rec *Integer, run *Run) error {
				run.SetPageNo(1)
				run.SetValue("copy", rec.Number > 1)
				run.SetValue("address", [3]string{"Nordkapp Kontor AS", "", "0159 Oslo"})
				return nil
			}),
			DataItems: []*DataItem{{
				Name: "Line", Table: IntegerTable,
				OnPreDataItem: Trigger(func(rec *Integer, run *Run) error {
					rec.SetRange("number", 1, r.lines)
					return nil
				}),
				OnAfterGetRecord: Trigger(func(rec *Integer, run *Run) error {
					if rec.Number == 3 {
						return Skip() // not printed, not in the totals
					}
					run.SetValue("amount", types.NewDecimalFromInt(int64(rec.Number)*1000).Div(types.NewDecimalFromInt(8)))
					return nil
				}),
			}},
		}},
	}
}

const printTestLayout = `report:
  id: 99950
  name: Print Test
  page: {size: A4, margins: {top: 10, bottom: 10, left: 10, right: 10}}
  data_items:
    - name: CopyLoop
      new_page_per_record: true
      sections:
        - type: header
          print_on_every_page: true
          height: 20
          elements:
            - {text: TITLE, x: 0, y: 0, width: 100, bold: true}
            - {text: COPY, x: 100, y: 0, width: 40, print_if: copy}
            - {page_no: true, x: 160, y: 0, width: 30, align: right}
            - {lines: address, x: 0, y: 5, width: 80}
    - name: Line
      sections:
        - type: trans_header
          elements:
            - {text: FROM_PAGE, x: 0, y: 0, width: 60}
            - {total: number, x: 150, y: 0, width: 40, align: right, decimal_places: "0:0"}
        - type: body
          height: 10
          elements:
            - {field: number, x: 0, y: 0, width: 20}
            - {value: amount, x: 100, y: 0, width: 40, align: right}
            - {field: CopyLoop.number, x: 175, y: 0, width: 10}
        - type: trans_footer
          elements:
            - {text: TO_PAGE, x: 0, y: 0, width: 60}
            - {total: number, x: 150, y: 0, width: 40, align: right, decimal_places: "0:0"}
        - type: footer
          elements:
            - {line: true, x: 0, y: 0}
            - {text: TOTAL, x: 0, y: 1, width: 60, bold: true}
            - {total: number, x: 150, y: 1, width: 40, align: right}
            - {total_value: amount, x: 100, y: 1, width: 40, align: right}
`

func setPrintTestLayout(t *testing.T, layoutYAML string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "99950-print-test.yaml")
	err := os.WriteFile(file, []byte(layoutYAML), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := readLayout(file)
	if err != nil {
		t.Fatal(err)
	}
	testLayouts[printTestID] = layout
	t.Cleanup(func() { delete(testLayouts, printTestID) })
}

func runPrintTest(t *testing.T, copies, lines int) Result {
	t.Helper()
	Register(printTestID, "print-test", func() Report { return &printTest{copies: copies, lines: lines} })
	setPrintTestLayout(t, printTestLayout)
	err := CheckLayout(testLayouts[printTestID])
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(printTestID, nil, "", "", RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(result.PDF, []byte("%PDF-")) {
		t.Fatalf("no PDF: %q", result.PDF)
	}
	return result
}

// pageTexts: the texts printed on each page
func pageTexts(texts []pdf.PlacedText) map[int][]string {
	pages := map[int][]string{}
	for _, text := range texts {
		pages[text.Page] = append(pages[text.Page], text.Text)
	}
	return pages
}

func contains(list []string, text string) bool {
	for _, item := range list {
		if item == text {
			return true
		}
	}
	return false
}

func TestPrintPagesHeadersAndCarriedTotals(t *testing.T) {
	result := runPrintTest(t, 1, 40)
	// Page: 297 − 20 margins; header 20, lines 10 each, trans footer reserved: 24 lines on
	// page 1, the rest on page 2
	if result.Pages != 2 {
		t.Fatalf("pages %d, want 2", result.Pages)
	}
	if result.Output != OutputPDF || result.FileName != "print-test.pdf" {
		t.Errorf("output %q, file %q", result.Output, result.FileName)
	}
	pages := pageTexts(result.Texts)
	for page := 1; page <= 2; page++ {
		if !contains(pages[page], "reports.99950.texts.TITLE") || !contains(pages[page], "Nordkapp Kontor AS") || !contains(pages[page], "0159 Oslo") {
			t.Errorf("page %d without the header: %q", page, pages[page])
		}
		if contains(pages[page], "reports.99950.texts.COPY") {
			t.Errorf("page %d of the original says COPY", page)
		}
	}
	if !contains(pages[1], "1") || contains(pages[1], "2") && !contains(pages[2], "2") {
		t.Errorf("page numbers: %q / %q", pages[1], pages[2])
	}
	// Line 3 is skipped; line 1's amount 125.00, line 2 250.00
	if contains(pages[1], "3") && !contains(pages[1], "reports.99950.texts.TO_PAGE") {
		t.Errorf("skipped line printed: %q", pages[1])
	}
	if !contains(pages[1], "125.00") || !contains(pages[1], "250.00") {
		t.Errorf("amounts: %q", pages[1])
	}
	// The lines printed on page 1 add up in the trans footer, page 2 starts with them
	var onPage1 int
	sum := 0
	for _, text := range result.Texts {
		if text.Page == 1 && text.X == 10 && text.Text != "" && text.Text[0] >= '0' && text.Text[0] <= '9' && text.Y >= 30 {
			onPage1++
			n := 0
			for _, c := range text.Text {
				n = n*10 + int(c-'0')
			}
			sum += n
		}
	}
	total := formatDecimal(types.NewDecimalFromInt(int64(sum)), "0:0", "en-US")
	if !contains(pages[1], "reports.99950.texts.TO_PAGE") || !contains(pages[1], total) {
		t.Errorf("page 1 trans footer (%d lines, total %s): %q", onPage1, total, pages[1])
	}
	if !contains(pages[2], "reports.99950.texts.FROM_PAGE") || !contains(pages[2], total) {
		t.Errorf("page 2 trans header (total %s): %q", total, pages[2])
	}
	if contains(pages[2], "reports.99950.texts.TO_PAGE") || contains(pages[1], "reports.99950.texts.FROM_PAGE") {
		t.Errorf("trans sections on the wrong page: %q / %q", pages[1], pages[2])
	}
	// The footer after the last line: 1+2+4+…+40 = 820 − 3
	// … and the amounts: 817 × 125
	if !contains(pages[2], "reports.99950.texts.TOTAL") || !contains(pages[2], "817.00") || !contains(pages[2], "102,125.00") {
		t.Errorf("footer: %q", pages[2])
	}
}

func TestPrintCopiesStartOnPageOne(t *testing.T) {
	result := runPrintTest(t, 2, 5)
	if result.Pages != 2 {
		t.Fatalf("pages %d, want 2 (a page per copy)", result.Pages)
	}
	pages := pageTexts(result.Texts)
	if contains(pages[1], "reports.99950.texts.COPY") || !contains(pages[2], "reports.99950.texts.COPY") {
		t.Errorf("COPY: %q / %q", pages[1], pages[2])
	}
	for page := 1; page <= 2; page++ {
		var pageNo string
		for _, text := range result.Texts {
			if text.Page == page && text.X == 170 {
				pageNo = text.Text
			}
		}
		if pageNo != "1" {
			t.Errorf("copy %d prints page number %q, want 1", page, pageNo)
		}
		// Each copy has its own totals: 1+2+4+5
		if !contains(pages[page], "12.00") {
			t.Errorf("copy %d footer: %q", page, pages[page])
		}
	}
	if !contains(pages[2], "2") {
		t.Errorf("copy 2's lines show CopyLoop.number 2: %q", pages[2])
	}
}

func TestPrintNothingGivesAnEmptyPage(t *testing.T) {
	result := runPrintTest(t, 0, 5)
	if result.Pages != 1 || len(result.Texts) != 0 {
		t.Errorf("pages %d, texts %v", result.Pages, result.Texts)
	}
}

func TestReadLayoutRefusesMistakes(t *testing.T) {
	cases := map[string]string{
		"two sources":     "report:\n  id: 99950\n  data_items:\n    - name: Line\n      sections:\n        - type: body\n          elements:\n            - {text: A, field: number}\n",
		"no source":       "report:\n  id: 99950\n  data_items:\n    - name: Line\n      sections:\n        - type: body\n          elements:\n            - {x: 1}\n",
		"section type":    "report:\n  id: 99950\n  data_items:\n    - name: Line\n      sections:\n        - type: group\n",
		"no group":        "report:\n  id: 99950\n  data_items:\n    - name: Line\n      sections:\n        - type: group_footer\n",
		"group of body":   "report:\n  id: 99950\n  data_items:\n    - name: Line\n      sections:\n        - type: body\n          group: number\n",
		"unknown key":     "report:\n  id: 99950\n  colour: red\n",
		"wrong number":    "report:\n  id: 12\n",
		"every page":      "report:\n  id: 99950\n  data_items:\n    - name: Line\n      sections:\n        - type: body\n          print_on_every_page: true\n",
		"decimals":        "report:\n  id: 99950\n  data_items:\n    - name: Line\n      sections:\n        - type: body\n          elements:\n            - {field: number, decimal_places: two}\n",
		"image size":      "report:\n  id: 99950\n  data_items:\n    - name: Line\n      sections:\n        - type: body\n          elements:\n            - {image: logo, width: 10}\n",
		"page size":       "report:\n  id: 99950\n  page: {size: A3}\n",
		"align":           "report:\n  id: 99950\n  data_items:\n    - name: Line\n      sections:\n        - type: body\n          elements:\n            - {text: A, align: middle}\n",
		"data item twice": "report:\n  id: 99950\n  data_items:\n    - name: Line\n    - name: Line\n",
	}
	for name, text := range cases {
		file := filepath.Join(t.TempDir(), "99950-x.yaml")
		_ = os.WriteFile(file, []byte(text), 0o600)
		_, err := readLayout(file)
		if err == nil {
			t.Errorf("%s: taken", name)
		}
	}
}

func TestCheckLayoutRefusesUnknownFields(t *testing.T) {
	Register(printTestID, "print-test", func() Report { return &printTest{} })
	for _, element := range []string{"{field: amount}", "{field: Header.number}", "{caption: CopyLoop.no}", "{total: amount}"} {
		setPrintTestLayout(t, "report:\n  id: 99950\n  data_items:\n    - name: Line\n      sections:\n        - type: body\n          elements:\n            - "+element+"\n")
		err := CheckLayout(testLayouts[printTestID])
		if err == nil {
			t.Errorf("%s taken", element)
		}
	}
	setPrintTestLayout(t, "report:\n  id: 99950\n  data_items:\n    - name: Header\n")
	if CheckLayout(testLayouts[printTestID]) == nil {
		t.Error("a data item the report does not have was taken")
	}
}

func TestFormatInLanguages(t *testing.T) {
	d := types.MustDecimal("-1234567.5")
	cases := []struct{ language, places, want string }{
		{"en-US", "2:2", "-1,234,567.50"},
		{"nb-NO", "2:2", "-1 234 567,50"},
		{"da-DK", "2:2", "-1.234.567,50"},
		{"en-US", "0:5", "-1,234,567.5"},
		{"en-US", "0:0", "-1,234,568"},
		{"xx-XX", "", "-1,234,567.5"},
	}
	for _, c := range cases {
		got := formatDecimal(d, c.places, c.language)
		if got != c.want {
			t.Errorf("%s %s: %q, want %q", c.language, c.places, got, c.want)
		}
	}
	if got := formatDecimal(types.MustDecimal("-0.001"), "2:2", "en-US"); got != "0.00" {
		t.Errorf("-0.001: %q", got)
	}
	if got := formatDecimal(types.MustDecimal("999.995"), "2:2", "en-US"); got != "1,000.00" {
		t.Errorf("999.995: %q", got)
	}
	date, _ := types.NewDateFromString("2026-10-28")
	if formatDate(date, "en-US") != "10/28/2026" || formatDate(date, "nb-NO") != "28.10.2026" || formatDate(types.Date{}, "nb-NO") != "" {
		t.Errorf("dates: %q %q", formatDate(date, "en-US"), formatDate(date, "nb-NO"))
	}
	if !isSet([8]string{"", "x"}) || isSet([8]string{}) || isSet("  ") || !isSet(true) || isSet(types.Decimal{}) {
		t.Error("isSet")
	}
	if strings.Join(linesOf([3]string{"a", "", "b"}), "|") != "a||b" || linesOf(42) != nil {
		t.Error("linesOf")
	}
}

// The user's own date and number formats (My Settings) win over the language's
func TestFormatWithRegionalSettings(t *testing.T) {
	fcodeunits.SetCurrentContext("", "", "", "en-US")
	defer fcodeunits.ClearCurrentContext()
	ctx := fcodeunits.GetCurrentContext()
	ctx.Regional = fcodeunits.Regional{DateFormat: "dd.mm.yy", NumberFormat: "1 234,56"}
	date, _ := types.NewDateFromString("2026-10-28")
	if got := formatDate(date, "en-US"); got != "28.10.26" {
		t.Errorf("date %q", got)
	}
	if got := formatDecimal(types.MustDecimal("1234.5"), "2:2", "en-US"); got != "1 234,50" {
		t.Errorf("decimal %q", got)
	}
}
