package reports

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/pdf"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
)

// Output of a printed report: what the user chose on the request page
const (
	OutputPreview = "preview" // shown in the browser (the run is a preview: CurrReport.PREVIEW)
	OutputPrint   = "print"   // sent to the browser's print dialog
	OutputPDF     = "pdf"     // downloaded as a PDF file
)

// printer prints a report's data items by its layout: pages, sections, page breaks with the
// headers repeated and the carried-over totals (trans footer/header)
type printer struct {
	run        *Run
	layout     *Layout
	doc        *pdf.Document
	language   string
	y          float64 // where the next section goes
	pageOpen   bool
	newPage    bool // the next section starts a new page (NewPage, NewPagePerRecord)
	pageNo     int  // CurrReport.PAGENO
	nextPageNo int  // > 0: the next page gets this number (SetPageNo before it starts)
	frames     []*frame
	// pageHeaders: the header sections the current page started with (not printed again)
	pageHeaders map[*Section]bool
	// footer: the totals of the group whose footer prints (Run.GroupTotal)
	footer *sums
}

// sums are the totals of a data item or a group: field → its total, value → its total
// (total_value)
type sums struct {
	fields map[string]types.Decimal
	values map[string]types.Decimal
}

func newSums() *sums {
	return &sums{fields: map[string]types.Decimal{}, values: map[string]types.Decimal{}}
}

// group is a group of a data item's GroupFields: the value of its field, its totals
type group struct {
	field string
	key   string
	open  bool // between its header and its footer (its headers repeat on a new page)
	sums  *sums
}

// frame is a data item being printed
type frame struct {
	item     *LayoutDataItem
	dataItem *DataItem
	rec      interface{}
	records  int // records got (not skipped)
	bodies   int // body sections printed: a page break carries it over (trans sections)
	// headersDone: the headers before the first record are printed (a later page repeats only
	// those printed on every page)
	headersDone bool
	sums        *sums
	// groups: the open groups of the data item's GroupFields, outermost first
	groups []*group
	// lastRec and lastValues: the last record printed and the values it was printed with
	// (what its groups' footers print)
	lastRec    interface{}
	lastValues map[string]interface{}
}

func newPrinter(run *Run, layout *Layout) (*printer, error) {
	m := layout.Page.Margins
	doc, err := pdf.New(layout.Page.Size, strings.EqualFold(layout.Page.Orientation, "landscape"), pdf.Margins{Top: m.Top, Bottom: m.Bottom, Left: m.Left, Right: m.Right})
	if err != nil {
		return nil, err
	}
	return &printer{run: run, layout: layout, doc: doc, language: fcodeunits.CurrentLanguage()}, nil
}

// beginItem: a data item starts its loop (nil when its layout prints nothing)
func (p *printer) beginItem(dataItem *DataItem, rec interface{}) *frame {
	item := p.layout.item(dataItem.Name)
	if item == nil {
		return nil
	}
	f := &frame{item: item, dataItem: dataItem, rec: rec, sums: newSums()}
	for _, field := range dataItem.GroupFields {
		f.groups = append(f.groups, &group{field: field, sums: newSums()})
	}
	p.frames = append(p.frames, f)
	return f
}

// beforeRecord: a record is read — with NewPagePerRecord it goes on a new page (before its
// OnAfterGetRecord, so the trigger can set the page number); the returned function takes that
// back when the trigger skips the record
func (p *printer) beforeRecord(f *frame) func() {
	if f == nil || !f.item.NewPagePerRecord || f.records == 0 {
		return func() {}
	}
	before := p.newPage
	p.newPage = true
	return func() { p.newPage = before }
}

