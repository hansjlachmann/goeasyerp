package setupchecklist

import (
	"reflect"
	"sync"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func init() {
	functions.Register("table", gtables.SetupChecklistLineTableName, "CopyData", functions.For(CopyData), functions.WithDialog())
}

var (
	recordTypes     map[int]reflect.Type
	recordTypesOnce sync.Once
)

// newTable is a new record of a table number in a company (nil when there is no such table)
func newTable(src ftables.Source, company string, tableID int) ftables.Table {
	recordTypesOnce.Do(func() {
		recordTypes = map[int]reflect.Type{}
		for id, rec := range gtables.NewRecords() {
			recordTypes[id] = reflect.TypeOf(rec).Elem()
		}
	})
	t, ok := recordTypes[tableID]
	if !ok {
		return nil
	}
	rec := reflect.New(t).Interface().(ftables.Table)
	rec.InitWithDBType(src.GetDB(), company, src.GetDBType())
	return rec
}

// NoOfRecords (No. of Records, classic Table Information): the records of the table in a
// company
func NoOfRecords(src ftables.Source, company string, tableID int) int {
	rec := newTable(src, company, tableID)
	if rec == nil {
		return 0
	}
	return rec.Count()
}

// CopyData (Copy Data, form 533): the selected lines' tables get the data of another company —
// the user picks the company; then CopyDataDialog
func CopyData(rec *gtables.SetupChecklistLine) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	setupChecklistLine := rec
	setupChecklistLine.SetSelectionFilter()
	company := gtables.NewCompany(ftables.DBSource(rec.GetDB(), "", rec.GetDBType()))
	lookup := &fcodeunits.InputLookup{
		Columns:  []fcodeunits.InputLookupColumn{{Source: "name", Width: 200}, {Source: "display_name", Width: 250}},
		Rows:     []map[string]interface{}{},
		Captions: map[string]string{"name": company.GetFieldCaption("name", lang), "display_name": company.GetFieldCaption("display_name", lang)},
	}
	for found := company.FindSet(); found; found = company.Next() {
		if company.Name.String() == rec.GetCompany() {
			continue
		}
		lookup.Rows = append(lookup.Rows, map[string]interface{}{"_key": company.Name.String(), "name": company.Name.String(), "display_name": company.Display_name.String()})
	}
	answer := fcodeunits.RequestInput(ts.Message("SETUP_CHECKLIST_COPY_TITLE", lang), []fcodeunits.InputField{
		{Name: "copy_from", Label: ts.Message("SETUP_CHECKLIST_COPY_FROM", lang), Type: "text", Required: true, Lookup: lookup},
	})
	if answer == nil {
		return fcodeunits.Result{Success: true}, nil
	}
	newCompanyName := answer["copy_from"]
	if newCompanyName != "" {
		exists := gtables.NewCompany(ftables.DBSource(rec.GetDB(), "", rec.GetDBType()))
		if newCompanyName == rec.GetCompany() || !exists.GetByPK(types.NewText(newCompanyName)) {
			return fcodeunits.Result{}, system.Error(ts.Message("SETUP_CHECKLIST_NO_COMPANY", lang), newCompanyName)
		}
	}

	tx, err := ftables.BeginTransaction(rec.GetDB())
	if err != nil {
		return fcodeunits.Result{}, err
	}
	setupChecklistLine.SetDB(tx)
	message, err := CopyDataDialog(newCompanyName, setupChecklistLine)
	if err != nil {
		_ = ftables.Rollback(tx)
		return fcodeunits.Result{}, err
	}
	err = ftables.Commit(tx)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if message == "" {
		return fcodeunits.Result{Success: true}, nil
	}
	result := system.Message(message)
	result.Refresh = true
	return result, nil
}

// CopyDataDialog (CopyDataDialog): after a question, the data of the lines' tables is copied
// from the company; the message to show when done ("" when the user said no)
func CopyDataDialog(newCompanyName string, setupChecklistLine *gtables.SetupChecklistLine) (string, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	if newCompanyName == "" {
		return "", system.Error(ts.Message("SETUP_CHECKLIST_COMPANY_REQUIRED", lang))
	}
	if !setupChecklistLine.FindFirst() {
		return "", nil
	}
	singleTable := setupChecklistLine.Count() == 1
	var tableText string
	if singleTable {
		tableText = system.StrSubstNo(ts.Message("SETUP_CHECKLIST_TABLE", lang), setupChecklistLine.Table_name)
	} else {
		tableText = ts.Message("SETUP_CHECKLIST_SELECTED_TABLES", lang)
	}
	tableText = system.StrSubstNo(ts.Message("SETUP_CHECKLIST_IN", lang), tableText, newCompanyName)
	if !system.Confirm(ts.Message("SETUP_CHECKLIST_COPY_CONFIRM", lang), tableText) {
		return "", nil
	}
	for found := setupChecklistLine.FindSetBuffered(); found; found = setupChecklistLine.Next() {
		err := CopyTableData(newCompanyName, setupChecklistLine)
		if err != nil {
			return "", err
		}
	}
	return system.StrSubstNo(ts.Message("SETUP_CHECKLIST_COPIED", lang), tableText), nil
}

// CopyTableData (CopyData): the line's table, without data, gets the data of the base company
func CopyTableData(baseCompanyName string, setupChecklistLine *gtables.SetupChecklistLine) error {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	company := setupChecklistLine.GetCompany()
	if baseCompanyName == company {
		return system.Error(ts.Message("SETUP_CHECKLIST_BASE_SAME", lang))
	}
	if !setupChecklistLine.Copying_available {
		return system.Error(ts.Message("SETUP_CHECKLIST_NO_COPYING", lang), setupChecklistLine.Table_name)
	}
	if NoOfRecords(setupChecklistLine, company, setupChecklistLine.Table_id) != 0 {
		return system.Error(ts.Message("SETUP_CHECKLIST_HAS_DATA", lang), setupChecklistLine.Table_name, company)
	}
	if NoOfRecords(setupChecklistLine, baseCompanyName, setupChecklistLine.Table_id) == 0 {
		return system.Error(ts.Message("SETUP_CHECKLIST_NO_DATA", lang), setupChecklistLine.Table_name, baseCompanyName, company)
	}
	return transferContents(setupChecklistLine, setupChecklistLine.Table_id, baseCompanyName)
}

// transferContents (TransferContents): the records of the table — and of the tables copied
// with it — of the other company are inserted here (with their insert triggers). The user
// needs read permission on them there and insert permission here.
func transferContents(src ftables.Source, tableID int, newCompanyName string) error {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	for _, id := range copyTables[tableID] {
		from := newTable(src, newCompanyName, id)
		to := newTable(src, src.GetCompany(), id)
		if from == nil || to == nil {
			continue
		}
		if !fcodeunits.CanReadInCompany(src.GetDB(), src.GetDBType(), newCompanyName, from.GetTableName()) {
			return system.Error(ts.Message("SETUP_CHECKLIST_NO_READ", lang), from.GetTableName(), newCompanyName)
		}
		if !fcodeunits.HasPermission(to.GetTableName(), session.PermInsert) {
			return system.Error(ts.Message("SETUP_CHECKLIST_NO_INSERT", lang), to.GetTableName())
		}
		transfer, ok := to.(interface {
			TransferFields(ftables.FieldValuer, ...bool)
		})
		values, isValuer := from.(ftables.FieldValuer)
		if !ok || !isValuer {
			continue
		}
		for found := from.FindSet(); found; found = from.Next() {
			to.InitRecord()
			transfer.TransferFields(values)
			err := to.InsertOrError(true)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
