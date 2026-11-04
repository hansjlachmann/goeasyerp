package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/noseries"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// customerLogic is the business logic of table 18 Customer.
//
// Not migrated yet (their tables come later): OnDelete refuses while sales orders are
// outstanding; the contact update (codeunit 5002) of OnInsert/OnModify/OnDelete.
type customerLogic struct{}

func init() {
	gtables.RegisterCustomerLogic(customerLogic{})
	// the No. field's "…" (Customer Card): pick another number series
	functions.Register("table", gtables.CustomerTableName, "AssistEdit", functions.ForAssistEdit(AssistEditCustomer), functions.WithAssistEdit())
}

// OnInsert: a customer without No. gets the next number of the Customer Nos. series of the
// Sales & Receivables Setup
func (customerLogic) OnInsert(rec *gtables.Customer) error {
	if rec.No == "" {
		salesSetup := gtables.GetSalesReceivablesSetup(rec)
		err := salesSetup.TestField("customer_nos")
		if err != nil {
			return err
		}
		noSeriesMgt := noseries.New(rec)
		xRec := rec.XRec()
		err = noSeriesMgt.InitSeries(salesSetup.Customer_nos, xRec.No_series, types.Date{}, &rec.No, &rec.No_series)
		if err != nil {
			return err
		}
		rec.Invoice_disc_code = rec.No
	}
	return nil
}

// OnModify sets Last Date Modified
func (customerLogic) OnModify(rec *gtables.Customer) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnRename sets Last Date Modified
func (customerLogic) OnRename(rec *gtables.Customer) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnDelete moves the customer's ledger entries away (MoveCustEntries): refused while it has
// entries in a fiscal year that is not closed — every entry, as long as there are no
// accounting periods — or open entries; otherwise the entries lose their Customer No. The
// customer's comment lines, bank accounts and ship-to addresses are deleted.
func (customerLogic) OnDelete(rec *gtables.Customer) error {
	err := moveCustEntries(rec)
	if err != nil {
		return err
	}

	DeleteComments(rec, gtables.CommentLine_Table_name.Customer, rec.No)

	custBankAcc := gtables.NewCustomerBankAccount(rec)
	custBankAcc.SetRange("customer_no", rec.No)
	custBankAcc.DeleteAll()

	shipToAddr := gtables.NewShipToAddress(rec)
	shipToAddr.SetRange("customer_no", rec.No)
	shipToAddr.DeleteAll()
	return nil
}

func moveCustEntries(rec *gtables.Customer) error {
	custLedgEntry := gtables.NewCustomerLedgerEntry(rec)
	custLedgEntry.LockTable()
	custLedgEntry.SetCurrentKey("customer_no")
	custLedgEntry.SetRange("customer_no", rec.No)
	accountingPeriod := gtables.NewAccountingPeriod(rec)
	accountingPeriod.SetRange("closed", false)
	if accountingPeriod.FindFirst() {
		custLedgEntry.SetFilter("posting_date", ">="+accountingPeriod.Starting_date.String())
	}
	if !custLedgEntry.IsEmpty() {
		return apperrors.DeleteHasEntries(gtables.CustomerTableName, rec.No.String())
	}
	custLedgEntry.SetRange("posting_date")
	custLedgEntry.SetRange("open", true)
	if !custLedgEntry.IsEmpty() {
		return apperrors.DeleteOpenEntries(gtables.CustomerTableName, rec.No.String())
	}
	custLedgEntry.SetRange("open")
	custLedgEntry.ModifyAll("customer_no", types.Code(""))
	return nil
}

// OnValidate_No: a number typed by hand must be allowed by the Customer Nos. series (Manual
// Nos.) and takes the customer out of its series; a new customer saved with the number its
// series just issued (AssistEdit) keeps that series. Invoice Disc. Code follows the No.
func (customerLogic) OnValidate_No(rec *gtables.Customer) error {
	xRec := rec.XRec()
	if rec.No != xRec.No {
		salesSetup := gtables.GetSalesReceivablesSetup(rec)
		noSeriesMgt := noseries.New(rec)
		series, issued := noSeriesMgt.IssuedBy(salesSetup.Customer_nos, rec.No)
		if issued && xRec.No == "" {
			rec.No_series = series
		} else {
			err := noSeriesMgt.TestManual(salesSetup.Customer_nos)
			if err != nil {
				return err
			}
			rec.No_series = ""
		}
	}
	rec.Invoice_disc_code = rec.No
	return nil
}

// OnValidate_Name: Search Name follows the name while it was blank or the old name
func (customerLogic) OnValidate_Name(rec *gtables.Customer) error {
	xRec := rec.XRec()
	if rec.Search_name == "" || string(rec.Search_name) == system.UpperCase(xRec.Name.String()) {
		rec.Search_name = types.NewCode(system.CopyStr(rec.Name.String(), 1, 50))
	}
	return nil
}

// OnValidate_Post_code: a known post code gives the city
func (customerLogic) OnValidate_Post_code(rec *gtables.Customer) error {
	postCode := gtables.NewPostCode(rec)
	if rec.Post_code != "" && postCode.Get(rec.Post_code) {
		rec.City = postCode.City
	}
	return nil
}

// OnValidate_Gen_bus_posting_group fills in the group's default Tax Bus. Posting Group
func (customerLogic) OnValidate_Gen_bus_posting_group(rec *gtables.Customer) error {
	xRec := rec.XRec()
	if xRec.Gen_bus_posting_group != rec.Gen_bus_posting_group {
		genBusPostingGroup := gtables.NewGenBusinessPostingGroup(rec)
		autoInsert, err := ValidateTaxBusPostingGroup(genBusPostingGroup, rec.Gen_bus_posting_group)
		if err != nil {
			return err
		}
		if autoInsert {
			err = rec.Validate("tax_bus_posting_group", genBusPostingGroup.Def_tax_bus_posting_group)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// OnValidate_Payment_terms_code: the payment terms must be active (Payment Terms' Active)
func (customerLogic) OnValidate_Payment_terms_code(rec *gtables.Customer) error {
	if rec.Payment_terms_code != "" {
		paymentTerms := gtables.NewPaymentTerms(rec)
		err := paymentTerms.GetOrError(rec.Payment_terms_code)
		if err != nil {
			return err
		}
		if !paymentTerms.Active {
			return apperrors.RelatedInactive(gtables.CustomerTableName, "payment_terms_code", rec.Payment_terms_code.String())
		}
	}
	return nil
}

// AssistEditCustomer lets the user pick another number series for the customer — the Customer
// Nos. series or one related to it — and gives the customer that series' next number
// (AssistEdit of the No. field); false when the user cancelled
func AssistEditCustomer(rec *gtables.Customer) (bool, error) {
	salesSetup := gtables.GetSalesReceivablesSetup(rec)
	err := salesSetup.TestField("customer_nos")
	if err != nil {
		return false, err
	}
	noSeriesMgt := noseries.New(rec)
	xRec := rec.XRec()
	noSeries := rec.No_series
	picked, err := noSeriesMgt.SelectSeries(salesSetup.Customer_nos, xRec.No_series, &noSeries)
	if err != nil {
		return false, err
	}
	if picked {
		err = noSeriesMgt.SetSeries(&rec.No)
		if err != nil {
			return false, err
		}
		rec.No_series = noSeries
		rec.Invoice_disc_code = rec.No
	}
	return picked, nil
}
