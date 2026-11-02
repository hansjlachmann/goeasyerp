package codeunits

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/copytaxsetup"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/session"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// CopyTaxSetup - Codeunit 399 "Copy Tax Setup" with its dialog (form 476): the user picks a
// company other than the current one and copies all of its tax setup — or the selected tables:
// tax groups, jurisdictions, areas (with their lines), details (copytaxsetup). Records that exist
// already are kept. The user needs read permission on the tables in the other company and insert
// permission in this one. Run from the menu (General Ledger → Setup → Sales Tax).
const CopyTaxSetupID = 399
const CopyTaxSetupName = "copy-tax-setup"

func init() {
	Register(CopyTaxSetupID, CopyTaxSetupName, NewCopyTaxSetup)
}

type CopyTaxSetup struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewCopyTaxSetup creates a new instance of the codeunit
func NewCopyTaxSetup(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &CopyTaxSetup{db: db, company: company, dbType: dbType}
}

func (c *CopyTaxSetup) ID() int                    { return CopyTaxSetupID }
func (c *CopyTaxSetup) Name() string               { return CopyTaxSetupName }
func (c *CopyTaxSetup) SourceTable() string        { return gtables.TaxGroupTableName }
func (c *CopyTaxSetup) UsesProgress() bool         { return true } // it asks (the dialog of form 476)
func (c *CopyTaxSetup) GetDB() database.Executor   { return c.db }
func (c *CopyTaxSetup) GetCompany() string         { return c.company }
func (c *CopyTaxSetup) GetDBType() database.DBType { return c.dbType }

// copyTaxSetupFields: the check boxes of the tables (CopyTable[1..4])
var copyTaxSetupFields = [4]string{"tax_groups", "tax_jurisdictions", "tax_areas", "tax_detail"}

// Run shows the dialog and copies
func (c *CopyTaxSetup) Run(record interface{}) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()

	// OnOpenForm: all tables
	fields := []fcodeunits.InputField{
		{Name: "from_company", Label: ts.Message("COPY_TAX_FROM_COMPANY", lang), Type: "text", Required: true, Lookup: c.companyLookup(lang)},
		{Name: "copy_mode", Label: ts.Message("COPY_TAX_COPY", lang), Type: "option", Default: "1",
			Options: []string{ts.Message("COPY_TAX_ALL", lang), ts.Message("COPY_TAX_SELECTED", lang)}},
	}
	captions := [4]string{"COPY_TAX_TAX_GROUPS", "COPY_TAX_TAX_JURISDICTIONS", "COPY_TAX_TAX_AREAS", "COPY_TAX_TAX_DETAIL"}
	for i, name := range copyTaxSetupFields {
		fields = append(fields, fcodeunits.InputField{Name: name, Label: ts.Message(captions[i], lang), Type: "bool", Default: "true"})
	}
	answer := fcodeunits.RequestInput(ts.Message("COPY_TAX_TITLE", lang), fields)
	if answer == nil {
		return fcodeunits.Result{Success: true}, nil
	}

	// OK
	sourceCompany := answer["from_company"]
	if sourceCompany == "" {
		return fcodeunits.Result{}, system.Error(ts.Message("COPY_TAX_SELECT_COMPANY", lang))
	}
	if sourceCompany == c.company {
		return fcodeunits.Result{}, system.Error(ts.Message("COPY_TAX_OTHER_COMPANY", lang))
	}
	var copyTable [4]bool
	for i, name := range copyTaxSetupFields {
		copyTable[i] = answer["copy_mode"] == "1" || answer[name] == "true"
	}
	if !copyTable[0] && !copyTable[1] && !copyTable[2] && !copyTable[3] {
		return fcodeunits.Result{}, system.Error(ts.Message("COPY_TAX_NOTHING_SELECTED", lang))
	}
	sourceTaxGroup := gtables.NewTaxGroup(c)
	if !sourceTaxGroup.ChangeCompany(sourceCompany) {
		return fcodeunits.Result{}, system.Error(ts.Message("COPY_TAX_NO_COMPANY", lang), sourceCompany)
	}
	err := c.checkAccess(sourceCompany, copyTable)
	if err != nil {
		return fcodeunits.Result{}, err
	}

	tx, err := ftables.BeginTransaction(c.db)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	window := system.OpenWindow(ts.Message("COPY_TAX_WINDOW", lang))
	copytaxsetup.CopyTaxInfo(ftables.DBSource(tx, c.company, c.dbType), sourceCompany, copyTable, func(table, progress int) {
		window.Update(table+1, progress)
	})
	window.Close()
	err = ftables.Commit(tx)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// companyLookup: the companies the user may copy from (not the current one)
func (c *CopyTaxSetup) companyLookup(lang string) *fcodeunits.InputLookup {
	company := gtables.NewCompany(c)
	lookup := &fcodeunits.InputLookup{
		Columns:  []fcodeunits.InputLookupColumn{{Source: "name", Width: 200}, {Source: "display_name", Width: 250}},
		Rows:     []map[string]interface{}{},
		Captions: map[string]string{"name": company.GetFieldCaption("name", lang), "display_name": company.GetFieldCaption("display_name", lang)},
	}
	for found := company.FindSet(); found; found = company.Next() {
		if company.Name.String() == c.company || !c.mayRead(company.Name.String()) {
			continue
		}
		lookup.Rows = append(lookup.Rows, map[string]interface{}{"_key": company.Name.String(), "name": company.Name.String(), "display_name": company.Display_name.String()})
	}
	return lookup
}

// mayRead: the user may read the other company's tax groups (has roles there, or works without
// permissions)
func (c *CopyTaxSetup) mayRead(sourceCompany string) bool {
	return fcodeunits.CanReadInCompany(c.db, c.dbType, sourceCompany, gtables.TaxGroupTableName) ||
		fcodeunits.CanReadInCompany(c.db, c.dbType, sourceCompany, gtables.TaxJurisdictionTableName)
}

// checkAccess: read permission on the tables in the other company, insert permission here
func (c *CopyTaxSetup) checkAccess(sourceCompany string, copyTable [4]bool) error {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	for i, tableName := range copytaxsetup.Tables {
		if !copyTable[i] {
			continue
		}
		tables := []string{tableName}
		if i == copytaxsetup.TaxAreas {
			tables = append(tables, gtables.TaxAreaLineTableName)
		}
		for _, name := range tables {
			if !fcodeunits.CanReadInCompany(c.db, c.dbType, sourceCompany, name) {
				return system.Error(ts.Message("COPY_TAX_NO_READ", lang), name, sourceCompany)
			}
			if !fcodeunits.HasPermission(name, session.PermInsert) {
				return system.Error(ts.Message("COPY_TAX_NO_INSERT", lang), name)
			}
		}
	}
	return nil
}
