package insurancejnl

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/noseries"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// maxPostingNoSeries is how many posting number series one journal may use (classic array)
const maxPostingNoSeries = 10

// PostBatchResult is what the posting tells its caller: the Insurance Register it wrote (0:
// nothing posted) and the batch the journal is in afterwards (a numbered batch moves to the next
// name)
type PostBatchResult struct {
	RegisterNo       int
	JournalBatchName types.Code
}

// PostBatch is codeunit 5653 Insurance Jnl.-Post Batch: every line of the batch checked, document
// numbers checked against the batch's number series and renumbered from the posting number
// series, every line posted, the posted lines deleted. Run it on the posting's transaction; an
// error leaves everything to the caller's rollback.
func PostBatch(src ftables.Source, rec *gtables.InsuranceJournalLine) (PostBatchResult, error) {
	insuranceJnlLine := gtables.NewInsuranceJournalLine(src)
	insuranceJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	insuranceJnlLine.SetRange("journal_batch_name", rec.Journal_batch_name)
	insuranceJnlLine.LockTable()
	result := PostBatchResult{JournalBatchName: rec.Journal_batch_name}

	insuranceJnlTemplate := gtables.NewInsuranceJournalTemplate(src)
	err := insuranceJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return result, err
	}
	insuranceJnlBatch := gtables.NewInsuranceJournalBatch(src)
	err = insuranceJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "name": rec.Journal_batch_name})
	if err != nil {
		return result, err
	}
	if system.StrLen(system.IncStr(insuranceJnlBatch.Name.String())) > insuranceJnlBatch.MaxStrLen("name") {
		return result, apperrors.JnlNameTooLong(gtables.InsuranceJournalBatchTableName, "name", insuranceJnlBatch.MaxStrLen("name"))
	}
	if !insuranceJnlLine.FindFirst() {
		return result, nil
	}
	window := system.OpenWindow(i18n.GetInstance().Message("FA_JNL_POST_WINDOW", fcodeunits.CurrentLanguage()))
	defer window.Close()
	window.Update(1, rec.Journal_batch_name)

	// Check lines
	var insuranceJnlCheckLine InsuranceJnlCheckLine
	lineCount := 0
	for found := insuranceJnlLine.FindSet(); found; found = insuranceJnlLine.Next() {
		lineCount = lineCount + 1
		window.Update(2, lineCount)
		err = insuranceJnlCheckLine.Run(insuranceJnlLine)
		if err != nil {
			return result, err
		}
	}
	noOfRecords := lineCount
	gtables.NewInsCoverageLedgerEntry(src).LockTable()
	insuranceReg := gtables.NewInsuranceRegister(src)
	insuranceReg.LockTable()
	insuranceRegNo := 1
	if insuranceReg.FindLast() {
		insuranceRegNo = insuranceReg.No + 1
	}

	// Post lines
	insuranceJnlPostLine := NewInsuranceJnlPostLine(src)
	var noSeriesMgt *noseries.Manager
	noSeriesMgt2 := map[types.Code]*noseries.Manager{}
	lineCount = 0
	var lastDocNo, lastDocNo2, lastPostedDocNo types.Code
	for found := insuranceJnlLine.FindSetBuffered(); found; found = insuranceJnlLine.Next() {
		insuranceJnlLine3 := gtables.NewInsuranceJournalLine(src)
		insuranceJnlLine3.Copy(insuranceJnlLine)
		lineCount = lineCount + 1
		window.Update(3, lineCount)
		window.Update(4, system.Round(types.NewDecimalFromInt(int64(lineCount*10000/noOfRecords)), 1))
		if !(insuranceJnlLine3.Insurance_no == "") &&
			insuranceJnlBatch.No_series != "" &&
			insuranceJnlLine3.Document_no != lastDocNo2 {
			if noSeriesMgt == nil {
				noSeriesMgt = noseries.New(src)
			}
			nextNo, err := noSeriesMgt.GetNextNo(insuranceJnlBatch.No_series, insuranceJnlLine3.Posting_date, false)
			if err != nil {
				return result, err
			}
			err = insuranceJnlLine3.TestField("document_no", nextNo)
			if err != nil {
				return result, err
			}
		}
		lastDocNo2 = insuranceJnlLine3.Document_no
		if insuranceJnlLine3.Posting_no_series == "" {
			insuranceJnlLine3.Posting_no_series = insuranceJnlBatch.No_series
		} else {
			if !(insuranceJnlLine3.Insurance_no == "") {
				if insuranceJnlLine3.Document_no == lastDocNo {
					insuranceJnlLine3.Document_no = lastPostedDocNo
				} else {
					postingNoSeriesMgt, ok := noSeriesMgt2[insuranceJnlLine3.Posting_no_series]
					if !ok {
						if len(noSeriesMgt2) >= maxPostingNoSeries {
							return result, apperrors.JnlMaxPostingNoSeries(maxPostingNoSeries)
						}
						postingNoSeriesMgt = noseries.New(src)
						noSeriesMgt2[insuranceJnlLine3.Posting_no_series] = postingNoSeriesMgt
					}
					lastDocNo = insuranceJnlLine3.Document_no
					insuranceJnlLine3.Document_no, err = postingNoSeriesMgt.GetNextNo(insuranceJnlLine3.Posting_no_series, insuranceJnlLine3.Posting_date, false)
					if err != nil {
						return result, err
					}
					lastPostedDocNo = insuranceJnlLine3.Document_no
				}
			}
		}
		err = insuranceJnlPostLine.RunWithOutCheck(insuranceJnlLine3)
		if err != nil {
			return result, err
		}
	}
	insuranceReg = gtables.NewInsuranceRegister(src)
	if !insuranceReg.FindLast() || insuranceReg.No != insuranceRegNo {
		insuranceRegNo = 0
	}
	result.RegisterNo = insuranceRegNo

	// Update/delete lines
	if insuranceRegNo != 0 {
		insuranceJnlLine2 := gtables.NewInsuranceJournalLine(src)
		insuranceJnlLine2.CopyFilters(insuranceJnlLine)
		insuranceJnlLine2.SetFilter("insurance_no", "<>''")
		_ = insuranceJnlLine2.FindLast() // Remember the last line
		insuranceJnlLine.DeleteAll()
		insuranceJnlLine3 := gtables.NewInsuranceJournalLine(src)
		insuranceJnlLine3.SetRange("journal_template_name", rec.Journal_template_name)
		insuranceJnlLine3.SetRange("journal_batch_name", rec.Journal_batch_name)
		if !insuranceJnlLine3.FindLast() {
			if system.IncStr(rec.Journal_batch_name.String()) != "" {
				err = insuranceJnlBatch.DeleteOrError(false)
				if err != nil {
					return result, err
				}
				insuranceJnlBatch.Name = types.Code(system.IncStr(rec.Journal_batch_name.String()))
				_ = insuranceJnlBatch.Insert(false) // IF INSERT THEN: an existing batch of that name is used
				result.JournalBatchName = insuranceJnlBatch.Name
			}
		}
		insuranceJnlLine3.SetRange("journal_batch_name", result.JournalBatchName)
		if insuranceJnlBatch.No_series == "" && !insuranceJnlLine3.FindLast() {
			insuranceJnlLine3.Init()
			insuranceJnlLine3.Journal_template_name = rec.Journal_template_name
			insuranceJnlLine3.Journal_batch_name = result.JournalBatchName
			insuranceJnlLine3.Line_no = 10000
			err = insuranceJnlLine3.InsertOrError(false)
			if err != nil {
				return result, err
			}
			err = tables.SetUpNewInsuranceJnlLine(insuranceJnlLine3, insuranceJnlLine2)
			if err != nil {
				return result, err
			}
			err = insuranceJnlLine3.ModifyOrError(false)
			if err != nil {
				return result, err
			}
		}
	}
	if noSeriesMgt != nil {
		err = noSeriesMgt.SaveNoSeries()
		if err != nil {
			return result, err
		}
	}
	for _, postingNoSeriesMgt := range noSeriesMgt2 {
		err = postingNoSeriesMgt.SaveNoSeries()
		if err != nil {
			return result, err
		}
	}
	return result, nil
}
