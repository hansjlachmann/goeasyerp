// Package navigate is page 344 Navigate's FindRecords: the tables with entries of a document
// (its Document No. and Posting Date filters) and how many, as Document Entry lines (table
// 265, temporary). Show opens a line's entries page with the same filters. FindExtRecords finds
// them by an external document no. and business contact (vendor or customer).
package navigate

import (
	"slices"
	"strings"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The entries pages Show opens
const (
	glEntriesPage           = 20
	custLedgEntriesPage     = 25
	vendLedgEntriesPage     = 29
	itemLedgEntriesPage     = 38
	resLedgEntriesPage      = 202
	projectLedgEntriesPage  = 92
	taxEntriesPage          = 315
	bankAccLedgEntriesPage  = 372
	checkLedgEntriesPage    = 374
	physInvtLedgEntriesPage = 390
	faLedgEntriesPage       = 5604
	maintLedgEntriesPage    = 5641
	insCovLedgEntriesPage   = 5647
	salesShptHeadersPage    = 142
	salesInvHeadersPage     = 143
	salesCrMemoHeadersPage  = 144
	purchRcptHeadersPage    = 145
	purchInvHeadersPage     = 146
	purchCrMemoHeadersPage  = 147
)

// entries is what Navigate needs of an entry table's record
type entries interface {
	SetFilter(fieldName, filterExpr string)
	Count() int
	TableCaption(language ...string) string
}

// FindRecords (FindRecords) fills docEntry, a temporary Document Entry, with a line per table
// the user may read that has entries with the document no. and posting date filters (blank:
// any). With both filters blank nothing is searched.
func FindRecords(docEntry *gtables.DocumentEntry, docNoFilter, postingDateFilter string) error {
	docEntry.DeleteAll()
	if docNoFilter == "" && postingDateFilter == "" {
		return nil
	}
	if fcodeunits.CanRead("Sales_Shipment_Header") {
		salesShptHeader := gtables.NewSalesShipmentHeader(docEntry)
		err := findPostedDocs(docEntry, salesShptHeader, gtables.SalesShipmentHeaderTableID, salesShptHeadersPage, docNoFilter, postingDateFilter)
		if err != nil {
			return err
		}
	}
	if fcodeunits.CanRead("Sales_Invoice_Header") {
		salesInvHeader := gtables.NewSalesInvoiceHeader(docEntry)
		err := findPostedDocs(docEntry, salesInvHeader, gtables.SalesInvoiceHeaderTableID, salesInvHeadersPage, docNoFilter, postingDateFilter)
		if err != nil {
			return err
		}
	}
	if fcodeunits.CanRead("Sales_Cr_Memo_Header") {
		salesCrMemoHeader := gtables.NewSalesCrMemoHeader(docEntry)
		err := findPostedDocs(docEntry, salesCrMemoHeader, gtables.SalesCrMemoHeaderTableID, salesCrMemoHeadersPage, docNoFilter, postingDateFilter)
		if err != nil {
			return err
		}
	}
	if fcodeunits.CanRead("Purch_Rcpt_Header") {
		purchRcptHeader := gtables.NewPurchRcptHeader(docEntry)
		err := findPostedDocs(docEntry, purchRcptHeader, gtables.PurchRcptHeaderTableID, purchRcptHeadersPage, docNoFilter, postingDateFilter)
		if err != nil {
			return err
		}
	}
	if fcodeunits.CanRead("Purch_Inv_Header") {
		purchInvHeader := gtables.NewPurchInvHeader(docEntry)
		err := findPostedDocs(docEntry, purchInvHeader, gtables.PurchInvHeaderTableID, purchInvHeadersPage, docNoFilter, postingDateFilter)
		if err != nil {
			return err
		}
	}
	if fcodeunits.CanRead("Purch_Cr_Memo_Hdr") {
		purchCrMemoHeader := gtables.NewPurchCrMemoHdr(docEntry)
		err := findPostedDocs(docEntry, purchCrMemoHeader, gtables.PurchCrMemoHdrTableID, purchCrMemoHeadersPage, docNoFilter, postingDateFilter)
		if err != nil {
			return err
		}
	}
	if fcodeunits.CanRead("G_L_Entry") {
		glEntry := gtables.NewGLEntry(docEntry)
		err := findEntries(docEntry, glEntry, gtables.GLEntryTableID, glEntriesPage, docNoFilter, postingDateFilter)
		if err != nil {
			return err
		}
	}
	if fcodeunits.CanRead("Tax_Entry") {
		taxEntry := gtables.NewTaxEntry(docEntry)
		err := findEntries(docEntry, taxEntry, gtables.TaxEntryTableID, taxEntriesPage, docNoFilter, postingDateFilter)
		if err != nil {
			return err
		}
	}
	if fcodeunits.CanRead("Customer_ledger_entry") {
		custLedgEntry := gtables.NewCustomerLedgerEntry(docEntry)
		err := findEntries(docEntry, custLedgEntry, gtables.CustomerLedgerEntryTableID, custLedgEntriesPage, docNoFilter, postingDateFilter)
		if err != nil {
			return err
		}
	}
	if fcodeunits.CanRead("Vendor_ledger_entry") {
		vendLedgEntry := gtables.NewVendorLedgerEntry(docEntry)
		err := findEntries(docEntry, vendLedgEntry, gtables.VendorLedgerEntryTableID, vendLedgEntriesPage, docNoFilter, postingDateFilter)
		if err != nil {
			return err
		}
	}
	if fcodeunits.CanRead("Bank_Account_Ledger_Entry") {
		bankAccLedgEntry := gtables.NewBankAccountLedgerEntry(docEntry)
		err := findEntries(docEntry, bankAccLedgEntry, gtables.BankAccountLedgerEntryTableID, bankAccLedgEntriesPage, docNoFilter, postingDateFilter)
		if err != nil {
			return err
		}
	}
	if fcodeunits.CanRead("Check_Ledger_Entry") {
		checkLedgEntry := gtables.NewCheckLedgerEntry(docEntry)
		err := findEntries(docEntry, checkLedgEntry, gtables.CheckLedgerEntryTableID, checkLedgEntriesPage, docNoFilter, postingDateFilter)
		if err != nil {
			return err
		}
	}
	if fcodeunits.CanRead("Item_Ledger_Entry") {
		itemLedgEntry := gtables.NewItemLedgerEntry(docEntry)
		err := findEntries(docEntry, itemLedgEntry, gtables.ItemLedgerEntryTableID, itemLedgEntriesPage, docNoFilter, postingDateFilter)
		if err != nil {
			return err
		}
	}
	if fcodeunits.CanRead("Phys_Inventory_Ledger_Entry") {
		physInvtLedgEntry := gtables.NewPhysInventoryLedgerEntry(docEntry)
		err := findEntries(docEntry, physInvtLedgEntry, gtables.PhysInventoryLedgerEntryTableID, physInvtLedgEntriesPage, docNoFilter, postingDateFilter)
		if err != nil {
			return err
		}
	}
	if fcodeunits.CanRead("Res_Ledger_Entry") {
		resLedgEntry := gtables.NewResLedgerEntry(docEntry)
		err := findEntries(docEntry, resLedgEntry, gtables.ResLedgerEntryTableID, resLedgEntriesPage, docNoFilter, postingDateFilter)
		if err != nil {
			return err
		}
	}
	if fcodeunits.CanRead("Project_Ledger_Entry") {
		projectLedgEntry := gtables.NewProjectLedgerEntry(docEntry)
		err := findEntries(docEntry, projectLedgEntry, gtables.ProjectLedgerEntryTableID, projectLedgEntriesPage, docNoFilter, postingDateFilter)
		if err != nil {
			return err
		}
	}
	if fcodeunits.CanRead("FA_Ledger_Entry") {
		faLedgEntry := gtables.NewFALedgerEntry(docEntry)
		err := findEntries(docEntry, faLedgEntry, gtables.FALedgerEntryTableID, faLedgEntriesPage, docNoFilter, postingDateFilter)
		if err != nil {
			return err
		}
	}
	if fcodeunits.CanRead("Maintenance_Ledger_Entry") {
		maintLedgEntry := gtables.NewMaintenanceLedgerEntry(docEntry)
		err := findEntries(docEntry, maintLedgEntry, gtables.MaintenanceLedgerEntryTableID, maintLedgEntriesPage, docNoFilter, postingDateFilter)
		if err != nil {
			return err
		}
	}
	if fcodeunits.CanRead("Ins_Coverage_Ledger_Entry") {
		insCoverageLedgEntry := gtables.NewInsCoverageLedgerEntry(docEntry)
		err := findEntries(docEntry, insCoverageLedgEntry, gtables.InsCoverageLedgerEntryTableID, insCovLedgEntriesPage, docNoFilter, postingDateFilter)
		if err != nil {
			return err
		}
	}
	return nil
}

// findEntries counts an entry table's entries in the filters and adds its line
func findEntries(docEntry *gtables.DocumentEntry, entryRec entries, tableID, pageID int, docNoFilter, postingDateFilter string) error {
	if docNoFilter != "" {
		entryRec.SetFilter("document_no", docNoFilter)
	}
	if postingDateFilter != "" {
		entryRec.SetFilter("posting_date", postingDateFilter)
	}
	return insertIntoDocEntry(docEntry, tableID, pageID, entryRec.TableCaption(fcodeunits.CurrentLanguage()), entryRec.Count(), docNoFilter, "", postingDateFilter)
}

// findPostedDocs counts a posted document table's documents — the document no. filter on their
// No. — and adds its line (Show filters the documents' list on No.)
func findPostedDocs(docEntry *gtables.DocumentEntry, docRec entries, tableID, pageID int, docNoFilter, postingDateFilter string) error {
	if docNoFilter != "" {
		docRec.SetFilter("no", docNoFilter)
	}
	if postingDateFilter != "" {
		docRec.SetFilter("posting_date", postingDateFilter)
	}
	return insertIntoDocEntry(docEntry, tableID, pageID, docRec.TableCaption(fcodeunits.CurrentLanguage()), docRec.Count(), "", docNoFilter, postingDateFilter)
}

// insertIntoDocEntry (InsertIntoDocEntry): a line for a table with entries (none for 0), with
// the filters Show applies — Document No. on entries, No. on posted documents
func insertIntoDocEntry(docEntry *gtables.DocumentEntry, docTableID, docPageID int, docTableName string, docNoOfRecords int, docNoFilter, noFilter, postingDateFilter string) error {
	if docNoOfRecords == 0 {
		return nil
	}
	entryNo := docEntry.Entry_no
	docEntry.Init()
	docEntry.Entry_no = entryNo + 1
	docEntry.Table_id = docTableID
	docEntry.Table_name = types.Text(system.CopyStr(docTableName, 1, 100))
	docEntry.No_of_records = docNoOfRecords
	docEntry.Page_id = docPageID
	docEntry.Document_no_filter = types.Text(docNoFilter)
	docEntry.No_filter = types.Text(noFilter)
	docEntry.Posting_date_filter = types.Text(postingDateFilter)
	return docEntry.InsertOrError(false)
}

// The business contact types of the external search (page 344's Business Contact Type)
const (
	ContactTypeVendor   = "0"
	ContactTypeCustomer = "1"
)

// maxExtDocuments: the most documents an external search may find (classic: the filters must
// fit their fields — TooLongFilter)
const maxExtDocuments = 50

// FindExtRecords (FindExtRecords): the vendor's — or customer's — ledger entries with the
// external document no. filter (and the business contact no. filter) give the document nos. and
// posting dates FindRecords searches with; found is false when no entry has it. Too many
// documents is an error (TooLongFilter).
func FindExtRecords(docEntry *gtables.DocumentEntry, contactType, contactNo, extDocNo string) (bool, error) {
	var docNos, dates []string
	makeExtFilter := func(postingDate types.Date, documentNo types.Code) {
		date := postingDate.String()
		if !slices.Contains(dates, date) {
			dates = append(dates, date)
		}
		if !slices.Contains(docNos, documentNo.String()) {
			docNos = append(docNos, documentNo.String())
		}
	}
	switch contactType {
	case ContactTypeCustomer:
		custLedgEntry := gtables.NewCustomerLedgerEntry(docEntry)
		if extDocNo != "" {
			custLedgEntry.SetFilter("external_document_no", extDocNo)
		}
		if contactNo != "" {
			custLedgEntry.SetFilter("customer_no", contactNo)
		}
		for found := custLedgEntry.FindSet(); found; found = custLedgEntry.Next() {
			makeExtFilter(custLedgEntry.Posting_date, custLedgEntry.Document_no)
		}
	default:
		vendLedgEntry := gtables.NewVendorLedgerEntry(docEntry)
		if extDocNo != "" {
			vendLedgEntry.SetFilter("external_document_no", extDocNo)
		}
		if contactNo != "" {
			vendLedgEntry.SetFilter("vendor_no", contactNo)
		}
		for found := vendLedgEntry.FindSet(); found; found = vendLedgEntry.Next() {
			makeExtFilter(vendLedgEntry.Posting_date, vendLedgEntry.Document_no)
		}
	}
	if len(docNos) == 0 {
		docEntry.DeleteAll()
		return false, nil
	}
	if len(docNos) > maxExtDocuments {
		if contactNo == "" {
			return false, system.Error(i18n.GetInstance().Message("NAVIGATE_TOO_MANY_EXT_DOCS_CONTACT", fcodeunits.CurrentLanguage()))
		}
		return false, system.Error(i18n.GetInstance().Message("NAVIGATE_TOO_MANY_EXT_DOCS", fcodeunits.CurrentLanguage()))
	}
	return true, FindRecords(docEntry, strings.Join(docNos, "|"), strings.Join(dates, "|"))
}
