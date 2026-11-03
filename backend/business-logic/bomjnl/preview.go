package bomjnl

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/postingpreview"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/functions"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Preview Posting on the BOM journals
func init() {
	functions.Register("table", gtables.BOMJournalLineTableName, "PreviewPosting", functions.For(BOMJnlPostPreview), functions.WithDialog())
}

// BOMJnlPostPreview posts the line's journal batch as Post would (codeunit 33), in a transaction that is rolled
// back, and opens Posting Preview on the entries it would write; nothing is saved — the lines
// stay, the number series do not move
func BOMJnlPostPreview(rec *gtables.BOMJournalLine) (fcodeunits.Result, error) {
	preview, err := postingpreview.Run(rec.GetDB(), rec.CompanyName(), rec.GetDBType(), fcodeunits.CurrentUserID(), func(src ftables.Source) error {
		line := gtables.NewBOMJournalLine(src)
		line.Journal_template_name = rec.Journal_template_name
		line.Journal_batch_name = rec.Journal_batch_name
		_, err := NewBOMJnlPostBatch(src).Run(line)
		return err
	})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return preview.Result()
}
