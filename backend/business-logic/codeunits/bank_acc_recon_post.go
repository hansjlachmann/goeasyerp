package codeunits

import (
	"errors"

	"github.com/hansjlachmann/openerp/backend/business-logic/bankrecon"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// BankAccReconPost - Codeunit 371 "Bank Acc. Recon. Post (Yes/No)": Post (F11) on the Bank
// Acc. Reconciliation card. After asking, it posts the reconciliation (codeunit 370) in one
// transaction.
const BankAccReconPostID = 371
const BankAccReconPostName = "bank-acc-recon-post"

func init() {
	Register(BankAccReconPostID, BankAccReconPostName, NewBankAccReconPost)
}

type BankAccReconPost struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewBankAccReconPost creates a new instance of the codeunit
func NewBankAccReconPost(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &BankAccReconPost{db: db, company: company, dbType: dbType}
}

func (c *BankAccReconPost) ID() int                    { return BankAccReconPostID }
func (c *BankAccReconPost) Name() string               { return BankAccReconPostName }
func (c *BankAccReconPost) SourceTable() string        { return gtables.BankAccReconciliationTableName }
func (c *BankAccReconPost) UsesProgress() bool         { return true } // it asks (Confirm) and shows the posting's progress
func (c *BankAccReconPost) GetDB() database.Executor   { return c.db }
func (c *BankAccReconPost) GetCompany() string         { return c.company }
func (c *BankAccReconPost) GetDBType() database.DBType { return c.dbType }

// Run posts the reconciliation (the card's record)
func (c *BankAccReconPost) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.BankAccReconciliation)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 371 runs on a bank account reconciliation")
	}
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	if !system.Confirm(ts.Message("BANK_REC_POST_CONFIRM", lang)) {
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
	result := system.Message(ts.Message("BANK_REC_POSTED", lang), rec.Statement_no)
	result.Refresh = true
	return result, nil
}
