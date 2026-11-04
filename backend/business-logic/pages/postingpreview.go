package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/postingpreview"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 115 Posting Preview: the page parameter preview is a previewed posting; its lines (a
// temporary Document Entry, like Navigate's) are the tables the posting wrote entries in and
// how many; Show opens a line's entries page on the preview
const postingPreviewPageID = 115

func init() {
	pages.OnFindRecords(postingPreviewPageID, func(rec *gtables.DocumentEntry, page *pages.Open) error {
		preview, ok := postingpreview.Get(page.Param("preview"), fcodeunits.CurrentUserID(), rec.CompanyName())
		if !ok {
			return apperrors.PreviewExpired()
		}
		lang := fcodeunits.CurrentLanguage()
		for i, entries := range preview.Tables {
			rec.Init()
			rec.Entry_no = i + 1
			rec.Table_id = entries.Table.ID
			rec.Page_id = entries.Table.PageID
			rec.Table_name = types.NewText(tableCaption(entries.Table.Name, lang))
			rec.No_of_records = len(entries.Rows)
			rec.Preview_id = types.NewText(preview.ID)
			err := rec.InsertOrError(false)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// tableCaption: a table's caption in the language
func tableCaption(tableName, lang string) string {
	factory, ok := gtables.GetTableFactory(tableName)
	if !ok {
		return tableName
	}
	rec, ok := factory().(interface {
		TableCaption(language ...string) string
	})
	if !ok {
		return tableName
	}
	return rec.TableCaption(lang)
}
