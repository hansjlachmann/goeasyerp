package reports_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Item Ledger Entries › Application: Invoicing Entries opens page 63 on the entries the entry
// closed, Applied Entries page 523 on its applications (inbound: those applied to it)
func TestItemLedgerEntryApplicationLinks(t *testing.T) {
	src := documentsCompany(t)
	inbound := gtables.NewItemLedgerEntry(src)
	inbound.SetRange("positive", true)
	if !inbound.FindFirst() {
		t.Fatal("no inbound item entry")
	}
	outbound := gtables.NewItemLedgerEntry(src)
	outbound.SetRange("positive", false)
	if !outbound.FindFirst() {
		t.Fatal("no outbound item entry")
	}

	opened := pageAnswers(t)
	_, err := tables.ItemLedgEntryShowInvoicingEntries(inbound)
	mustOK(t, err)
	_, err = tables.ItemLedgEntryShowAppliedEntries(inbound)
	mustOK(t, err)
	_, err = tables.ItemLedgEntryShowAppliedEntries(outbound)
	mustOK(t, err)
	if len(*opened) != 3 {
		t.Fatalf("pages opened: %d", len(*opened))
	}
	filters := func(i int) string { return fmt.Sprint((*opened)[i].Data["filters"]) }
	want := []struct {
		page     int
		contains []string
	}{
		{tables.AppliedInvoicingItemEntriesPageID, []string{"closed_by_entry_no", fmt.Sprint(inbound.Entry_no), "<>"}},
		{tables.ClosingItemApplnEntriesPageID, []string{"applied_to_entry_no", fmt.Sprint(inbound.Entry_no), "applied_from_entry_no"}},
		{tables.ClosingItemApplnEntriesPageID, []string{"item_ledger_entry_no", "applied_from_entry_no", fmt.Sprint(outbound.Entry_no)}},
	}
	for i, w := range want {
		if (*opened)[i].Data["page_id"] != w.page {
			t.Errorf("link %d: page %v, want %d", i, (*opened)[i].Data["page_id"], w.page)
		}
		for _, c := range w.contains {
			if !strings.Contains(filters(i), c) {
				t.Errorf("link %d: filters %s without %q", i, filters(i), c)
			}
		}
	}
}
