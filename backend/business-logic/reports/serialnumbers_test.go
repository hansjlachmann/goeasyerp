package reports_test

import (
	"strings"
	"testing"

	"github.com/hansjlachmann/openerp/backend/business-logic/reports"
)

// Serial Number Status/Aging (705) prints the demo office chairs on inventory by serial number
// with their age, Serial Number Sold History (704) the one sold
func TestSerialNumberReports(t *testing.T) {
	src := listCompany(t)
	aging := runList(t, src, reports.SerialNumberStatusAgingID, nil)
	for _, want := range []string{"Serial Number Status/Aging", "Office chair Ergo", "ERGO-0001", "ERGO-0003", "Total Quantity: 2"} {
		if !strings.Contains(aging, want) {
			t.Errorf("705 does not print %q:\n%s", want, aging)
		}
	}
	if strings.Contains(aging, "ERGO-0002") {
		t.Error("705 prints the sold ERGO-0002")
	}
	sold := runList(t, src, reports.SerialNumberSoldHistoryID, nil)
	if !strings.Contains(sold, "ERGO-0002") || strings.Contains(sold, "ERGO-0001") {
		t.Errorf("704:\n%s", sold)
	}
}
