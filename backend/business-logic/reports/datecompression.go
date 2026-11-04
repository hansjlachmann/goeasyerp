package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/datecompr"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// What the date compression reports share: the request page (Starting/Ending Date, Period
// Length, Posting Description, the fields to retain — their contents, or their totals), the
// confirmation, codeunit 356 and the Date Compr. Register row of the run. Each report keeps the
// classic code of its ledger: for every entry of the period that is still there, the entries of
// its group (the same account, posting groups … and the retained fields) are added up into one
// new entry and deleted; the register of the new entries and the Date Compr. Register are
// written every 10 deletions and at the end.

// retainField is a field the user can keep (Retain Field Contents: the entries are grouped by
// it) or keep the total of (Retain Totals: a quantity added up)
type retainField struct {
	name  string
	total bool
}

// dateComprOptions: the request page options of a date compression with its retain fields
// (shown as retain_<field> check boxes) — description: with a Posting Description
func dateComprOptions(description bool, fields []retainField) []Option {
	options := []Option{
		{Name: "starting_date", Type: "date"},
		{Name: "ending_date", Type: "date", Required: true},
		{Name: "period_length", Type: "option", Default: "0", Options: []string{"DAY", "WEEK", "MONTH", "QUARTER", "YEAR", "PERIOD"}},
	}
	if description {
		options = append(options, Option{Name: "description", Type: "text"})
	}
	for _, f := range fields {
		options = append(options, Option{Name: "retain_" + f.name, Type: "bool", Default: "false"})
	}
	return options
}

// dateCompression is the state a date compression report shares with the others
type dateCompression struct {
	tableID           int
	fields            []retainField
	retain            map[string]bool
	entrdDateComprJnl *gtables.DateComprRegister // the request: Starting/Ending Date, Period Length
	description       types.Text
	dateComprReg      *gtables.DateComprRegister
	dateComprMgt      *datecompr.DateComprMgt
	noOfDeleted       int
	filter            string
}

// onInitReport (OnOpenForm): the Ending Date is today, the description "Date Compressed"
func (d *dateCompression) onInitReport(run *Run) error {
	run.SetOption("ending_date", system.Today().String())
	run.SetOption("description", run.Text("DATE_COMPRESSED"))
	return nil
}

// read takes the request page's choices: the period, the description, the fields to retain
func (d *dateCompression) read(run *Run) {
	d.entrdDateComprJnl = gtables.NewDateComprRegister(run)
	d.entrdDateComprJnl.Starting_date = run.OptionDate("starting_date")
	d.entrdDateComprJnl.Ending_date = run.OptionDate("ending_date")
	d.entrdDateComprJnl.Period_length = gtables.DateComprRegisterPeriod_length(run.OptionInt("period_length"))
	d.description = types.Text(run.Option("description"))
	d.retain = map[string]bool{}
	for _, f := range d.fields {
		d.retain[f.name] = run.OptionBool("retain_" + f.name)
	}
	d.dateComprReg = gtables.NewDateComprRegister(run)
	d.dateComprMgt = datecompr.New(run)
}

// confirm (OnPreDataItem): the user confirms that entries are deleted (classic CONFIRM … BREAK)
// and the Ending Date is given
func (d *dateCompression) confirm(run *Run) error {
	if system.GuiAllowed() && !system.Confirm(run.Text("CONFIRM")) {
		return Break()
	}
	if d.entrdDateComprJnl.Ending_date.IsZero() {
		return apperrors.DateComprEndingDate()
	}
	return nil
}

// retainNo (RetainNo): whether the field's contents (or total) are kept
func (d *dateCompression) retainNo(field string) bool {
	return d.retain[field]
}

