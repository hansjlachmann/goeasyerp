package projectjnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/noseries"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// maxPostingNoSeries is how many posting number series one journal may use (classic array)
const maxPostingNoSeries = 10

// ProjectJnlPostBatch is codeunit 203 Project Jnl.-Post Batch: it posts the lines of a project
// journal batch — every line checked, document numbers checked against the batch's number series and
// renumbered from the posting number series, every line posted — and deletes the posted lines.
// Open it on the posting's transaction; an error leaves everything to the caller's rollback.
//
// A recurring journal (recurring.go) posts the lines that are due and keeps them, each moved
// on by its Recurring Frequency.
type ProjectJnlPostBatch struct {
	src                 ftables.Source
	projectJnlCheckLine ProjectJnlCheckLine
	projectJnlPostLine  *ProjectJnlPostLine
	noSeriesMgt         *noseries.Manager
	postingNoSeriesMgt  map[types.Code]*noseries.Manager
}

// PostBatchResult is what the posting tells its caller: the Project Register it wrote (0: nothing
// posted) and the batch the journal is in afterwards (a numbered batch moves to the next name)
type PostBatchResult struct {
	RegisterNo       int
	JournalBatchName types.Code
}

// NewProjectJnlPostBatch opens the batch posting in the posting's database (its transaction)
func NewProjectJnlPostBatch(src ftables.Source) *ProjectJnlPostBatch {
	return &ProjectJnlPostBatch{src: src}
}

