package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 672 Job Queue: Object Caption to Run — the caption of the job's codeunit or report in the
// user's language (the Object table's Object Caption; classic Object Caption to Run)
const jobQueueListPageID = 672

func init() {
	pages.OnAfterGetRecord(jobQueueListPageID, func(rec *gtables.JobQueue, page *pages.Control) {
		page.SetValue("object_caption_to_run", JobQueueObjectCaption(rec))
	})
}

// JobQueueObjectCaption: the Object table's caption of the job's object in the user's language
// ("" when it is not there)
func JobQueueObjectCaption(rec *gtables.JobQueue) string {
	if rec.Object_id_to_run == 0 {
		return ""
	}
	objectType := gtables.Object_Type.Codeunit
	if rec.Object_type_to_run == gtables.JobQueue_Object_type_to_run.Report {
		objectType = gtables.Object_Type.Report
	}
	object := gtables.NewObject(ftables.DBSource(rec.GetDB(), "", rec.GetDBType()))
	if !object.Get(map[string]interface{}{"type": objectType, "id": rec.Object_id_to_run}) {
		return ""
	}
	object.CalcFields("caption")
	return object.Caption.String()
}
