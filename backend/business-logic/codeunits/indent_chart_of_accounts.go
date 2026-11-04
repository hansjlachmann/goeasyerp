package codeunits

import (
	"errors"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
)

// IndentChartOfAccounts - Codeunit 3 "G/L Account-Indent": indents the chart of accounts
// (every account between a Begin-Total and its End-Total one level) and sets the End-Totals'
// Totaling, after asking. Run from the Chart of Accounts (Functions).
const IndentChartOfAccountsID = 3
const IndentChartOfAccountsName = "indent-chart-of-accounts"

func init() {
	Register(IndentChartOfAccountsID, IndentChartOfAccountsName, NewIndentChartOfAccounts)
}

type IndentChartOfAccounts struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewIndentChartOfAccounts creates a new instance of the codeunit
func NewIndentChartOfAccounts(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &IndentChartOfAccounts{db: db, company: company, dbType: dbType}
}

func (c *IndentChartOfAccounts) ID() int             { return IndentChartOfAccountsID }
func (c *IndentChartOfAccounts) Name() string        { return IndentChartOfAccountsName }
func (c *IndentChartOfAccounts) SourceTable() string { return "G_L_Account" }
func (c *IndentChartOfAccounts) UsesProgress() bool  { return true }

// Run asks, then indents with the progress window showing the account
func (c *IndentChartOfAccounts) Run(record interface{}) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	if !system.Confirm(ts.Message("INDENT_COA_CONFIRM", lang)) {
		return fcodeunits.Result{}, nil
	}
	window := system.OpenWindow(ts.Message("INDENT_COA_WINDOW", lang))
	defer window.Close()
	err := tables.IndentChartOfAccounts(c.db, c.company, c.dbType, func(no string) {
		window.Update(1, no)
	})
	if err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			return fcodeunits.Result{}, errors.New(appErr.Message(lang))
		}
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Refresh: true}, nil
}
