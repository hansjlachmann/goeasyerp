package itemjnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/postingpreview"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Preview Posting on the item journals (item, transfer, phys. inventory, recurring)
func init() {
	functions.Register("table", gtables.ItemJournalLineTableName, "PreviewPosting", functions.For(ItemJnlPostPreview), functions.WithDialog())
}

// ItemJnlPostPreview posts the line's journal batch as Post would (codeunit 23), in a transaction that is rolled
// back, and opens Posting Preview on the entries it would write; nothing is saved — the lines
// stay, the number series do not move
func ItemJnlPostPreview(rec *gtables.ItemJournalLine) (fcodeunits.Result, error) {
	preview, err := postingpreview.Run(rec.GetDB(), rec.CompanyName(), rec.GetDBType(), fcodeunits.CurrentUserID(), func(src ftables.Source) error {
		line := gtables.NewItemJournalLine(src)
		line.Journal_template_name = rec.Journal_template_name
		line.Journal_batch_name = rec.Journal_batch_name
		_, err := NewItemJnlPostBatch(src).Run(line)
		return err
	})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return preview.Result()
}
