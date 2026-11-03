package resjnl

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/postingpreview"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/functions"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Preview Posting on the resource journals
func init() {
	functions.Register("table", gtables.ResJournalLineTableName, "PreviewPosting", functions.For(ResJnlPostPreview), functions.WithDialog())
}

// ResJnlPostPreview posts the line's journal batch as Post would (codeunit 213), in a transaction that is rolled
// back, and opens Posting Preview on the entries it would write; nothing is saved — the lines
// stay, the number series do not move
func ResJnlPostPreview(rec *gtables.ResJournalLine) (fcodeunits.Result, error) {
	preview, err := postingpreview.Run(rec.GetDB(), rec.CompanyName(), rec.GetDBType(), fcodeunits.CurrentUserID(), func(src ftables.Source) error {
		line := gtables.NewResJournalLine(src)
		line.Journal_template_name = rec.Journal_template_name
		line.Journal_batch_name = rec.Journal_batch_name
		_, err := NewResJnlPostBatch(src).Run(line)
		return err
	})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return preview.Result()
}
