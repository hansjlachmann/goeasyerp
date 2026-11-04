package reports

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// printerOutputs: the printers of Printer Selection (78) are the outputs of a printout
var printerOutputs = map[gtables.PrinterSelectionPrinter_name]string{
	gtables.PrinterSelection_Printer_name.Preview: OutputPreview,
	gtables.PrinterSelection_Printer_name.Print:   OutputPrint,
	gtables.PrinterSelection_Printer_name.PDF:     OutputPDF,
}

// PrinterOutput returns the output a report prints to for the current user by Printer
// Selection: the user's line for the report, the user's line for all reports (Report ID 0), the
// line of all users (blank User ID) for the report, the line for all users and reports — else
// the fallback
func PrinterOutput(src ftables.Source, reportID int, fallback string) string {
	userID := types.Code(fcodeunits.CurrentUserID())
	selection := gtables.NewPrinterSelection(src)
	users := []types.Code{userID, ""}
	if userID == "" {
		users = users[1:]
	}
	for _, user := range users {
		for _, id := range []int{reportID, 0} {
			selection.Reset()
			selection.SetRange("user_id", user)
			selection.SetRange("report_id", id)
			if selection.FindFirst() {
				return printerOutputs[selection.Printer_name]
			}
		}
	}
	return fallback
}