// record prints a record the trigger did not skip: the data item's headers before its first
// record; where a group ends the footers of the groups before, where one starts its headers;
// then its body sections; the totals (of the data item and its groups) add its fields
func (p *printer) record(f *frame) error {
	if f == nil {
		return nil
	}
	f.records++
	// the outermost group whose value changes (all of them at the first record)
	changed := len(f.groups)
	for i, g := range f.groups {
		key := p.groupKey(f, g.field)
		if f.records == 1 || key != g.key {
			changed = i
			break
		}
	}
	if f.records == 1 {
		for _, s := range f.item.sections(SectionHeader) {
			err := p.output(f, s)
			if err != nil {
				return err
			}
		}
		f.headersDone = true
	} else if changed < len(f.groups) {
		// the groups that end print their footers on the page they end on (a new page asked
		// for the record comes after them)
		pending := p.newPage
		p.newPage = false
		err := p.groupFooters(f, changed)
		if err != nil {
			return err
		}
		p.newPage = pending || (changed == 0 && f.item.NewPagePerGroup)
	}
	for _, g := range f.groups[changed:] {
		g.open = false
	}
	for _, g := range f.groups[changed:] {
		g.key = p.groupKey(f, g.field)
		g.sums = newSums()
		g.open = true
		for _, s := range f.item.groupSections(SectionGroupHeader, g.field) {
			err := p.output(f, s)
			if err != nil {
				return err
			}
		}
	}
	for _, s := range f.item.sections(SectionBody) {
		err := p.output(f, s)
		if err != nil {
			return err
		}
	}
	f.bodies++
	all := []*sums{f.sums}
	for _, g := range f.groups {
		all = append(all, g.sums)
	}
	for _, field := range f.item.totals {
		value, _ := fieldValue(f.rec, field)
		for _, t := range all {
			t.fields[field] = addTo(t.fields[field], value)
		}
	}
	for _, name := range f.item.totalValues {
		for _, t := range all {
			t.values[name] = addTo(t.values[name], p.run.values[name])
		}
	}
	for _, name := range f.dataItem.TotalValues {
		if slices.Contains(f.item.totalValues, name) {
			continue
		}
		for _, t := range all {
			t.values[name] = addTo(t.values[name], p.run.values[name])
		}
	}
	if len(f.groups) > 0 {
		f.lastRec = copyRecord(f.rec)
		f.lastValues = make(map[string]interface{}, len(p.run.values))
		for name, value := range p.run.values {
			f.lastValues[name] = value
		}
	}
	return nil
}

// addTo adds a decimal or whole number to a total
func addTo(total types.Decimal, value interface{}) types.Decimal {
	switch v := value.(type) {
	case types.Decimal:
		return total.Add(v)
	case int:
		return total.Add(types.NewDecimalFromInt(int64(v)))
	}
	return total
}

// groupKey: the value of a group field — of the data item's record, else the report value
func (p *printer) groupKey(f *frame, field string) string {
	values, ok := f.rec.(interface{ ToMap() map[string]interface{} })
	if ok {
		value, ok := values.ToMap()[field]
		if ok {
			return fmt.Sprint(value)
		}
	}
	return fmt.Sprint(p.run.values[field])
}

