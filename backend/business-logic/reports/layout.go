package reports

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
	"gopkg.in/yaml.v3"
)

// A printed report has a layout: backend/definitions/reports/<id>-<name>.yaml, the page and,
// per data item, its sections (classic Header, GroupHeader, Body, GroupFooter, Footer,
// TransHeader, TransFooter) with the elements printed in them:
//
//	report:
//	  id: 204
//	  page: {size: A4, orientation: portrait, margins: {top: 12, bottom: 12, left: 15, right: 12}}
//	  data_items:
//	    - name: PageLoop
//	      sections:
//	        - type: header
//	          print_on_every_page: true
//	          elements:
//	            - {lines: company_address, x: 0, y: 0, width: 80}
//	            - {text: QUOTE, x: 120, y: 0, width: 63, align: right, size: 16, bold: true}
//
// Positions and sizes are millimetres from the section's top left corner (inside the page
// margins). An element prints one of: text (a text of the report, reports.<id>.texts.<key>),
// caption (a field caption: <field> of the section's data item or <DataItem>.<field>), field
// (a field's value, formatted in the user's language), value (a report value the triggers set
// with Run.SetValue), total (the sum of a field of the data item over the records printed so
// far — in a group footer over the group's records), total_value (the same of a value its
// trigger sets per record), lines (a value holding lines, one under the other), image (a value holding a
// picture), page_no, line (a line of the width, or of the height: vertical), box.

// Layout is a printed report's layout
type Layout struct {
	ID        int               `yaml:"id"`
	Name      string            `yaml:"name"`
	Page      PageSetup         `yaml:"page"`
	FontSize  float64           `yaml:"font_size"` // points (default 9)
	DataItems []*LayoutDataItem `yaml:"data_items"`
	file      string            // the YAML file's name
	items     map[string]*LayoutDataItem
}

// PageSetup is the paper of a layout
type PageSetup struct {
	Size        string  `yaml:"size"`        // A4 (default) or Letter
	Orientation string  `yaml:"orientation"` // portrait (default) or landscape
	Margins     Margins `yaml:"margins"`
}

// Margins of the page in millimetres
type Margins struct {
	Top    float64 `yaml:"top"`
	Bottom float64 `yaml:"bottom"`
	Left   float64 `yaml:"left"`
	Right  float64 `yaml:"right"`
}

// LayoutDataItem is what a data item prints
type LayoutDataItem struct {
	Name string `yaml:"name"` // the data item's name in the report's Definition
	// NewPagePerRecord: each record (after the first) starts on a new page
	NewPagePerRecord bool `yaml:"new_page_per_record"`
	// NewPagePerGroup: each group of the outermost group field (after the first) starts on a
	// new page
	NewPagePerGroup bool       `yaml:"new_page_per_group"`
	Sections        []*Section `yaml:"sections"`
	totals          []string   // the fields its total elements sum
	totalValues     []string   // the values its total_value elements sum
}

// Section types
const (
	SectionHeader = "header" // before the first record; with print_on_every_page at the top of every page too
	SectionBody   = "body"   // for each record
	SectionFooter = "footer" // after the last record
	// The groups of the data item's GroupFields: a group header before the first record of a
	// group (with print_on_every_page at the top of every page the group goes on to too), a
	// group footer after its last record — printed with that record's fields and values
	SectionGroupHeader = "group_header"
	SectionGroupFooter = "group_footer"
	SectionTransHeader = "trans_header" // at the top of a page the data item goes on to (carried-over totals)
	SectionTransFooter = "trans_footer" // at the bottom of a page the data item goes on from
)

var sectionTypes = map[string]bool{SectionHeader: true, SectionGroupHeader: true, SectionBody: true, SectionGroupFooter: true, SectionFooter: true, SectionTransHeader: true, SectionTransFooter: true}

