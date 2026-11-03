package tables

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/noseries"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// insuranceJournalLineLogic is the business logic of table 5635 Insurance Journal Line: a fixed
// asset's coverage by an insurance policy (codeunit 5652 posts it as an Ins. Coverage Ledger
// Entry).
type insuranceJournalLineLogic struct{}

func init() { gtables.RegisterInsuranceJournalLineLogic(insuranceJournalLineLogic{}) }

// OnInsert: the template's source code and the batch's reason code
func (insuranceJournalLineLogic) OnInsert(rec *gtables.InsuranceJournalLine) error {
	rec.LockTable()
	insuranceJnlTemplate := gtables.NewInsuranceJournalTemplate(rec)
	err := insuranceJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return err
	}
	rec.Source_code = insuranceJnlTemplate.Source_code
	insuranceJnlBatch := gtables.NewInsuranceJournalBatch(rec)
	err = insuranceJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "name": rec.Journal_batch_name})
	if err != nil {
		return err
	}
	rec.Reason_code = insuranceJnlBatch.Reason_code
	return nil
}

// OnValidate_Insurance_no: an insurance that is not blocked; its description, dimension 1 and
// dimension 2
func (insuranceJournalLineLogic) OnValidate_Insurance_no(rec *gtables.InsuranceJournalLine) error {
	if rec.Insurance_no == "" {
		return nil
	}
	insurance := gtables.NewInsurance(rec)
	err := insurance.GetOrError(rec.Insurance_no)
	if err != nil {
		return err
	}
	err = insurance.TestField("blocked", false)
	if err != nil {
		return err
	}
	rec.Description = insurance.Description
	rec.Dimension_1_code = insurance.Dimension_1_code
	rec.Dimension_2_code = insurance.Dimension_2_code
	return nil
}

// OnValidate_Fa_no: the asset's description; not blocked or inactive
func (insuranceJournalLineLogic) OnValidate_Fa_no(rec *gtables.InsuranceJournalLine) error {
	if rec.Fa_no == "" {
		rec.Fa_description = ""
		return nil
	}
	fa := gtables.NewFixedAsset(rec)
	err := fa.GetOrError(rec.Fa_no)
	if err != nil {
		return err
	}
	rec.Fa_description = fa.Description
	err = fa.TestField("blocked", false)
	if err != nil {
		return err
	}
	return fa.TestField("inactive", false)
}

// SetUpNewInsuranceJnlLine (SetUpNewLine): a new line takes the posting date and document no. of
// the last line — the first line of a batch the work date and the next number of the batch's
// series —, the template's source code and the batch's reason code and posting no. series
func SetUpNewInsuranceJnlLine(rec, lastInsuranceJnlLine *gtables.InsuranceJournalLine) error {
	insuranceJnlTemplate := gtables.NewInsuranceJournalTemplate(rec)
	err := insuranceJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return err
	}
	insuranceJnlBatch := gtables.NewInsuranceJournalBatch(rec)
	err = insuranceJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "name": rec.Journal_batch_name})
	if err != nil {
		return err
	}
	insuranceJnlLine := gtables.NewInsuranceJournalLine(rec)
	insuranceJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	insuranceJnlLine.SetRange("journal_batch_name", rec.Journal_batch_name)
	if insuranceJnlLine.FindFirst() {
		rec.Posting_date = lastInsuranceJnlLine.Posting_date
		rec.Document_no = lastInsuranceJnlLine.Document_no
	} else {
		rec.Posting_date = system.WorkDate()
		if insuranceJnlBatch.No_series != "" {
			noSeriesMgt := noseries.New(rec)
			rec.Document_no = noSeriesMgt.TryGetNextNo(insuranceJnlBatch.No_series, rec.Posting_date)
		}
	}
	rec.Source_code = insuranceJnlTemplate.Source_code
	rec.Reason_code = insuranceJnlBatch.Reason_code
	rec.Posting_no_series = insuranceJnlBatch.Posting_no_series
	return nil
}
