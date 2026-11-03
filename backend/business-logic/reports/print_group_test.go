package reports

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hansjlachmann/openerp/backend/foundation/types"
)

// groupTestID is the test's grouped report: sales rows (an Integer loop over computed rows)
// grouped by region and city
const groupTestID = 99951

type groupTestRow struct {
	region, city string
	amount       int64
}

type groupTest struct {
	rows []groupTestRow
}

func (r *groupTest) Definition() *Definition {
	return &Definition{
		DataItems: []*DataItem{{
			Name: "Row", Table: IntegerTable, GroupFields: []string{"region", "city"},
			OnPreDataItem: Trigger(func(rec *Integer, run *Run) error {
				rec.SetRange("number", 1, len(r.rows))
				return nil
			}),
			OnAfterGetRecord: Trigger(func(rec *Integer, run *Run) error {
				row := r.rows[rec.Number-1]
				if row.amount == 0 {
					return Skip() // not printed: no group changes
				}
				run.SetValue("region", row.region)
				run.SetValue("city", row.city)
				run.SetValue("amount", types.NewDecimalFromInt(row.amount))
				return nil
			}),
			OnGroupFooter: GroupTrigger(func(rec *Integer, group string, run *Run) error {
				// the group's share of its first row's amount: the totals of the group only
				run.SetValue("count", rec.Number)
				run.SetValue("doubled", run.GroupTotal("amount").Mul(types.NewDecimalFromInt(2)))
				if group == "city" && run.Value("city") == "" {
					return Skip()
				}
				return nil
			}),
		}},
	}
}

const groupTestLayout = `report:
  id: 99951
  name: Group Test
  page: {size: A4, margins: {top: 10, bottom: 10, left: 10, right: 10}}
  data_items:
    - name: Row
      new_page_per_group: %s
      sections:
        - type: header
          print_on_every_page: true
          height: 10
          elements:
            - {text: TITLE, x: 0, y: 0, width: 100}
        - type: group_header
          group: region
          print_on_every_page: true
          height: 10
          elements:
            - {value: region, x: 0, y: 0, width: 40, bold: true}
        - type: group_header
          group: city
          print_if: city
          height: 10
          elements:
            - {value: city, x: 5, y: 0, width: 40}
        - type: body
          height: %s
          elements:
            - {field: number, x: 10, y: 0, width: 20}
            - {value: amount, x: 100, y: 0, width: 40, align: right, decimal_places: "0:0"}
        - type: group_footer
          group: city
          height: 10
          elements:
            - {value: city, x: 5, y: 0, width: 40}
            - {total_value: amount, x: 100, y: 0, width: 40, align: right, decimal_places: "0:0"}
        - type: group_footer
          group: region
          height: 10
          elements:
            - {value: region, x: 0, y: 0, width: 40, bold: true}
            - {total_value: amount, x: 100, y: 0, width: 40, align: right, decimal_places: "0:0"}
            - {value: doubled, x: 150, y: 0, width: 40, align: right, decimal_places: "0:0"}
            - {value: count, x: 50, y: 0, width: 20}
        - type: footer
          height: 10
          elements:
            - {text: TOTAL, x: 0, y: 0, width: 40}
            - {total_value: amount, x: 100, y: 0, width: 40, align: right, decimal_places: "0:0"}
`

func runGroupTest(t *testing.T, rows []groupTestRow, newPagePerGroup bool, bodyHeight string) Result {
	t.Helper()
	Register(groupTestID, "group-test", func() Report { return &groupTest{rows: rows} })
	t.Cleanup(func() {
		registryMu.Lock()
		delete(registry, groupTestID)
		delete(names, groupTestID)
		registryMu.Unlock()
	})
	setTestLayout(t, groupTestID, "99951-group-test.yaml", strings.Replace(strings.Replace(groupTestLayout, "%s", map[bool]string{true: "true", false: "false"}[newPagePerGroup], 1), "%s", bodyHeight, 1))
	err := CheckLayout(testLayouts[groupTestID])
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(groupTestID, nil, "", "", RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

var groupTestRows = []groupTestRow{
	{"East", "Oslo", 10}, {"East", "Oslo", 20}, {"East", "Moss", 5}, {"East", "Moss", 0},
	{"West", "Moss", 7}, {"West", "", 1},
}

// Group headers before a group's first record, footers after its last with that record's
// values and the group's totals (inner group first); a skipped record changes no group; a
// group footer's trigger sees the group's totals and can leave the footer out
func TestPrintGroupHeadersAndFooters(t *testing.T) {
	result := runGroupTest(t, groupTestRows, false, "5")
	var got []string
	for _, text := range result.Texts {
		got = append(got, text.Text)
	}
	want := strings.Join([]string{
		"reports.99951.texts.TITLE",
		"East", "Oslo", "1", "10", "2", "20", "Oslo", "30",
		"Moss", "3", "5", "Moss", "5",
		"East", "35", "70", "3", // the region's last printed record is row 3
		"West", "Moss", "5", "7", "Moss", "7",
		"6", "1", // a blank city: no header, its footer skipped by the trigger
		"West", "8", "16", "6",
		"reports.99951.texts.TOTAL", "43",
	}, "|")
	if strings.Join(got, "|") != want {
		t.Errorf("printed\n %s\nwant\n %s", strings.Join(got, "|"), want)
	}
}

// A region per page (new_page_per_group), its footer on the page it ends on; a group going
// on to a new page repeats its print_on_every_page header there
func TestPrintGroupPages(t *testing.T) {
	result := runGroupTest(t, groupTestRows, true, "5")
	pages := pageTexts(result.Texts)
	if result.Pages != 2 || !contains(pages[1], "35") || contains(pages[1], "West") || !contains(pages[2], "West") || !contains(pages[2], "reports.99951.texts.TITLE") {
		t.Fatalf("%d pages: %q / %q", result.Pages, pages[1], pages[2])
	}

	// 30 rows of East over the pages: the region's header repeats on the second page, the
	// city's (not on every page) does not
	var rows []groupTestRow
	for i := 0; i < 30; i++ {
		rows = append(rows, groupTestRow{"East", "Oslo", 1})
	}
	result = runGroupTest(t, rows, false, "12")
	pages = pageTexts(result.Texts)
	if result.Pages < 2 {
		t.Fatalf("%d pages", result.Pages)
	}
	if !contains(pages[2], "East") || !contains(pages[2], "reports.99951.texts.TITLE") || pages[2][1] != "East" {
		t.Errorf("page 2 does not start with the headers: %q", pages[2])
	}
	if contains(pages[2][:2], "Oslo") {
		t.Errorf("page 2 repeats the city header: %q", pages[2])
	}
}

// setTestLayout gives a test report its layout
func setTestLayout(t *testing.T, id int, name, layoutYAML string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), name)
	err := os.WriteFile(file, []byte(layoutYAML), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := readLayout(file)
	if err != nil {
		t.Fatal(err)
	}
	testLayouts[id] = layout
	t.Cleanup(func() { delete(testLayouts, id) })
}

// A group section's group must be one of the data item's GroupFields
func TestCheckLayoutRefusesUnknownGroup(t *testing.T) {
	Register(groupTestID, "group-test", func() Report { return &groupTest{} })
	setTestLayout(t, groupTestID, "99951-group-test.yaml", "report:\n  id: 99951\n  data_items:\n    - name: Row\n      sections:\n        - type: group_footer\n          group: country\n")
	if CheckLayout(testLayouts[groupTestID]) == nil {
		t.Error("a group the data item does not have was taken")
	}
}