// Section is a part of the printout of a data item
type Section struct {
	Type             string     `yaml:"type"`
	Group            string     `yaml:"group"`               // group_header, group_footer: the group field (of the data item's GroupFields)
	PrintOnEveryPage bool       `yaml:"print_on_every_page"` // header, group_header: at the top of every page
	Height           float64    `yaml:"height"`              // millimetres; 0: as high as its elements
	PrintIf          string     `yaml:"print_if"`            // printed only when this value is set (not blank, not 0, true)
	PrintUnless      string     `yaml:"print_unless"`        // not printed when this value is set
	Elements         []*Element `yaml:"elements"`
}

// Element is a thing printed in a section
type Element struct {
	Text    string `yaml:"text"`
	Caption string `yaml:"caption"`
	Field   string `yaml:"field"`
	Value   string `yaml:"value"`
	Total   string `yaml:"total"`
	// TotalValue: the sum of a report value over the data item's records printed so far (the
	// value each record's trigger set)
	TotalValue string `yaml:"total_value"`
	Lines      string `yaml:"lines"`
	Image      string `yaml:"image"`
	PageNo     bool   `yaml:"page_no"`
	Line       bool   `yaml:"line"`
	Box        bool   `yaml:"box"`

	X      float64 `yaml:"x"`
	Y      float64 `yaml:"y"`
	Width  float64 `yaml:"width"`
	Height float64 `yaml:"height"`
	Align  string  `yaml:"align"` // left (default), right, center
	Bold   bool    `yaml:"bold"`
	Size   float64 `yaml:"size"` // points (default: the layout's font size)
	// DecimalPlaces of a decimal ("2:2": at least 2, at most 2 digits after the separator);
	// default: the field's, else 2:2 for values and totals
	DecimalPlaces string `yaml:"decimal_places"`
	BlankZero     bool   `yaml:"blank_zero"` // a zero prints nothing
	Wrap          bool   `yaml:"wrap"`       // a long text goes on in lines below (the section grows)
	PrintIf       string `yaml:"print_if"`
	PrintUnless   string `yaml:"print_unless"`
}

// kind is which of the element's sources is set ("" when none)
func (e *Element) kinds() []string {
	var kinds []string
	add := func(set bool, kind string) {
		if set {
			kinds = append(kinds, kind)
		}
	}
	add(e.Text != "", "text")
	add(e.Caption != "", "caption")
	add(e.Field != "", "field")
	add(e.Value != "", "value")
	add(e.Total != "", "total")
	add(e.TotalValue != "", "total_value")
	add(e.Lines != "", "lines")
	add(e.Image != "", "image")
	add(e.PageNo, "page_no")
	add(e.Line, "line")
	add(e.Box, "box")
	return kinds
}

func (e *Element) kind() string {
	kinds := e.kinds()
	if len(kinds) != 1 {
		return ""
	}
	return kinds[0]
}

var decimalPlacesPattern = regexp.MustCompile(`^\d+:\d+$`)

// item returns the layout of a data item (nil when the data item prints nothing)
func (l *Layout) item(name string) *LayoutDataItem {
	return l.items[name]
}

// sections returns the data item's sections of a type
func (i *LayoutDataItem) sections(sectionType string) []*Section {
	var sections []*Section
	for _, s := range i.Sections {
		if s.Type == sectionType {
			sections = append(sections, s)
		}
	}
	return sections
}

// groupSections returns the data item's sections of a type of a group
func (i *LayoutDataItem) groupSections(sectionType, group string) []*Section {
	var sections []*Section
	for _, s := range i.Sections {
		if s.Type == sectionType && s.Group == group {
			sections = append(sections, s)
		}
	}
	return sections
}

var (
	layoutsOnce sync.Once
	layouts     map[int]*Layout
	layoutsErr  error
)

// LayoutsDir is where the layouts are read from (set by tests; default
// <project root>/backend/definitions/reports)
var LayoutsDir string

// loadLayouts reads every layout once
func loadLayouts() (map[int]*Layout, error) {
	layoutsOnce.Do(func() {
		dir := LayoutsDir
		if dir == "" {
			root, err := projectRoot()
			if err != nil {
				layoutsErr = err
				return
			}
			dir = filepath.Join(root, "backend", "definitions", "reports")
		}
		layouts, layoutsErr = ReadLayouts(dir)
	})
	return layouts, layoutsErr
}

