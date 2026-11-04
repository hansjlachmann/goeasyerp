package demodata

import (
	"fmt"
	"math"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/fadepr"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The demo fixed assets' ledgers, posted through codeunit 12 as Fixed Asset G/L Journal lines
// (source code FAGLJNL) — first a bank loan that finances them (paid into the bank on the first
// acquisition's day: the acquisitions' cost rounded up to 100,000, G/L 2210 Bank Loans), then
// per asset in date order:
//   - the acquisition invoice from the asset's vendor — the fixed asset line and the vendor line
//     (insured assets with the insurance: the coverage is posted with it, FA Setup Automatic
//     Insurance Posting) — and its payment from the bank two weeks later;
//   - each month's depreciation at the month's end up to last month, as codeunit 5610 calculates
//     it, against the posting group's depreciation expense account;
//   - the yearly service of the assets with a maintenance vendor (maintenance invoice);
//   - the sale of one asset last month (data/fixedassets.yaml disposal): its depreciation up to
//     that day (Depr. until FA Posting Date), the proceeds into the bank, gain or loss.

func postDemoFixedAssets(src ftables.Source, open func(src ftables.Source) GenJnlPoster, data fixedAssetsFile, today time.Time, counts *Counts, p *progress) error {
	genJnlPostLine := open(src)
	monthStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
	disposalDate := time.Date(monthStart.Year(), monthStart.Month()-1, data.Disposal.Day, 0, 0, 0, 0, today.Location())
	p.setTotal(len(data.FixedAssets))
	err := postFALoan(src, genJnlPostLine, data, monthStart)
	if err != nil {
		return err
	}
	for _, a := range data.FixedAssets {
		faNo := types.NewCode(a.No)
		acquired := monthStart.AddDate(0, -a.MonthsAgo, 0)
		disposed := a.No == data.Disposal.FANo
		post := func(l *gtables.GenJournalLine) error {
			err := genJnlPostLine.Run(l)
			if err != nil {
				return fmt.Errorf("fixed asset %s, %s %s: %w", a.No, l.Document_no, l.Posting_date, err)
			}
			return nil
		}

		// The acquisition and its payment
		documentNo := types.NewCode("FA-" + a.No)
		acq, err := faLine(src, faNo, gtables.GenJournalLine_Fa_posting_type.AcquisitionCost, acquired, documentNo, a.Cost)
		if err != nil {
			return err
		}
		if a.Insured {
			acq.Insurance_no = types.NewCode(data.Insurance.No)
		}
		err = postFAInvoice(src, post, acq, types.NewCode(a.Vendor), types.NewCode("V-"+a.No))
		if err != nil {
			return err
		}
		err = postFAPayment(src, post, types.NewCode(a.Vendor), acquired.AddDate(0, 0, 14), documentNo, a.Cost)
		if err != nil {
			return err
		}

		// The yearly service
		if a.MaintenanceVendor != "" {
			for s := acquired.AddDate(1, 0, 14); s.Before(today) && !(disposed && s.After(disposalDate)); s = s.AddDate(1, 0, 0) {
				maint, err := faLine(src, faNo, gtables.GenJournalLine_Fa_posting_type.Maintenance, s,
					types.NewCode(fmt.Sprintf("MS-%s-%d", a.No, s.Year())), math.Round(a.Cost*0.012*100)/100)
				if err != nil {
					return err
				}
				maint.Maintenance_code = "SERVICE"
				maint.Description = types.NewText("Periodic service " + s.Format("2006"))
				err = postFAInvoice(src, post, maint, types.NewCode(a.MaintenanceVendor), types.NewCode(fmt.Sprintf("V-MS-%s-%d", a.No, s.Year())))
				if err != nil {
					return err
				}
			}
		}

		// The depreciation at each month's end up to last month (the disposed asset: up to the
		// month before its sale)
		depreciationEnd := monthStart
		if disposed {
			depreciationEnd = time.Date(disposalDate.Year(), disposalDate.Month(), 1, 0, 0, 0, 0, today.Location())
		}
		for m := acquired.AddDate(0, 1, -1); m.Before(depreciationEnd); m = time.Date(m.Year(), m.Month()+2, 0, 0, 0, 0, 0, m.Location()) {
			result, err := fadepr.Calculate(src, faNo, demoDeprBook, types.NewDateFromTime(m), fadepr.EntryAmounts{}, types.Date{}, 0)
			if err != nil {
				return err
			}
			if result.DeprAmount.IsZero() {
				continue
			}
			depr, err := faLine(src, faNo, gtables.GenJournalLine_Fa_posting_type.Depreciation, m, types.NewCode(m.Format("DEPR2006-01")), 0)
			if err != nil {
				return err
			}
			err = depr.Validate("amount", result.DeprAmount)
			if err != nil {
				return err
			}
			depr.No_of_depreciation_days = result.NumberOfDays
			depr.Description = types.NewText("Depreciation " + m.Format("01/2006"))
			err = setBalAccount(depr, gtables.GenJournalLine_Bal_account_type.GLAccount, depreciationExpenseAccount(src, types.NewCode(a.PostingGroup)))
			if err != nil {
				return err
			}
			err = post(depr)
			if err != nil {
				return err
			}
		}

		// The sale
		if disposed {
			sale, err := faLine(src, faNo, gtables.GenJournalLine_Fa_posting_type.Disposal, disposalDate, types.NewCode("FA-SALE-"+a.No), -data.Disposal.Proceeds)
			if err != nil {
				return err
			}
			sale.Depr_until_fa_posting_date = true
			sale.Description = types.NewText("Sale " + a.Description)
			err = setBalAccount(sale, gtables.GenJournalLine_Bal_account_type.BankAccount, demoBankAccount)
			if err != nil {
				return err
			}
			err = post(sale)
			if err != nil {
				return err
			}
		}
		p.step()
	}
	counts.FALedgerEntries = gtables.NewFALedgerEntry(src).Count()
	counts.MaintenanceLedgerEntries = gtables.NewMaintenanceLedgerEntry(src).Count()
	return nil
}

// faLine: a Fixed Asset G/L Journal line of an asset in the COMPANY book (the asset's FA posting
// group from its book)
func faLine(src ftables.Source, faNo types.Code, postingType gtables.GenJournalLineFa_posting_type, date time.Time, documentNo types.Code, amount float64) (*gtables.GenJournalLine, error) {
	l := gtables.NewGenJournalLine(src)
	l.Posting_date = types.NewDateFromTime(date)
	l.Document_date = l.Posting_date
	l.Document_no = documentNo
	l.Source_code = "FAGLJNL"
	err := l.Validate("account_type", gtables.GenJournalLine_Account_type.FixedAsset)
	if err == nil {
		err = l.Validate("account_no", faNo)
	}
	if err == nil {
		err = l.Validate("depreciation_book_code", demoDeprBook)
	}
	if err == nil {
		err = l.Validate("fa_posting_type", postingType)
	}
	if err == nil && amount != 0 {
		err = l.Validate("amount", types.NewDecimal(amount))
	}
	if err != nil {
		return nil, &InsertError{Table: gtables.GenJournalLineTableName, Key: faNo.String(), Err: err}
	}
	return l, nil
}

func setBalAccount(l *gtables.GenJournalLine, balAccountType gtables.GenJournalLineBal_account_type, balAccountNo types.Code) error {
	err := l.Validate("bal_account_type", balAccountType)
	if err == nil {
		err = l.Validate("bal_account_no", balAccountNo)
	}
	if err != nil {
		return &InsertError{Table: gtables.GenJournalLineTableName, Key: l.Account_no.String(), Err: err}
	}
	return nil
}

// postFAInvoice: a vendor's invoice for an asset — the fixed asset line and the vendor line of
// one transaction (a fixed asset line balances only against a G/L or bank account)
func postFAInvoice(src ftables.Source, post func(*gtables.GenJournalLine) error, faLine *gtables.GenJournalLine, vendorNo, externalDocumentNo types.Code) error {
	faLine.Document_type = gtables.GenJournalLine_Document_type.Invoice
	faLine.External_document_no = externalDocumentNo
	err := post(faLine)
	if err != nil {
		return err
	}
	l := gtables.NewGenJournalLine(src)
	l.Posting_date = faLine.Posting_date
	l.Document_date = faLine.Document_date
	l.Document_type = gtables.GenJournalLine_Document_type.Invoice
	l.Document_no = faLine.Document_no
	l.External_document_no = externalDocumentNo
	l.Source_code = faLine.Source_code
	err = l.Validate("account_type", gtables.GenJournalLine_Account_type.Vendor)
	if err == nil {
		err = l.Validate("account_no", vendorNo)
	}
	if err == nil {
		err = l.Validate("amount", faLine.Amount.Neg())
	}
	if err != nil {
		return &InsertError{Table: gtables.GenJournalLineTableName, Key: vendorNo.String(), Err: err}
	}
	l.Description = faLine.Description
	return post(l)
}

// postFAPayment: the payment of an asset's invoice from the bank, applied to it
func postFAPayment(src ftables.Source, post func(*gtables.GenJournalLine) error, vendorNo types.Code, date time.Time, invoiceNo types.Code, amount float64) error {
	l := gtables.NewGenJournalLine(src)
	l.Posting_date = types.NewDateFromTime(date)
	l.Document_date = l.Posting_date
	l.Document_type = gtables.GenJournalLine_Document_type.Payment
	l.Document_no = types.NewCode("PAY-" + invoiceNo.String())
	l.Source_code = "FAGLJNL"
	err := l.Validate("account_type", gtables.GenJournalLine_Account_type.Vendor)
	if err == nil {
		err = l.Validate("account_no", vendorNo)
	}
	if err == nil {
		err = l.Validate("amount", types.NewDecimal(amount))
	}
	if err != nil {
		return &InsertError{Table: gtables.GenJournalLineTableName, Key: vendorNo.String(), Err: err}
	}
	l.Applies_to_doc_type = gtables.GenJournalLine_Applies_to_doc_type.Invoice
	l.Applies_to_doc_no = invoiceNo
	err = setBalAccount(l, gtables.GenJournalLine_Bal_account_type.BankAccount, demoBankAccount)
	if err != nil {
		return err
	}
	return post(l)
}

// depreciationExpenseAccount: the FA posting group's depreciation expense account (the balancing
// account Insert Bal. Account would take)
func depreciationExpenseAccount(src ftables.Source, postingGroup types.Code) types.Code {
	faPostingGroup := gtables.NewFAPostingGroup(src)
	if faPostingGroup.Get(postingGroup) {
		return faPostingGroup.Depreciation_expense_acc
	}
	return ""
}

// postFALoan: the bank loan for the fixed assets — the acquisitions' cost rounded up to 100,000,
// paid into the bank on the first acquisition's day against G/L 2210 Bank Loans
func postFALoan(src ftables.Source, genJnlPostLine GenJnlPoster, data fixedAssetsFile, monthStart time.Time) error {
	if len(data.FixedAssets) == 0 {
		return nil
	}
	var cost float64
	first := monthStart
	for _, a := range data.FixedAssets {
		cost += a.Cost
		if acquired := monthStart.AddDate(0, -a.MonthsAgo, 0); acquired.Before(first) {
			first = acquired
		}
	}
	l := gtables.NewGenJournalLine(src)
	l.Posting_date = types.NewDateFromTime(first)
	l.Document_date = l.Posting_date
	l.Document_no = "LOAN-FA"
	l.Source_code = "GENJNL"
	err := l.Validate("account_type", gtables.GenJournalLine_Account_type.BankAccount)
	if err == nil {
		err = l.Validate("account_no", demoBankAccount)
	}
	if err == nil {
		err = l.Validate("amount", types.NewDecimal(math.Ceil(cost/100000)*100000))
	}
	if err == nil {
		err = setBalAccount(l, gtables.GenJournalLine_Bal_account_type.GLAccount, types.Code(accBankLoans))
	}
	if err != nil {
		return &InsertError{Table: gtables.GenJournalLineTableName, Key: "LOAN-FA", Err: err}
	}
	l.Description = "Bank loan for the fixed assets"
	err = genJnlPostLine.Run(l)
	if err != nil {
		return fmt.Errorf("fixed asset loan: %w", err)
	}
	return nil
}
