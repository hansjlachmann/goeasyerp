package itemjnl

import (
	"strings"
	"testing"

	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Serial numbers: an item comes in with its serial numbers (one piece each) and goes out with
// them — the outbound entry takes that serial number's inbound entry, not the oldest; a serial
// number is on inventory once, cannot leave twice, and a line with one posts one piece; a
// Specific item needs one
func TestSerialNumbers(t *testing.T) {
	src := demoCompany(t)
	testItem(t, src, "TS1", gtables.Item_Costing_method.Specific, "")
	purchase, sale := gtables.ItemJournalLine_Entry_type.Purchase, gtables.ItemJournalLine_Entry_type.Sale
	_, err := postItemLines(t, src,
		itemLine{entryType: purchase, itemNo: "TS1", qty: "1", unitAmount: "100", serialNo: "S1"},
		itemLine{entryType: purchase, itemNo: "TS1", qty: "1", unitAmount: "150", serialNo: "S2"},
		itemLine{entryType: sale, itemNo: "TS1", qty: "1", serialNo: "S2", day: 11})
	must(t, err)
	list := entries(src, "TS1")
	if len(list) != 3 || list[0].Serial_no != "S1" || !list[0].Open || list[1].Open || list[2].Serial_no != "S2" || list[2].Open {
		for _, e := range list {
			t.Logf("entry %d %s %s open %v remaining %s", e.Entry_no, e.Serial_no, e.Quantity, e.Open, e.Remaining_quantity)
		}
		t.Fatal("the sale of S2 did not take S2's purchase")
	}

	for _, c := range []struct {
		name string
		line itemLine
		want string
	}{
		{"S2 again", itemLine{entryType: sale, itemNo: "TS1", qty: "1", serialNo: "S2", documentNo: "T00002"}, "is not on inventory"},
		{"S1 received twice", itemLine{entryType: purchase, itemNo: "TS1", qty: "1", unitAmount: "100", serialNo: "S1", documentNo: "T00002"}, "already on inventory"},
		{"two pieces", itemLine{entryType: purchase, itemNo: "TS1", qty: "2", unitAmount: "100", serialNo: "S9"}, "must be 1 or -1"},
		{"no serial no.", itemLine{entryType: sale, itemNo: "TS1", qty: "1"}, ""},
	} {
		gtables.NewItemJournalLine(src).DeleteAll()
		_, err = postItemLines(t, src, c.line)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
}

// Lot numbers: a sale of a lot takes that lot's entries, not the oldest; a transfer gives the
// quantity a new lot number at the new location
func TestLotNumbers(t *testing.T) {
	src := demoCompany(t)
	testItem(t, src, "TL1", gtables.Item_Costing_method.FIFO, "")
	purchase := gtables.ItemJournalLine_Entry_type.Purchase
	_, err := postItemLines(t, src,
		itemLine{entryType: purchase, itemNo: "TL1", qty: "5", unitAmount: "10", lotNo: "L1"},
		itemLine{entryType: purchase, itemNo: "TL1", qty: "5", unitAmount: "12", lotNo: "L2"},
		itemLine{entryType: gtables.ItemJournalLine_Entry_type.Sale, itemNo: "TL1", qty: "3", lotNo: "L2", day: 11},
		itemLine{entryType: gtables.ItemJournalLine_Entry_type.Transfer, itemNo: "TL1", qty: "2", lotNo: "L1", newLocation: "MAIN", newLotNo: "L1B", day: 12})
	must(t, err)
	remaining := map[string]string{}
	for _, e := range entries(src, "TL1") {
		if e.Open {
			remaining[string(e.Lot_no)] = e.Remaining_quantity.String()
		}
	}
	if remaining["L1"] != "3" || remaining["L2"] != "2" || remaining["L1B"] != "2" {
		t.Errorf("remaining by lot %v, want L1 3, L2 2, L1B 2", remaining)
	}
}

// Bins: the entries keep the line's bin; a sale from a bin takes that bin's entries, not the
// oldest; a transfer moves the quantity to a new bin
func TestBins(t *testing.T) {
	src := demoCompany(t)
	testItem(t, src, "TB1", gtables.Item_Costing_method.FIFO, "")
	for _, code := range []string{"A1", "B1"} {
		bin := gtables.NewBin(src)
		bin.Location_code, bin.Code = "MAIN", types.Code(code)
		must(t, bin.InsertOrError(true))
	}
	purchase := gtables.ItemJournalLine_Entry_type.Purchase
	_, err := postItemLines(t, src,
		itemLine{entryType: purchase, itemNo: "TB1", qty: "5", unitAmount: "10", binCode: "A1"},
		itemLine{entryType: purchase, itemNo: "TB1", qty: "5", unitAmount: "12", binCode: "B1"},
		itemLine{entryType: gtables.ItemJournalLine_Entry_type.Sale, itemNo: "TB1", qty: "3", binCode: "B1", day: 11},
		itemLine{entryType: gtables.ItemJournalLine_Entry_type.Transfer, itemNo: "TB1", qty: "2", binCode: "A1", newLocation: "MAIN", newBinCode: "B1", day: 12})
	must(t, err)
	remaining := map[string]string{}
	for _, e := range entries(src, "TB1") {
		if e.Open {
			remaining[string(e.Bin_code)] = remaining[string(e.Bin_code)] + e.Remaining_quantity.String() + " "
		}
	}
	if remaining["A1"] != "3 " || remaining["B1"] != "2 2 " {
		t.Errorf("remaining by bin %v, want A1 3, B1 2 and 2", remaining)
	}
}
