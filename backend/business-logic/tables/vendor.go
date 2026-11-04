package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/noseries"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// vendorLogic is the business logic of table 23 Vendor.
//
// Not migrated yet (their tables come later): OnDelete refuses while purchase orders are
// outstanding; the contact update (codeunit 5003) of OnInsert/OnModify/OnDelete.
type vendorLogic struct{}

func init() {
	gtables.RegisterVendorLogic(vendorLogic{})
	// the No. field's "…" (Vendor Card): pick another number series
	functions.Register("table", gtables.VendorTableName, "AssistEdit", functions.ForAssistEdit(AssistEditVendor), functions.WithAssistEdit())
}

// OnInsert: a vendor without No. gets the next number of the Vendor Nos. series of the
// Purchases & Payables Setup
func (vendorLogic) OnInsert(rec *gtables.Vendor) error {
	if rec.No == "" {
		purchSetup := gtables.GetPurchasesPayablesSetup(rec)
		err := purchSetup.TestField("vendor_nos")
		if err != nil {
			return err
		}
		noSeriesMgt := noseries.New(rec)
		xRec := rec.XRec()
		err = noSeriesMgt.InitSeries(purchSetup.Vendor_nos, xRec.No_series, types.Date{}, &rec.No, &rec.No_series)
		if err != nil {
			return err
		}
		rec.Invoice_disc_code = rec.No
	}
	return nil
}

// OnModify sets Last Date Modified
func (vendorLogic) OnModify(rec *gtables.Vendor) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnRename sets Last Date Modified
func (vendorLogic) OnRename(rec *gtables.Vendor) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnDelete moves the vendor's ledger entries away (MoveVendorEntries); the vendor's comment
// lines, bank accounts and order addresses are deleted
func (vendorLogic) OnDelete(rec *gtables.Vendor) error {
	err := moveVendorEntries(rec)
	if err != nil {
		return err
	}

	DeleteComments(rec, gtables.CommentLine_Table_name.Vendor, rec.No)

	vendBankAcc := gtables.NewVendorBankAccount(rec)
	vendBankAcc.SetRange("vendor_no", rec.No)
	vendBankAcc.DeleteAll()

	orderAddr := gtables.NewOrderAddress(rec)
	orderAddr.SetRange("vendor_no", rec.No)
	orderAddr.DeleteAll()
	return nil
}

// moveVendorEntries: refused while the vendor has entries in a fiscal year that is not closed
// — every entry, as long as there are no accounting periods — or open entries; otherwise the
// entries lose their Vendor No.
func moveVendorEntries(rec *gtables.Vendor) error {
	vendLedgEntry := gtables.NewVendorLedgerEntry(rec)
	vendLedgEntry.LockTable()
	vendLedgEntry.SetCurrentKey("vendor_no")
	vendLedgEntry.SetRange("vendor_no", rec.No)
	accountingPeriod := gtables.NewAccountingPeriod(rec)
	accountingPeriod.SetRange("closed", false)
	if accountingPeriod.FindFirst() {
		vendLedgEntry.SetFilter("posting_date", ">="+accountingPeriod.Starting_date.String())
	}
	if !vendLedgEntry.IsEmpty() {
		return apperrors.DeleteHasEntries(gtables.VendorTableName, rec.No.String())
	}
	vendLedgEntry.SetRange("posting_date")
	vendLedgEntry.SetRange("open", true)
	if !vendLedgEntry.IsEmpty() {
		return apperrors.DeleteOpenEntries(gtables.VendorTableName, rec.No.String())
	}
	vendLedgEntry.SetRange("open")
	vendLedgEntry.ModifyAll("vendor_no", types.Code(""))
	return nil
}

// OnValidate_No: a number typed by hand must be allowed by the Vendor Nos. series (Manual
// Nos.) and takes the vendor out of its series; a new vendor saved with the number its series
// just issued (AssistEdit) keeps that series. Invoice Disc. Code follows the No.
func (vendorLogic) OnValidate_No(rec *gtables.Vendor) error {
	xRec := rec.XRec()
	if rec.No != xRec.No {
		purchSetup := gtables.GetPurchasesPayablesSetup(rec)
		noSeriesMgt := noseries.New(rec)
		series, issued := noSeriesMgt.IssuedBy(purchSetup.Vendor_nos, rec.No)
		if issued && xRec.No == "" {
			rec.No_series = series
		} else {
			err := noSeriesMgt.TestManual(purchSetup.Vendor_nos)
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
func (vendorLogic) OnValidate_Name(rec *gtables.Vendor) error {
	xRec := rec.XRec()
	if rec.Search_name == "" || string(rec.Search_name) == system.UpperCase(xRec.Name.String()) {
		rec.Search_name = types.NewCode(system.CopyStr(rec.Name.String(), 1, 50))
	}
	return nil
}

// OnValidate_Post_code: a known post code gives the city
func (vendorLogic) OnValidate_Post_code(rec *gtables.Vendor) error {
	postCode := gtables.NewPostCode(rec)
	if rec.Post_code != "" && postCode.Get(rec.Post_code) {
		rec.City = postCode.City
	}
	return nil
}

// OnValidate_Gen_bus_posting_group fills in the group's default Tax Bus. Posting Group
func (vendorLogic) OnValidate_Gen_bus_posting_group(rec *gtables.Vendor) error {
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
func (vendorLogic) OnValidate_Payment_terms_code(rec *gtables.Vendor) error {
	if rec.Payment_terms_code != "" {
		paymentTerms := gtables.NewPaymentTerms(rec)
		err := paymentTerms.GetOrError(rec.Payment_terms_code)
		if err != nil {
			return err
		}
		if !paymentTerms.Active {
			return apperrors.RelatedInactive(gtables.VendorTableName, "payment_terms_code", rec.Payment_terms_code.String())
		}
	}
	return nil
}

// AssistEditVendor lets the user pick another number series for the vendor — the Vendor Nos.
// series or one related to it — and gives the vendor that series' next number (AssistEdit of
// the No. field); false when the user cancelled
func AssistEditVendor(rec *gtables.Vendor) (bool, error) {
	purchSetup := gtables.GetPurchasesPayablesSetup(rec)
	err := purchSetup.TestField("vendor_nos")
	if err != nil {
		return false, err
	}
	noSeriesMgt := noseries.New(rec)
	xRec := rec.XRec()
	noSeries := rec.No_series
	picked, err := noSeriesMgt.SelectSeries(purchSetup.Vendor_nos, xRec.No_series, &noSeries)
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