// dateFilter (DateComprMgt.GetDateFilter): the period of the date as a filter, and its first
// date — the new entry's date (GETRANGEMIN)
func (d *dateCompression) dateFilter(date types.Date, checkFiscYearEnd bool) (string, types.Date, error) {
	filter, err := d.dateComprMgt.GetDateFilter(date, d.entrdDateComprJnl, checkFiscYearEnd)
	if err != nil {
		return "", types.Date{}, err
	}
	start, _, err := d.dateComprMgt.GetDateRange(date, d.entrdDateComprJnl, checkFiscYearEnd)
	return filter, start, err
}

// initRegister (InitRegisters, the Date Compr. Register part): the next register row of the
// run, with the request, the retained fields' captions, the filter, the register of the new
// entries and the source code
func (d *dateCompression) initRegister(registerNo int, sourceCode types.Code, captionOf func(field string) string) {
	dateComprReg := d.dateComprReg
	dateComprReg.Reset()
	no := 0
	if dateComprReg.FindLast() {
		no = dateComprReg.No
	}
	dateComprReg.Init()
	dateComprReg.No = no + 1
	dateComprReg.Table_id = d.tableID
	dateComprReg.Creation_date = system.Today()
	dateComprReg.Starting_date = d.entrdDateComprJnl.Starting_date
	dateComprReg.Ending_date = d.entrdDateComprJnl.Ending_date
	dateComprReg.Period_length = d.entrdDateComprJnl.Period_length
	contents := ""
	totals := ""
	for _, f := range d.fields {
		if d.retain[f.name] {
			if f.total {
				totals = totals + "," + captionOf(f.name)
			} else {
				contents = contents + "," + captionOf(f.name)
			}
		}
	}
	dateComprReg.Retain_field_contents = types.Text(system.CopyStr(system.CopyStr(contents, 2), 1, dateComprReg.MaxStrLen("retain_field_contents")))
	dateComprReg.Retain_totals = types.Text(system.CopyStr(system.CopyStr(totals, 2), 1, dateComprReg.MaxStrLen("retain_totals")))
	dateComprReg.Filter = types.Text(system.CopyStr(d.filter, 1, dateComprReg.MaxStrLen("filter")))
	dateComprReg.Register_no = registerNo
	dateComprReg.Source_code = sourceCode
	dateComprReg.User_id = types.Code(fcodeunits.CurrentUserID())
	d.noOfDeleted = 0
}

// saveRegister (InsertRegisters, the Date Compr. Register part): the register row inserted the
// first time, modified afterwards
func (d *dateCompression) saveRegister(exists bool) error {
	if exists {
		return d.dateComprReg.ModifyOrError(false)
	}
	return d.dateComprReg.InsertOrError(false)
}

// due: 10 more entries deleted since the registers were written last (they are written then)
func (d *dateCompression) due() bool {
	if d.dateComprReg.No_records_deleted >= d.noOfDeleted+10 {
		d.noOfDeleted = d.dateComprReg.No_records_deleted
		return true
	}
	return false
}

// deleted counts an entry deleted
func (d *dateCompression) deleted() {
	d.dateComprReg.No_records_deleted++
}

// newRecord counts a new entry
func (d *dateCompression) newRecord() {
	d.dateComprReg.No_of_new_records++
}

// requestFilter is the data item's filters as text (GETFILTERS, the register's Filter)
func requestFilter(run *Run, dataItem string) string {
	text := ""
	for _, f := range run.filters[dataItem] {
		if text != "" {
			text += ", "
		}
		text += f.Field + ": " + f.Expression
	}
	return text
}

// glNumbered is the G/L register of a compression of entries numbered with the G/L entries
// (tax, customer, vendor, bank account): the new entries take G/L entry numbers, and each time
// the registers are written a G/L entry of 0 closes the register's range (InsertRegisters)
type glNumbered struct {
	glReg             *gtables.GLRegister
	glRegExists       bool
	lastEntryNo       int
	nextTransactionNo int
	sourceCode        types.Code
	glDescription     types.Text
}

