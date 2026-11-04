package fajnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/postingpreview"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Preview Posting on the fixed asset journals
func init() {
	functions.Register("table", gtables.FAJournalLineTableName, "PreviewPosting", functions.For(FAJnlPostPreview), functions.WithDialog())
}

// FAJnlPostPreview posts the line's journal batch as Post would (codeunit 5633), in a transaction that is rolled
// back, and opens Posting Preview on the entries it would write; nothing is saved — the lines
// stay, the number series do not move
func FAJnlPostPreview(rec *gtables.FAJournalLine) (fcodeunits.Result, error) {
	preview, err := postingpreview.Run(rec.GetDB(), rec.CompanyName(), rec.GetDBType(), fcodeunits.CurrentUserID(), func(src ftables.Source) error {
		line := gtables.NewFAJournalLine(src)
		line.Journal_template_name = rec.Journal_template_name
		line.Journal_batch_name = rec.Journal_batch_name
		_, err := NewFAJnlPostBatch(src).Run(line)
		return err
	})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return preview.Result()
}