// ReadLayouts reads the layout files of a directory (none when it does not exist)
func ReadLayouts(dir string) (map[int]*Layout, error) {
	result := map[int]*Layout{}
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	for _, file := range files {
		layout, err := readLayout(file)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(file), err)
		}
		other, ok := result[layout.ID]
		if ok {
			return nil, fmt.Errorf("%s: report %d already has the layout %s", filepath.Base(file), layout.ID, other.file)
		}
		result[layout.ID] = layout
	}
	return result, nil
}

// readLayout reads and checks the form of a layout file (CheckLayout checks it against its
// report)
func readLayout(file string) (*Layout, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Report *Layout `yaml:"report"`
	}
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	err = dec.Decode(&doc)
	if err != nil {
		return nil, err
	}
	layout := doc.Report
	if layout == nil {
		return nil, errors.New("the file has no report section")
	}
	layout.file = filepath.Base(file)
	prefix, _, _ := strings.Cut(layout.file, "-")
	id, err := strconv.Atoi(prefix)
	if err != nil || id != layout.ID {
		return nil, fmt.Errorf("the file name must start with the report number %d", layout.ID)
	}
	switch strings.ToLower(layout.Page.Size) {
	case "", "a4", "letter":
	default:
		return nil, fmt.Errorf("page size %q: A4 or Letter", layout.Page.Size)
	}
	switch strings.ToLower(layout.Page.Orientation) {
	case "", "portrait", "landscape":
	default:
		return nil, fmt.Errorf("orientation %q: portrait or landscape", layout.Page.Orientation)
	}
	if layout.FontSize == 0 {
		layout.FontSize = 9
	}
	layout.items = map[string]*LayoutDataItem{}
	for _, item := range layout.DataItems {
		if _, ok := layout.items[item.Name]; ok {
			return nil, fmt.Errorf("data item %s twice", item.Name)
		}
		layout.items[item.Name] = item
		totals := map[string]bool{}
		for i, s := range item.Sections {
			if !sectionTypes[s.Type] {
				return nil, fmt.Errorf("data item %s: section %d: type %q", item.Name, i+1, s.Type)
			}
			if s.PrintOnEveryPage && s.Type != SectionHeader && s.Type != SectionGroupHeader {
				return nil, fmt.Errorf("data item %s: section %d: print_on_every_page is for headers", item.Name, i+1)
			}
			grouped := s.Type == SectionGroupHeader || s.Type == SectionGroupFooter
			if grouped != (s.Group != "") {
				return nil, fmt.Errorf("data item %s: section %d (%s): group is for group_header and group_footer, which need it", item.Name, i+1, s.Type)
			}
			for j, e := range s.Elements {
				kinds := e.kinds()
				if len(kinds) != 1 {
					return nil, fmt.Errorf("data item %s: section %d (%s): element %d: one of text, caption, field, value, total, total_value, lines, image, page_no, line, box (has %v)", item.Name, i+1, s.Type, j+1, kinds)
				}
				switch e.Align {
				case "", "left", "right", "center":
				default:
					return nil, fmt.Errorf("data item %s: section %d: element %d: align %q", item.Name, i+1, j+1, e.Align)
				}
				if e.DecimalPlaces != "" && !decimalPlacesPattern.MatchString(e.DecimalPlaces) {
					return nil, fmt.Errorf("data item %s: section %d: element %d: decimal_places %q", item.Name, i+1, j+1, e.DecimalPlaces)
				}
				if (e.Image != "" || e.Box) && (e.Width <= 0 || e.Height <= 0) {
					return nil, fmt.Errorf("data item %s: section %d: element %d: an image or box needs width and height", item.Name, i+1, j+1)
				}
				if e.Total != "" && !totals["field:"+e.Total] {
					totals["field:"+e.Total] = true
					item.totals = append(item.totals, e.Total)
				}
				if e.TotalValue != "" && !totals["value:"+e.TotalValue] {
					totals["value:"+e.TotalValue] = true
					item.totalValues = append(item.totalValues, e.TotalValue)
				}
			}
		}
	}
	return layout, nil
}

