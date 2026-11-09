package codeunits

import (
	"errors"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/bankrecon"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// BankAccReconPostPrint - Codeunit 372 "Bank Acc. Recon. Post+Print": Post and Print
// (Shift+F11) on the Bank Acc. Reconciliation card. After asking, it posts the reconciliation
// (codeunit 370) in one transaction and prints the statement it became with the reports of
// Report Selections usage B.Stmt (DocPrint.PrintBankAccStmt), then opens the statement.
const BankAccReconPostPrintID = 372
const BankAccReconPostPrintName = "bank-acc-recon-post-print"

func init() {
	Register(BankAccReconPostPrintID, BankAccReconPostPrintName, NewBankAccReconPostPrint)
}

type BankAccReconPostPrint struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewBankAccReconPostPrint creates a new instance of the codeunit
func NewBankAccReconPostPrint(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &BankAccReconPostPrint{db: db, company: company, dbType: dbType}
}

func (c *BankAccReconPostPrint) ID() int                    { return BankAccReconPostPrintID }
func (c *BankAccReconPostPrint) Name() string               { return BankAccReconPostPrintName }
func (c *BankAccReconPostPrint) SourceTable() string        { return gtables.BankAccReconciliationTableName }
func (c *BankAccReconPostPrint) UsesProgress() bool         { return true } // it asks (Confirm) and shows the posting's progress
func (c *BankAccReconPostPrint) GetDB() database.Executor   { return c.db }
func (c *BankAccReconPostPrint) GetCompany() string         { return c.company }
func (c *BankAccReconPostPrint) GetDBType() database.DBType { return c.dbType }

// Run posts the reconciliation (the card's record) and prints its statement
func (c *BankAccReconPostPrint) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.BankAccReconciliation)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 372 runs on a bank account reconciliation")
	}
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	if !system.Confirm(ts.Message("BANK_REC_POST_PRINT_CONFIRM", lang)) {
		return fcodeunits.Result{Success: true}, nil
	}
	tx, err := ftables.BeginTransaction(c.db)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	src := ftables.DBSource(tx, c.company, c.dbType)
	bankAccRecon := gtables.NewBankAccReconciliation(src)
	err = bankAccRecon.GetOrError(map[string]interface{}{"bank_account_no": rec.Bank_account_no, "statement_no": rec.Statement_no})
	if err != nil {
		_ = ftables.Rollback(tx)
		return fcodeunits.Result{}, err
	}
	posted, err := bankrecon.Post(bankAccRecon)
	if err != nil {
		_ = ftables.Rollback(tx)
		return fcodeunits.Result{}, err
	}
	err = ftables.Commit(tx)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if !posted {
		return system.Message(ts.Message("BANK_REC_NOTHING_TO_POST", lang)), nil
	}

	var printouts []reports.Result
	bankAccStmt := gtables.NewBankAccountStatement(ftables.DBSource(c.db, c.company, c.dbType))
	if bankAccStmt.Get(map[string]interface{}{"bank_account_no": rec.Bank_account_no, "statement_no": rec.Statement_no}) {
		printouts, err = reports.PrintDocuments(bankAccStmt, gtables.ReportSelections_Usage.BStmt, false)
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}
	result := system.Message(ts.Message("BANK_REC_POSTED", lang), rec.Statement_no)
	result.OpenPage = &fcodeunits.PageLink{PageID: bankAccountStatementCardID, Key: rec.Bank_account_no.String() + "," + rec.Statement_no.String()}
	return reports.WithPrintouts(result, printouts), nil
}