// Run posts the batch of a journal line (Code)
func (b *ProjectJnlPostBatch) Run(rec *gtables.ProjectJournalLine) (PostBatchResult, error) {
	projectJnlLine := gtables.NewProjectJournalLine(b.src)
	projectJnlLine.LockTable()
	projectJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	projectJnlLine.SetRange("journal_batch_name", rec.Journal_batch_name)
	result := PostBatchResult{JournalBatchName: rec.Journal_batch_name}

	projectJnlTemplate := gtables.NewProjectJournalTemplate(b.src)
	err := projectJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return result, err
	}
	projectJnlBatch := gtables.NewProjectJournalBatch(b.src)
	err = projectJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "name": rec.Journal_batch_name})
	if err != nil {
		return result, err
	}
	if system.StrLen(system.IncStr(projectJnlBatch.Name.String())) > projectJnlBatch.MaxStrLen("name") {
		return result, apperrors.JnlNameTooLong(gtables.ProjectJournalBatchTableName, "name", projectJnlBatch.MaxStrLen("name"))
	}

	if projectJnlTemplate.Recurring {
		tables.SetRecurringFilters(projectJnlLine, "posting_date")
	}
	if !projectJnlLine.FindFirst() {
		return result, nil
	}

	language := fcodeunits.CurrentLanguage()
	window := system.OpenWindow(i18n.GetInstance().Message("PROJECT_JNL_POST_WINDOW", language))
	defer window.Close()
	window.Update(1, rec.Journal_batch_name)

	// Check Lines
	lineCount := 0
	for found := projectJnlLine.FindSetBuffered(); found; found = projectJnlLine.Next() { // the checks read other tables: read all first
		lineCount = lineCount + 1
		window.Update(2, lineCount)
		projectJnlLine2 := gtables.NewProjectJournalLine(b.src)
		projectJnlLine2.Copy(projectJnlLine)
		if projectJnlLine2.No != "" {
			err = tables.CheckRecurringLine(projectJnlLine2, projectJnlTemplate.Recurring, projectJnlLine2.Recurring_method == gtables.ProjectJournalLine_Recurring_method.Variable, "quantity")
			if err != nil {
				return result, err
			}
		}
		err = b.projectJnlCheckLine.RunCheck(projectJnlLine2)
		if err != nil {
			return result, err
		}
	}
	noOfRecords := lineCount

	// Find next register no.
	projectLedgEntry := gtables.NewProjectLedgerEntry(b.src)
	projectLedgEntry.LockTable()
	projectReg := gtables.NewProjectRegister(b.src)
	projectReg.LockTable()
	var projectRegNo int
	if projectReg.FindLast() && projectReg.To_entry_no == 0 {
		projectRegNo = projectReg.No
	} else {
		projectRegNo = projectReg.No + 1
	}

	// Post lines
	b.projectJnlPostLine = NewProjectJnlPostLine(b.src)
	b.postingNoSeriesMgt = map[types.Code]*noseries.Manager{}
	lineCount = 0
	var lastDocNo, lastDocNo2, lastPostedDocNo types.Code
	for found := projectJnlLine.FindSetBuffered(); found; found = projectJnlLine.Next() {
		projectJnlLine3 := gtables.NewProjectJournalLine(b.src)
		projectJnlLine3.Copy(projectJnlLine)
		lineCount = lineCount + 1
		window.Update(3, lineCount)
		window.Update(4, system.Round(types.NewDecimalFromInt(int64(lineCount*10000/noOfRecords)), 1))
		if !tables.ProjectJnlLineEmptyLine(projectJnlLine3) &&
			projectJnlBatch.No_series != "" &&
			projectJnlLine3.Document_no != lastDocNo2 {
			if b.noSeriesMgt == nil {
				b.noSeriesMgt = noseries.New(b.src)
			}
			nextNo, err := b.noSeriesMgt.GetNextNo(projectJnlBatch.No_series, projectJnlLine3.Posting_date, false)
			if err != nil {
				return result, err
			}
			err = projectJnlLine3.TestField("document_no", nextNo)
			if err != nil {
				return result, err
			}
		}
		lastDocNo2 = projectJnlLine3.Document_no
		if projectJnlLine3.No != "" && projectJnlLine3.Recurring_method != 0 {
			projectJnlLine3.Document_no, projectJnlLine3.Description = tables.RecurringText(b.src, projectJnlLine3.Posting_date, projectJnlLine3.Document_no, projectJnlLine3.Description,
				projectJnlLine3.MaxStrLen("document_no"), projectJnlLine3.MaxStrLen("description"))
		}
		if projectJnlLine3.Posting_no_series == "" {
			projectJnlLine3.Posting_no_series = projectJnlBatch.No_series
		} else {
			if !tables.ProjectJnlLineEmptyLine(projectJnlLine3) {
				if projectJnlLine3.Document_no == lastDocNo {
					projectJnlLine3.Document_no = lastPostedDocNo
				} else {
					noSeriesMgt2, ok := b.postingNoSeriesMgt[projectJnlLine3.Posting_no_series]
					if !ok {
						if len(b.postingNoSeriesMgt) >= maxPostingNoSeries {
							return result, apperrors.JnlMaxPostingNoSeries(maxPostingNoSeries)
						}
						noSeriesMgt2 = noseries.New(b.src)
						b.postingNoSeriesMgt[projectJnlLine3.Posting_no_series] = noSeriesMgt2
					}
					lastDocNo = projectJnlLine3.Document_no
					projectJnlLine3.Document_no, err = noSeriesMgt2.GetNextNo(projectJnlLine3.Posting_no_series, projectJnlLine3.Posting_date, false)
					if err != nil {
						return result, err
					}
					lastPostedDocNo = projectJnlLine3.Document_no
				}
			}
		}
		err = b.projectJnlPostLine.Run(projectJnlLine3)
		if err != nil {
			return result, err
		}
	}

	// Copy register no. and current journal batch name to the project journal
	projectReg = gtables.NewProjectRegister(b.src)
	if !projectReg.FindLast() || projectReg.No != projectRegNo {
		projectRegNo = 0
	}
	result.RegisterNo = projectRegNo

	// Update/delete lines
	// A recurring journal keeps its lines, the next date ahead
	if projectRegNo != 0 && projectJnlTemplate.Recurring {
		err = b.updateRecurringLines(projectJnlLine)
		if err != nil {
			return result, err
		}
	}
	if projectRegNo != 0 && !projectJnlTemplate.Recurring {
		projectJnlLine2 := gtables.NewProjectJournalLine(b.src)
		projectJnlLine2.CopyFilters(projectJnlLine)
		projectJnlLine2.SetFilter("no", "<>''")
		_ = projectJnlLine2.FindLast() // Remember the last line
		projectJnlLine.DeleteAll()

		projectJnlLine3 := gtables.NewProjectJournalLine(b.src)
		projectJnlLine3.SetRange("journal_template_name", rec.Journal_template_name)
		projectJnlLine3.SetRange("journal_batch_name", rec.Journal_batch_name)
		if !projectJnlLine3.FindLast() {
			if system.IncStr(rec.Journal_batch_name.String()) != "" {
				err = projectJnlBatch.DeleteOrError(false)
				if err != nil {
					return result, err
				}
				projectJnlBatch.Name = types.Code(system.IncStr(rec.Journal_batch_name.String()))
				_ = projectJnlBatch.Insert(false) // IF INSERT THEN: an existing batch of that name is used
				result.JournalBatchName = projectJnlBatch.Name
			}
		}

		projectJnlLine3.SetRange("journal_batch_name", result.JournalBatchName)
		if projectJnlBatch.No_series == "" && !projectJnlLine3.FindLast() {
			projectJnlLine3.Init()
			projectJnlLine3.Journal_template_name = rec.Journal_template_name
			projectJnlLine3.Journal_batch_name = result.JournalBatchName
			projectJnlLine3.Line_no = 10000
			err = projectJnlLine3.InsertOrError(false)
			if err != nil {
				return result, err
			}
			err = tables.SetUpNewProjectJnlLine(projectJnlLine3, projectJnlLine2)
			if err != nil {
				return result, err
			}
			err = projectJnlLine3.ModifyOrError(false)
			if err != nil {
				return result, err
			}
		}
	}
	if b.noSeriesMgt != nil {
		err = b.noSeriesMgt.SaveNoSeries()
		if err != nil {
			return result, err
		}
	}
	for _, noSeriesMgt2 := range b.postingNoSeriesMgt {
		err = noSeriesMgt2.SaveNoSeries()
		if err != nil {
			return result, err
		}
	}
	return result, nil
}