// start: the last G/L entry's number and the next transaction (sameTransaction: the last one)
func (g *glNumbered) start(run *Run, sourceCode types.Code, description types.Text, sameTransaction bool) {
	g.sourceCode = sourceCode
	g.glDescription = description
	g.glReg = gtables.NewGLRegister(run)
	g.glReg.LockTable()
	glEntry := gtables.NewGLEntry(run)
	glEntry.LockTable()
	g.lastEntryNo = 0
	g.nextTransactionNo = 0
	if glEntry.FindLast() {
		g.lastEntryNo = glEntry.Entry_no
		g.nextTransactionNo = glEntry.Transaction_no
	}
	if !sameTransaction {
		g.nextTransactionNo++
	}
}

// initGLReg (InitRegisters, the G/L register part): the next G/L register, from the next entry
func (g *glNumbered) initGLReg() {
	glReg := g.glReg
	glReg.Reset()
	no := 0
	if glReg.FindLast() {
		no = glReg.No
	}
	glReg.Init()
	glReg.No = no + 1
	glReg.Creation_date = system.Today()
	glReg.Source_code = g.sourceCode
	glReg.User_id = types.Code(fcodeunits.CurrentUserID())
	glReg.From_entry_no = g.lastEntryNo + 1
	g.glRegExists = false
}

// insertGLEntry (InsertRegisters, the G/L entry of 0): the register's last entry
func (g *glNumbered) insertGLEntry(run *Run) error {
	glEntry := gtables.NewGLEntry(run)
	glEntry.Init()
	g.lastEntryNo++
	glEntry.Entry_no = g.lastEntryNo
	glEntry.Posting_date = system.Today()
	glEntry.Description = g.glDescription
	glEntry.Source_code = g.sourceCode
	glEntry.System_created_entry = true
	glEntry.User_id = types.Code(fcodeunits.CurrentUserID())
	glEntry.Transaction_no = g.nextTransactionNo
	err := glEntry.InsertOrError(false)
	if err != nil {
		return err
	}
	glEntry.Consistent(glEntry.Amount.IsZero())
	g.glReg.To_entry_no = glEntry.Entry_no
	return nil
}

// saveGLReg: the G/L register inserted the first time, modified afterwards
func (g *glNumbered) saveGLReg() error {
	if g.glRegExists {
		return g.glReg.ModifyOrError(false)
	}
	return g.glReg.InsertOrError(false)
}

// entryRegister is a register of entries numbered on their own (Item, BOM, Resource, FA,
// Insurance Register): fields 1 No., 2 From Entry No., 3 To Entry No., 4 Creation Date,
// 5 Source Code, 6 User ID
type entryRegister interface {
	Reset()
	Init()
	FindLast() bool
	LockTable()
	FieldValue(no int) (any, bool)
	SetFieldValue(no int, value any) bool
	InsertOrError(runTrigger bool) error
	ModifyOrError(runTrigger bool) error
}

// ownRegister is the register of a compression whose entries are numbered on their own
type ownRegister struct {
	reg       entryRegister
	regExists bool
}

// init (InitRegisters, the register part): the next register, from the entry after lastEntryNo;
// its number
func (o *ownRegister) init(sourceCode types.Code, lastEntryNo int) int {
	reg := o.reg
	reg.Reset()
	no := 0
	if reg.FindLast() {
		value, _ := reg.FieldValue(1)
		no, _ = value.(int)
	}
	reg.Init()
	reg.SetFieldValue(1, no+1)
	reg.SetFieldValue(4, system.Today())
	reg.SetFieldValue(5, sourceCode)
	reg.SetFieldValue(6, types.Code(fcodeunits.CurrentUserID()))
	reg.SetFieldValue(2, lastEntryNo+1)
	o.regExists = false
	return no + 1
}

// save (InsertRegisters, the register part): To Entry No. the last new entry; inserted the first
// time, modified afterwards
func (o *ownRegister) save(toEntryNo int) error {
	o.reg.SetFieldValue(3, toEntryNo)
	if o.regExists {
		return o.reg.ModifyOrError(false)
	}
	return o.reg.InsertOrError(false)
}