// CheckLayout checks a layout against its report: its data items exist, the fields its
// elements print exist in their tables
func CheckLayout(layout *Layout) error {
	factory, ok := Get(layout.ID)
	if !ok {
		return fmt.Errorf("layout %s: report %d does not exist", layout.file, layout.ID)
	}
	def := factory().Definition()
	tables := map[string]string{}
	groups := map[string][]string{}
	for _, item := range allDataItems(def.DataItems) {
		tables[item.Name] = item.Table
		groups[item.Name] = item.GroupFields
	}
	fields := func(table string) map[string]bool {
		names := map[string]bool{}
		if table == IntegerTable {
			names["number"] = true
			return names
		}
		f, ok := gtables.GetTableFactory(table)
		if !ok {
			return names
		}
		for _, field := range f().GetFields() {
			names[field.Name] = true
		}
		return names
	}
	for _, item := range layout.DataItems {
		_, ok := tables[item.Name]
		if !ok {
			return fmt.Errorf("layout %s: report %d has no data item %s", layout.file, layout.ID, item.Name)
		}
		if item.NewPagePerGroup && len(groups[item.Name]) == 0 {
			return fmt.Errorf("layout %s: %s: new_page_per_group without GroupFields", layout.file, item.Name)
		}
		for _, s := range item.Sections {
			if s.Group != "" && !slices.Contains(groups[item.Name], s.Group) {
				return fmt.Errorf("layout %s: %s: %s section of group %s: the data item's GroupFields are %v", layout.file, item.Name, s.Type, s.Group, groups[item.Name])
			}
			for _, e := range s.Elements {
				for _, ref := range []string{e.Field, e.Caption, e.Total} {
					if ref == "" {
						continue
					}
					refItem, field := item.Name, ref
					name, f, ok := strings.Cut(ref, ".")
					if ok {
						refItem, field = name, f
					}
					refTable, ok := tables[refItem]
					if !ok || layout.item(refItem) == nil {
						return fmt.Errorf("layout %s: %s: %s: no data item %s in the layout", layout.file, item.Name, ref, refItem)
					}
					if !fields(refTable)[field] {
						return fmt.Errorf("layout %s: %s: %s: table %s has no field %s", layout.file, item.Name, ref, refTable, field)
					}
				}
			}
		}
	}
	return nil
}

// Layouts returns the layouts read (all reports with a layout are printed)
func Layouts() (map[int]*Layout, error) {
	return loadLayouts()
}

// testLayouts: layouts tests give their reports (setTestLayout)
var testLayouts = map[int]*Layout{}

// layoutOf returns a report's layout; nil for a processing-only report
func layoutOf(id int) (*Layout, error) {
	layout, ok := testLayouts[id]
	if ok {
		return layout, nil
	}
	all, err := loadLayouts()
	if err != nil {
		return nil, err
	}
	return all[id], nil
}

// Printed reports whether a report has a layout (prints)
func Printed(id int) bool {
	layout, err := layoutOf(id)
	return err == nil && layout != nil
}

// TextKeys returns the texts a layout prints (reports.<id>.texts.<key>), sorted
func (l *Layout) TextKeys() []string {
	seen := map[string]bool{}
	var keys []string
	for _, item := range l.DataItems {
		for _, s := range item.Sections {
			for _, e := range s.Elements {
				if e.Text != "" && !seen[e.Text] {
					seen[e.Text] = true
					keys = append(keys, e.Text)
				}
			}
		}
	}
	sort.Strings(keys)
	return keys
}

// projectRoot is the directory with go.mod, from the working directory up (the release image
// has go.mod as the marker too)
func projectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		_, err := os.Stat(filepath.Join(dir, "go.mod"))
		if err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("project root not found (no go.mod)")
		}
		dir = parent
	}
}