// groupFooters prints the footers of the groups from level from inwards (the innermost first)
// with the record they ended with: its fields and the values it was printed with
func (p *printer) groupFooters(f *frame, from int) error {
	rec, values := f.rec, p.run.values
	f.rec, p.run.values = f.lastRec, f.lastValues
	defer func() {
		f.rec, p.run.values, p.footer = rec, values, nil
	}()
	for i := len(f.groups) - 1; i >= from; i-- {
		g := f.groups[i]
		p.footer = g.sums
		if f.dataItem.OnGroupFooter != nil {
			err := f.dataItem.OnGroupFooter(f.rec, g.field, p.run)
			if errors.Is(err, errSkip) {
				continue
			}
			if err != nil {
				return err
			}
		}
		for _, s := range f.item.groupSections(SectionGroupFooter, g.field) {
			err := p.output(f, s)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// copyRecord copies a record (a pointer to a struct) as it is now
func copyRecord(rec interface{}) interface{} {
	v := reflect.ValueOf(rec)
	if v.Kind() != reflect.Pointer || v.Elem().Kind() != reflect.Struct {
		return rec
	}
	c := reflect.New(v.Elem().Type())
	c.Elem().Set(v.Elem())
	return c.Interface()
}

// endItem: the data item's loop ended — the footers of its groups and its own after its last
// record
func (p *printer) endItem(f *frame) error {
	if f == nil {
		return nil
	}
	var err error
	if f.records > 0 {
		err = p.groupFooters(f, 0)
		if err == nil {
			for _, s := range f.item.sections(SectionFooter) {
				err = p.output(f, s)
				if err != nil {
					break
				}
			}
		}
	}
	p.frames = p.frames[:len(p.frames)-1]
	return err
}

// output prints a section where the page goes on, on a new page when it does not fit (or a
// new page was asked for)
func (p *printer) output(f *frame, s *Section) error {
	if !p.shown(s.PrintIf, s.PrintUnless) {
		return nil
	}
	height, draw := p.prepare(f, s)
	if !p.pageOpen || p.newPage || p.y+height > p.limit() {
		err := p.breakPage()
		if err != nil {
			return err
		}
	}
	if (s.Type == SectionHeader || s.Type == SectionGroupHeader) && p.pageHeaders[s] {
		return nil // the page started with it
	}
	err := draw(p.y)
	p.y += height
	return err
}

// place prints a section at the current position without checking the room left (the parts of
// a page break)
func (p *printer) place(f *frame, s *Section) error {
	if !p.shown(s.PrintIf, s.PrintUnless) {
		return nil
	}
	height, draw := p.prepare(f, s)
	err := draw(p.y)
	p.y += height
	return err
}

// limit is how far down the page sections go: the bottom margin, less the trans footers of the
// data items that carry over
func (p *printer) limit() float64 {
	limit := p.doc.Height() - p.doc.Margins().Bottom
	for _, f := range p.frames {
		if f.bodies == 0 {
			continue
		}
		for _, s := range f.item.sections(SectionTransFooter) {
			if p.shown(s.PrintIf, s.PrintUnless) {
				height, _ := p.prepare(f, s)
				limit -= height
			}
		}
	}
	return limit
}

// breakPage ends the page (the trans footers of the data items going on) and starts the next
func (p *printer) breakPage() error {
	if p.pageOpen {
		for i := len(p.frames) - 1; i >= 0; i-- {
			f := p.frames[i]
			if f.bodies == 0 {
				continue
			}
			for _, s := range f.item.sections(SectionTransFooter) {
				err := p.place(f, s)
				if err != nil {
					return err
				}
			}
		}
	}
	return p.startPage()
}

// startPage starts a page: the headers printed on every page of the data items in progress (all
// the headers of one whose first record starts the page, in their order), then the trans headers
// of those carried over
func (p *printer) startPage() error {
	p.doc.AddPage()
	p.pageOpen = true
	p.newPage = false
	if p.nextPageNo > 0 {
		p.pageNo = p.nextPageNo
		p.nextPageNo = 0
	} else {
		p.pageNo++
	}
	p.y = p.doc.Margins().Top
	p.pageHeaders = map[*Section]bool{}
	for _, f := range p.frames {
		if f.records == 0 {
			continue
		}
		// the headers on every page; on the data item's first page all its headers, in order
		for _, s := range f.item.sections(SectionHeader) {
			if s.PrintOnEveryPage || !f.headersDone {
				err := p.place(f, s)
				if err != nil {
					return err
				}
				p.pageHeaders[s] = true
			}
		}
		// the open groups' headers on every page (a group that starts the page prints its own)
		for _, g := range f.groups {
			if !g.open {
				continue
			}
			for _, s := range f.item.groupSections(SectionGroupHeader, g.field) {
				if s.PrintOnEveryPage {
					err := p.place(f, s)
					if err != nil {
						return err
					}
					p.pageHeaders[s] = true
				}
			}
		}
	}
	for _, f := range p.frames {
		if f.bodies == 0 {
			continue
		}
		for _, s := range f.item.sections(SectionTransHeader) {
			err := p.place(f, s)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// setPageNo: CurrReport.PAGENO := n — the current page's number, or the next page's when
// one is about to start
func (p *printer) setPageNo(n int) {
	if !p.pageOpen || p.newPage {
		p.nextPageNo = n
		return
	}
	p.pageNo = n
}

// pageNumber: CurrReport.PAGENO — the next page's while one is about to start
func (p *printer) pageNumber() int {
	if !p.pageOpen || p.newPage {
		if p.nextPageNo > 0 {
			return p.nextPageNo
		}
		return p.pageNo + 1
	}
	return p.pageNo
}

// shown: print_if / print_unless of a section or element
func (p *printer) shown(printIf, printUnless string) bool {
	if printIf != "" && !isSet(p.run.values[printIf]) {
		return false
	}
	if printUnless != "" && isSet(p.run.values[printUnless]) {
		return false
	}
	return true
}

// drawFn draws a section at a position down the page
type drawFn func(y float64) error

// prepare lays a section out: its height and how to draw it (the values are read now, the
// page number when it is drawn)
func (p *printer) prepare(f *frame, s *Section) (float64, drawFn) {
	height := s.Height
	totals := f.sumsOf(s)
	var draws []drawFn
	left := p.doc.Margins().Left
	for _, e := range s.Elements {
		if !p.shown(e.PrintIf, e.PrintUnless) {
			continue
		}
		e := e
		size := e.Size
		if size == 0 {
			size = p.layout.FontSize
		}
		style := pdf.Style{Size: size, Bold: e.Bold, Align: e.Align}
		lineHeight := style.LineHeight()
		x := left + e.X
		width := e.Width
		if width <= 0 {
			width = p.doc.ContentWidth() - e.X
		}
		var bottom float64 // where the element ends, from the section's top
		switch e.kind() {
		case "line":
			bottom = e.Y + e.Height
			draws = append(draws, func(y float64) error {
				if e.Height > 0 && e.Width <= 0 {
					p.doc.Line(x, y+e.Y, x, y+e.Y+e.Height, 0)
				} else {
					p.doc.Line(x, y+e.Y, x+width, y+e.Y, 0)
				}
				return nil
			})
		case "box":
			bottom = e.Y + e.Height
			draws = append(draws, func(y float64) error {
				p.doc.Box(x, y+e.Y, e.Width, e.Height, 0)
				return nil
			})
		case "image":
			data, ok := p.run.values[e.Image].([]byte)
			if !ok || len(data) == 0 {
				continue
			}
			bottom = e.Y + e.Height
			draws = append(draws, func(y float64) error {
				err := p.doc.Image(x, y+e.Y, e.Width, e.Height, data)
				if err != nil {
					return fmt.Errorf("report %d: picture %s: %w", p.run.ID, e.Image, err)
				}
				return nil
			})
		case "lines":
			lines := linesOf(p.run.values[e.Lines])
			bottom = e.Y + float64(len(lines))*lineHeight
			draws = append(draws, func(y float64) error {
				for i, line := range lines {
					p.doc.Text(x, y+e.Y+float64(i)*lineHeight, width, line, style)
				}
				return nil
			})
		case "page_no":
			bottom = e.Y + lineHeight
			draws = append(draws, func(y float64) error {
				p.doc.Text(x, y+e.Y, width, fmt.Sprint(p.pageNo), style)
				return nil
			})
		default:
			text := p.text(f, totals, e)
			lines := []string{text}
			if e.Wrap {
				lines = p.doc.SplitText(text, width, style)
			}
			bottom = e.Y + float64(len(lines))*lineHeight
			draws = append(draws, func(y float64) error {
				for i, line := range lines {
					p.doc.Text(x, y+e.Y+float64(i)*lineHeight, width, line, style)
				}
				return nil
			})
		}
		if bottom > height {
			height = bottom
		}
	}
	return height, func(y float64) error {
		for _, draw := range draws {
			err := draw(y)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

// sumsOf: the totals a section prints — of its group in a group section, else the data item's
func (f *frame) sumsOf(s *Section) *sums {
	for _, g := range f.groups {
		if s.Group != "" && g.field == s.Group {
			return g.sums
		}
	}
	return f.sums
}

// text is what a text, caption, field, value or total element prints (totals: of the section)
func (p *printer) text(f *frame, totals *sums, e *Element) string {
	switch e.kind() {
	case "text":
		return p.run.Text(e.Text)
	case "caption":
		rec, field := p.reference(f, e.Caption)
		c, ok := rec.(interface{ GetFieldCaption(string, string) string })
		if ok {
			return c.GetFieldCaption(field, p.language)
		}
		return field
	case "field":
		rec, field := p.reference(f, e.Field)
		value, info := fieldValue(rec, field)
		if e.BlankZero && isZero(value) {
			return ""
		}
		places := e.DecimalPlaces
		if places == "" && info != nil {
			places = info.DecimalPlaces
		}
		return p.fieldText(rec, field, value, places)
	case "value":
		value := p.run.values[e.Value]
		if e.BlankZero && isZero(value) {
			return ""
		}
		return formatValue(value, e.DecimalPlaces, p.language)
	case "total":
		total := totals.fields[e.Total]
		if e.BlankZero && total.IsZero() {
			return ""
		}
		places := e.DecimalPlaces
		if places == "" {
			_, info := fieldValue(f.rec, e.Total)
			if info != nil && info.DecimalPlaces != "" {
				places = info.DecimalPlaces
			} else {
				places = "2:2"
			}
		}
		return formatDecimal(total, places, p.language)
	case "total_value":
		total := totals.values[e.TotalValue]
		if e.BlankZero && total.IsZero() {
			return ""
		}
		places := e.DecimalPlaces
		if places == "" {
			places = "2:2"
		}
		return formatDecimal(total, places, p.language)
	}
	return ""
}

// reference: the record and field an element names — <field> of the section's data item, or
// <DataItem>.<field> of a data item in progress
func (p *printer) reference(f *frame, ref string) (interface{}, string) {
	name, field, ok := strings.Cut(ref, ".")
	if !ok {
		return f.rec, ref
	}
	for i := len(p.frames) - 1; i >= 0; i-- {
		if p.frames[i].item.Name == name {
			return p.frames[i].rec, field
		}
	}
	return nil, field
}

// fieldText prints a field's value in the language: an option's caption, Yes/No, a date, a
// decimal with its decimal places
func (p *printer) fieldText(rec interface{}, field string, value interface{}, places string) string {
	switch v := value.(type) {
	case types.Decimal:
		if places == "" {
			places = "0:5"
		}
		return formatDecimal(v, places, p.language)
	case optionValue:
		options, ok := rec.(interface {
			GetOptionFields() map[string][]string
			GetOptionCaption(string, string, string) string
		})
		if !ok {
			return fmt.Sprint(int(v))
		}
		values := options.GetOptionFields()[field]
		if int(v) < 0 || int(v) >= len(values) {
			return fmt.Sprint(int(v))
		}
		name := values[v]
		if strings.TrimSpace(name) == "" {
			return ""
		}
		caption := options.GetOptionCaption(field, i18n.Key(name), p.language)
		if strings.HasPrefix(caption, "tables.") {
			return name
		}
		return caption
	}
	return formatValue(value, places, p.language)
}

// optionValue is an option field's value (its number) as fieldValue gives it
type optionValue int

// fieldValue: a field of a record as a Go value — decimals as types.Decimal, dates as
// types.Date, options as optionValue, booleans, texts — and its field info (nil for an Integer)
func fieldValue(rec interface{}, field string) (interface{}, *ftables.FieldInfo) {
	values, ok := rec.(interface{ ToMap() map[string]interface{} })
	if !ok {
		return nil, nil
	}
	raw := values.ToMap()[field]
	table, ok := rec.(interface{ GetFields() []ftables.FieldInfo })
	if !ok {
		return raw, nil
	}
	for _, info := range table.GetFields() {
		if info.Name != field {
			continue
		}
		info := info
		switch info.Type {
		case ftables.FieldTypeDecimal:
			d, err := types.NewDecimalFromString(fmt.Sprint(raw))
			if err != nil {
				return types.Decimal{}, &info
			}
			return d, &info
		case ftables.FieldTypeDate:
			d, _ := types.NewDateFromString(fmt.Sprint(raw))
			return d, &info
		case ftables.FieldTypeOption:
			n, _ := raw.(int)
			return optionValue(n), &info
		case ftables.FieldTypeBlob:
			return nil, &info
		}
		return raw, &info
	}
	return raw, nil
}

// linesOf: a lines element's value as its lines — a list or array of texts (an address
// array), or a text with line breaks
func linesOf(v interface{}) []string {
	text, ok := v.(string)
	if ok {
		if text == "" {
			return nil
		}
		return strings.Split(text, "\n")
	}
	list := reflect.ValueOf(v)
	if (list.Kind() != reflect.Slice && list.Kind() != reflect.Array) || list.Type().Elem().Kind() != reflect.String {
		return nil
	}
	lines := make([]string, list.Len())
	for i := range lines {
		lines[i] = list.Index(i).String()
	}
	return lines
}
