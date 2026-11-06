package genjnl

import (
	"testing"

	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// inTx runs fn in one transaction, committed as the codeunits commit (the balance check refuses
// unbalanced G/L entries)
func inTx(t *testing.T, src ftables.Source, fn func(txSrc ftables.Source) error) error {
	t.Helper()
	tx, err := ftables.BeginTransaction(src.GetDB())
	must(t, err)
	err = fn(ftables.DBSource(tx, src.GetCompany(), src.GetDBType()))
	if err != nil {
		_ = ftables.Rollback(tx)
		return err
	}
	return ftables.Commit(tx)
}

// markCust gives customer entries an Applies-to ID (Set Applies-to ID)
func markCust(t *testing.T, src ftables.Source, customerNo, id string, docNos ...string) {
	t.Helper()
	for _, docNo := range docNos {
		entry := custEntry(t, src, customerNo, docNo)
		entry.Applies_to_id = types.Code(id)
		must(t, entry.ModifyOrError(false))
	}
}

func custApplications(src ftables.Source, entryNo int, unapplied bool) int {
	entryAppl := gtables.NewEntryApplication(src)
	entryAppl.SetRange("account_type", gtables.EntryApplication_Account_type.Customer)
	entryAppl.SetRange("ledger_entry_no", entryNo)
	entryAppl.SetRange("unapplied", unapplied)
	return entryAppl.Count()
}

// An invoice and a payment posted without application are applied afterwards (Post Application):
// both close, the payment discount goes to its account against the receivables; Unapply Entries
// gives both entries their remaining amounts back and posts the discount back, and the entries can
// be applied again
func TestPostApplicationAndUnapply(t *testing.T) {
	src := demoCompany(t)
	testCustomer(t, src, "TA1", "DOMESTIC", "", false)
	addLine(t, src, "SALES", "DEFAULT", lineSpec{lineNo: 10000, docNo: "AINV1", docType: invoice, accType: custType, accNo: "TA1", amount: "1000", balNo: "1210",
		pmtDiscPercent: "2", pmtDiscDay: 25})
	_, err := postBatch(t, src, "SALES", "DEFAULT")
	must(t, err)
	addLine(t, src, "CASHRCPT", "DEFAULT", lineSpec{lineNo: 10000, docNo: "APAY1", docType: payment, accType: custType, accNo: "TA1", amount: "-980", postingDay: 20,
		balType: gtables.GenJournalLine_Bal_account_type.BankAccount, balNo: "B010"})
	_, err = postBatch(t, src, "CASHRCPT", "DEFAULT")
	must(t, err)
	if pay := custEntry(t, src, "TA1", "APAY1"); !pay.Open || !pay.Remaining_amount.Equal(dec("-980")) {
		t.Fatalf("payment posted without application: open %v remaining %s", pay.Open, pay.Remaining_amount)
	}

	markCust(t, src, "TA1", "TEST", "AINV1", "APAY1")
	must(t, inTx(t, src, func(txSrc ftables.Source) error {
		return PostCustApplication(txSrc, custEntry(t, txSrc, "TA1", "APAY1"))
	}))
	inv, pay := custEntry(t, src, "TA1", "AINV1"), custEntry(t, src, "TA1", "APAY1")
	if inv.Open || pay.Open || !inv.Pmt_disc_given_lcy.Equal(dec("20")) || inv.Closed_by_entry_no != pay.Entry_no || pay.Applies_to_id != "" {
		t.Fatalf("applied: invoice open %v discount %s closed by %d, payment open %v id %q",
			inv.Open, inv.Pmt_disc_given_lcy, inv.Closed_by_entry_no, pay.Open, pay.Applies_to_id)
	}
	if !glAmount(src, "3150", "APAY1").Equal(dec("20")) || !glAmount(src, "1410", "APAY1").Equal(dec("-1000")) {
		t.Errorf("applied: discount account %s, receivables %s", glAmount(src, "3150", "APAY1"), glAmount(src, "1410", "APAY1"))
	}
	if custApplications(src, inv.Entry_no, false) != 1 || custApplications(src, pay.Entry_no, false) != 1 {
		t.Errorf("application register: invoice %d, payment %d", custApplications(src, inv.Entry_no, false), custApplications(src, pay.Entry_no, false))
	}

	var applicationNo int
	must(t, inTx(t, src, func(txSrc ftables.Source) error {
		var err error
		applicationNo, err = UnapplyCustEntry(txSrc, custEntry(t, txSrc, "TA1", "AINV1"))
		return err
	}))
	inv, pay = custEntry(t, src, "TA1", "AINV1"), custEntry(t, src, "TA1", "APAY1")
	if applicationNo == 0 || !inv.Open || !pay.Open || !inv.Remaining_amount.Equal(dec("1000")) || !pay.Remaining_amount.Equal(dec("-980")) ||
		!pay.Amount.Equal(dec("-980")) || !inv.Pmt_disc_given_lcy.IsZero() || inv.Closed_by_entry_no != 0 {
		t.Fatalf("unapplied: invoice open %v remaining %s discount %s closed by %d, payment open %v remaining %s amount %s",
			inv.Open, inv.Remaining_amount, inv.Pmt_disc_given_lcy, inv.Closed_by_entry_no, pay.Open, pay.Remaining_amount, pay.Amount)
	}
	if !glAmount(src, "3150", "APAY1").IsZero() || !glAmount(src, "1410", "APAY1").Equal(dec("-980")) {
		t.Errorf("unapplied: discount account %s, receivables %s", glAmount(src, "3150", "APAY1"), glAmount(src, "1410", "APAY1"))
	}
	if custApplications(src, inv.Entry_no, false) != 0 || custApplications(src, inv.Entry_no, true) != 1 {
		t.Errorf("unapplied register: open %d, unapplied %d", custApplications(src, inv.Entry_no, false), custApplications(src, inv.Entry_no, true))
	}
	// nothing left to unapply; the entries apply again
	err = inTx(t, src, func(txSrc ftables.Source) error {
		_, err := UnapplyCustEntry(txSrc, custEntry(t, txSrc, "TA1", "AINV1"))
		return err
	})
	if err == nil {
		t.Error("a second unapply of the same entry was not refused")
	}
	markCust(t, src, "TA1", "TEST", "AINV1", "APAY1")
	must(t, inTx(t, src, func(txSrc ftables.Source) error {
		return PostCustApplication(txSrc, custEntry(t, txSrc, "TA1", "APAY1"))
	}))
	if inv = custEntry(t, src, "TA1", "AINV1"); inv.Open {
		t.Error("applying again left the invoice open")
	}
}

// A payment applied when it was posted (Applies-to Doc. No.) is unapplied; two partial payments
// of one invoice are unapplied in the order they came: the first one only after the second
func TestUnapplyPostedPaymentsInOrder(t *testing.T) {
	src := demoCompany(t)
	testCustomer(t, src, "TA2", "DOMESTIC", "", false)
	addLine(t, src, "SALES", "DEFAULT", lineSpec{lineNo: 10000, docNo: "AINV2", docType: invoice, accType: custType, accNo: "TA2", amount: "1000", balNo: "1210"})
	_, err := postBatch(t, src, "SALES", "DEFAULT")
	must(t, err)
	for _, p := range []struct{ docNo, amount string }{{"APAY2", "-400"}, {"APAY3", "-600"}} {
		addLine(t, src, "CASHRCPT", "DEFAULT", lineSpec{lineNo: 10000, docNo: p.docNo, docType: payment, accType: custType, accNo: "TA2", amount: p.amount,
			balType: gtables.GenJournalLine_Bal_account_type.BankAccount, balNo: "B010", appliesDocType: gtables.GenJournalLine_Applies_to_doc_type.Invoice, appliesDocNo: "AINV2"})
		_, err = postBatch(t, src, "CASHRCPT", "DEFAULT")
		must(t, err)
	}
	if inv := custEntry(t, src, "TA2", "AINV2"); inv.Open {
		t.Fatalf("invoice still open after both payments (remaining %s)", inv.Remaining_amount)
	}
	unapply := func(docNo string) error {
		return inTx(t, src, func(txSrc ftables.Source) error {
			_, err := UnapplyCustEntry(txSrc, custEntry(t, txSrc, "TA2", docNo))
			return err
		})
	}
	if err = unapply("APAY2"); err == nil {
		t.Fatal("the first payment was unapplied while the invoice was applied again later")
	}
	must(t, unapply("APAY3"))
	if inv := custEntry(t, src, "TA2", "AINV2"); !inv.Open || !inv.Remaining_amount.Equal(dec("600")) {
		t.Errorf("after unapplying the second payment: open %v remaining %s, want 600", inv.Open, inv.Remaining_amount)
	}
	must(t, unapply("APAY2"))
	inv, pay := custEntry(t, src, "TA2", "AINV2"), custEntry(t, src, "TA2", "APAY2")
	if !inv.Remaining_amount.Equal(dec("1000")) || !pay.Open || !pay.Remaining_amount.Equal(dec("-400")) {
		t.Errorf("after unapplying both: invoice remaining %s, first payment open %v remaining %s", inv.Remaining_amount, pay.Open, pay.Remaining_amount)
	}
	// the receivables account still holds the invoice and both payments
	if !glAmount(src, "1410", "APAY2").Equal(dec("-400")) || !glAmount(src, "1410", "APAY3").Equal(dec("-600")) {
		t.Errorf("receivables %s / %s", glAmount(src, "1410", "APAY2"), glAmount(src, "1410", "APAY3"))
	}
}

// Vendor entries: a payment posted without application is applied to the invoice afterwards and
// unapplied again
func TestVendPostApplicationAndUnapply(t *testing.T) {
	src := demoCompany(t)
	vend := gtables.NewVendor(src)
	vend.No = "TVA"
	vend.Name = "Test vendor"
	vend.Vendor_posting_group = "DOMESTIC"
	must(t, vend.InsertOrError(false))
	inv := addLine(t, src, "PURCHASES", "DEFAULT", lineSpec{lineNo: 10000, docNo: "VINV1", docType: invoice, accType: vendType, accNo: "TVA", amount: "-500", balNo: "1270"})
	inv.External_document_no = "VEXT-1"
	must(t, inv.ModifyOrError(false))
	_, err := postBatch(t, src, "PURCHASES", "DEFAULT")
	must(t, err)
	addLine(t, src, "PAYMENT", "DEFAULT", lineSpec{lineNo: 10000, docNo: "VPAY1", docType: payment, accType: vendType, accNo: "TVA", amount: "500",
		balType: gtables.GenJournalLine_Bal_account_type.BankAccount, balNo: "B010"})
	_, err = postBatch(t, src, "PAYMENT", "DEFAULT")
	must(t, err)
	vendEntry := func(src ftables.Source, docNo string) *gtables.VendorLedgerEntry {
		entry := gtables.NewVendorLedgerEntry(src)
		entry.SetRange("vendor_no", "TVA")
		entry.SetRange("document_no", docNo)
		if !entry.FindFirst() {
			t.Fatalf("no vendor entry %s", docNo)
		}
		return entry
	}
	for _, docNo := range []string{"VINV1", "VPAY1"} {
		entry := vendEntry(src, docNo)
		entry.Applies_to_id = "TEST"
		must(t, entry.ModifyOrError(false))
	}
	must(t, inTx(t, src, func(txSrc ftables.Source) error { return PostVendApplication(txSrc, vendEntry(txSrc, "VPAY1")) }))
	if vendEntry(src, "VINV1").Open || vendEntry(src, "VPAY1").Open {
		t.Fatal("vendor entries still open after Post Application")
	}
	must(t, inTx(t, src, func(txSrc ftables.Source) error {
		_, err := UnapplyVendEntry(txSrc, vendEntry(txSrc, "VPAY1"))
		return err
	}))
	if !vendEntry(src, "VINV1").Remaining_amount.Equal(dec("-500")) || !vendEntry(src, "VPAY1").Remaining_amount.Equal(dec("500")) {
		t.Errorf("unapplied: invoice remaining %s, payment remaining %s", vendEntry(src, "VINV1").Remaining_amount, vendEntry(src, "VPAY1").Remaining_amount)
	}
}
