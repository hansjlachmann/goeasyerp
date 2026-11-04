package demodata

import (
	"fmt"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/intrastatjnl"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// insertDemoIntrastat: the Intrastat journal (template INTRASTAT, batch of this month's
// Statistics Period) with this month's shipments and receipts with Denmark (Get Entries)
func insertDemoIntrastat(src ftables.Source, today time.Time, counts *Counts) error {
	intraJnlTemplate := gtables.NewIntrastatJnlTemplate(src)
	intraJnlTemplate.Name = "INTRASTAT"
	intraJnlTemplate.Description = "Intrastat Journal"
	err := intraJnlTemplate.InsertOrError(true)
	if err != nil {
		return err
	}
	intraJnlBatch := gtables.NewIntrastatJnlBatch(src)
	intraJnlBatch.Journal_template_name = intraJnlTemplate.Name
	intraJnlBatch.Name = types.Code(today.Format("2006-01"))
	intraJnlBatch.Description = types.Text(fmt.Sprintf("Intrastat %s", today.Format("January 2006")))
	err = intraJnlBatch.Validate("statistics_period", types.Code(today.Format("0601")))
	if err == nil {
		err = intraJnlBatch.InsertOrError(true)
	}
	if err != nil {
		return err
	}
	startDate, endDate, _ := intrastatjnl.PeriodDates(intraJnlBatch.Statistics_period, types.NewDateFromTime(today))
	counts.IntrastatJnlLines, err = intrastatjnl.GetEntries(intraJnlBatch, startDate, endDate, types.Decimal{})
	return err
}
