package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/consolidation"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 92 Import Consolidation from File: a business unit's G/L as Export Consolidation wrote
// it (its company names the business unit) is consolidated as Import Consolidation from DB does —
// the earlier consolidation of the file's period taken out, the opening balance sheet adjusted,
// the file's entries posted on their consolidation accounts, exchange rate differences and
// residuals; the dimension 1 and 2 values of the file must be here.
const ImportConsolidationFileID = 92

func init() {
	Register(ImportConsolidationFileID, "import-consolidation-from-file", func() Report { return &importConsolidationFile{} })
}

type importConsolidationFile struct{}

func (r *importConsolidationFile) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "file", Type: "file", Required: true},
			{Name: "document_no", Type: "text", Required: true},
		},
		Transaction: true,
		OnPreReport: r.onPreReport,
	}
}

// readConsolidationFile reads the file option: the header, its lines and the business unit of
// its company (one, to consolidate)
func readConsolidationFile(run *Run) (consolidation.FileHeader, []consolidation.FileLine, *gtables.BusinessUnit, error) {
	data := run.OptionFile("file")
	if data == nil {
		return consolidation.FileHeader{}, nil, nil, system.Error(run.Text("ENTER_FILE"))
	}
	header, lines, err := consolidation.ReadFile(data)
	if err != nil {
		return header, nil, nil, err
	}
	if system.GuiAllowed() && !system.Confirm(run.Text("CONFIRM_IMPORT", header.CompanyName, header.StartDate, header.EndDate)) {
		return header, nil, nil, system.Error(run.Text("INTERRUPTED"))
	}
	if header.PerDay && header.StartDate.DaysBetween(header.EndDate)+1 > consolidation.MaxDays {
		return header, nil, nil, system.Error(run.Text("MAX_DAYS", consolidation.MaxDays))
	}
	busUnit := gtables.NewBusinessUnit(run)
	busUnit.SetCurrentKey("company_name")
	busUnit.SetRange("company_name", types.NewText(header.CompanyName))
	if !busUnit.FindFirst() {
		return header, nil, nil, system.Error(run.Text("NO_BUSINESS_UNIT", header.CompanyName))
	}
	if busUnit.Count() > 1 {
		return header, nil, nil, system.Error(run.Text("NOT_UNIQUE", busUnit.FieldCaption("company_name"), header.CompanyName))
	}
	err = busUnit.TestField("consolidate", true)
	if err != nil {
		return header, nil, nil, err
	}
	return header, lines, busUnit, nil
}

func (r *importConsolidationFile) onPreReport(run *Run) error {
	docNo := types.Code(system.UpperCase(run.Option("document_no")))
	if docNo == "" {
		return system.Error(run.Text("ENTER_DOCUMENT_NO"))
	}
	header, lines, busUnit, err := readConsolidationFile(run)
	if err != nil {
		return err
	}
	window := system.OpenWindow(run.Text("WINDOW"))
	defer window.Close()
	window.Update(1, busUnit.Code)
	window.Update(4, 1)

	// the file's dimension 1 and 2 values are this company's
	thisCompanys := func(rec interface{ GetOrError(interface{}) error }) func(types.Code) (types.Code, error) {
		return func(code types.Code) (types.Code, error) {
			err := rec.GetOrError(code)
			if err != nil {
				return "", err
			}
			return code, nil
		}
	}
	c := &consolidation.Consolidation{Src: run, BusUnit: busUnit, DocNo: docNo, SourceCode: gtables.GetSourceCodeSetup(run).Consolidation,
		StartDate: header.StartDate, EndDate: header.EndDate, PerDay: header.PerDay, PerDept: header.PerDept, PerProj: header.PerProj,
		MapDimension1: thisCompanys(gtables.NewDimension1(run)), MapDimension2: thisCompanys(gtables.NewDimension2(run))}
	var cleared consolidation.Cleared
	err = c.ClearPrevious(&cleared)
	if err != nil {
		return err
	}
	err = cleared.Check()
	if err != nil {
		return err
	}

	// the opening balance sheet at the new balance factor: the accounts at the current rate
	if c.NeedsBalanceAdjustment() {
		consolidGLAcc := gtables.NewGLAccount(run)
		consolidGLAcc.SetCurrentKey("no")
		consolidGLAcc.SetRange("account_type", gtables.GLAccount_Account_type.Posting)
		for found := consolidGLAcc.FindSetBuffered(); found; found = consolidGLAcc.Next() {
			method := consolidation.AccountMethod(consolidGLAcc.Consol_currency_val_method, consolidGLAcc.Income_balance)
			if method != consolidation.CurrentRate {
				continue
			}
			window.Update(2, consolidGLAcc.No)
			err = adjustAccountBalance(run, c, consolidGLAcc, header.PerDept, header.PerProj)
			if err != nil {
				return err
			}
		}
		err = c.FinishBalanceAdjustment()
		if err != nil {
			return err
		}
	}

	// the file's lines: an account, then its entries
	var acc *consolidation.SubsidAccount
	for i, line := range lines {
		window.Update(4, i+2)
		if line.Account != nil {
			acc = line.Account
			window.Update(2, acc.No)
			continue
		}
		if acc == nil || line.Entry.AccountNo != acc.No {
			return system.Error(run.Text("E_LINE_INCONSISTENT", line.Entry.AccountNo, i+2))
		}
		if header.PerDay {
			window.Update(3, line.Entry.PostingDate)
		}
		err = postGroup(c, *acc, &line.Entry.Group)
		if err != nil {
			return err
		}
	}
	err = c.FinishEntries()
	if err != nil {
		return err
	}
	return consolidation.Post(run, c.Lines())
}
