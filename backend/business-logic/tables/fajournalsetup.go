package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/noseries"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// fAJournalSetupLogic is the business logic of table 5605 FA Journal Setup: per depreciation
// book and user (a blank user: everyone else), the FA journal and the general journal the batch
// functions (Calculate Depreciation …) fill.
type fAJournalSetupLogic struct{}

func init() { gtables.RegisterFAJournalSetupLogic(fAJournalSetupLogic{}) }

// OnValidate_Fa_jnl_template_name: another template, no batch
func (fAJournalSetupLogic) OnValidate_Fa_jnl_template_name(rec *gtables.FAJournalSetup) error {
	rec.Fa_jnl_batch_name = ""
	return nil
}

// OnValidate_Gen_jnl_template_name: another template, no batch
func (fAJournalSetupLogic) OnValidate_Gen_jnl_template_name(rec *gtables.FAJournalSetup) error {
	rec.Gen_jnl_batch_name = ""
	return nil
}

// OnValidate_Insurance_jnl_template_name: another template, no batch
func (fAJournalSetupLogic) OnValidate_Insurance_jnl_template_name(rec *gtables.FAJournalSetup) error {
	rec.Insurance_jnl_batch_name = ""
	return nil
}

// getFAJnlSetup: the depreciation book's FA Journal Setup of the user, else of everyone (blank
// user)
func getFAJnlSetup(src ftables.Source, deprBookCode types.Code) (*gtables.FAJournalSetup, error) {
	faJnlSetup := gtables.NewFAJournalSetup(src)
	if faJnlSetup.Get(map[string]interface{}{"depreciation_book_code": deprBookCode, "user_id": types.Code(system.UserId())}) {
		return faJnlSetup, nil
	}
	err := faJnlSetup.GetOrError(map[string]interface{}{"depreciation_book_code": deprBookCode, "user_id": types.Code("")})
	return faJnlSetup, err
}

// FAJnlName (FAJnlName): the FA journal batch a batch function of the book fills (FA Journal
// Setup) and its last line no. (SetFAJnlRange)
func FAJnlName(src ftables.Source, deprBookCode types.Code) (templateName, batchName types.Code, nextLineNo int, err error) {
	faJnlSetup, err := getFAJnlSetup(src, deprBookCode)
	if err != nil {
		return
	}
	if err = faJnlSetup.TestField("fa_jnl_template_name"); err != nil {
		return
	}
	if err = faJnlSetup.TestField("fa_jnl_batch_name"); err != nil {
		return
	}
	templateName, batchName = faJnlSetup.Fa_jnl_template_name, faJnlSetup.Fa_jnl_batch_name
	if err = gtables.NewFAJournalTemplate(src).GetOrError(templateName); err != nil {
		return
	}
	if err = gtables.NewFAJournalBatch(src).GetOrError(map[string]interface{}{"journal_template_name": templateName, "name": batchName}); err != nil {
		return
	}
	faJnlLine := gtables.NewFAJournalLine(src)
	faJnlLine.SetRange("journal_template_name", templateName)
	faJnlLine.SetRange("journal_batch_name", batchName)
	if faJnlLine.FindLast() {
		nextLineNo = faJnlLine.Line_no
	}
	return
}

// GenJnlName (GenJnlName): the general journal batch a batch function of the book fills (FA
// Journal Setup) and its last line no. (SetGenJnlRange)
func GenJnlName(src ftables.Source, deprBookCode types.Code) (templateName, batchName types.Code, nextLineNo int, err error) {
	faJnlSetup, err := getFAJnlSetup(src, deprBookCode)
	if err != nil {
		return
	}
	if err = faJnlSetup.TestField("gen_jnl_template_name"); err != nil {
		return
	}
	if err = faJnlSetup.TestField("gen_jnl_batch_name"); err != nil {
		return
	}
	templateName, batchName = faJnlSetup.Gen_jnl_template_name, faJnlSetup.Gen_jnl_batch_name
	if err = gtables.NewGenJournalTemplate(src).GetOrError(templateName); err != nil {
		return
	}
	if err = gtables.NewGenJournalBatch(src).GetOrError(GenJnlBatchKey(templateName, batchName)); err != nil {
		return
	}
	genJnlLine := gtables.NewGenJournalLine(src)
	genJnlLine.SetRange("journal_template_name", templateName)
	genJnlLine.SetRange("journal_batch_name", batchName)
	if genJnlLine.FindLast() {
		nextLineNo = genJnlLine.Line_no
	}
	return
}

// GetFAJnlDocumentNo / GetGenJnlDocumentNo (GetFAJnlDocumentNo, GetGenJnlDocumentNo): the next
// number of the batch's No. Series when the batch has no lines yet (blank otherwise)
func GetFAJnlDocumentNo(src ftables.Source, templateName, batchName types.Code, postingDate types.Date) (types.Code, error) {
	faJnlBatch := gtables.NewFAJournalBatch(src)
	err := faJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": templateName, "name": batchName})
	if err != nil || faJnlBatch.No_series == "" {
		return "", err
	}
	faJnlLine := gtables.NewFAJournalLine(src)
	faJnlLine.SetRange("journal_template_name", templateName)
	faJnlLine.SetRange("journal_batch_name", batchName)
	if !faJnlLine.IsEmpty() {
		return "", nil
	}
	return noseries.New(src).GetNextNo(faJnlBatch.No_series, postingDate, false)
}

func GetGenJnlDocumentNo(src ftables.Source, templateName, batchName types.Code, postingDate types.Date) (types.Code, error) {
	genJnlBatch := gtables.NewGenJournalBatch(src)
	err := genJnlBatch.GetOrError(GenJnlBatchKey(templateName, batchName))
	if err != nil || genJnlBatch.No_series == "" {
		return "", err
	}
	genJnlLine := gtables.NewGenJournalLine(src)
	genJnlLine.SetRange("journal_template_name", templateName)
	genJnlLine.SetRange("journal_batch_name", batchName)
	if !genJnlLine.IsEmpty() {
		return "", nil
	}
	return noseries.New(src).GetNextNo(genJnlBatch.No_series, postingDate, false)
}

// GetFANoSeries / GetGenNoSeries (GetFANoSeries, GetGenNoSeries): the batch's Posting No. Series
// when it differs from its No. Series
func GetFANoSeries(src ftables.Source, templateName, batchName types.Code) types.Code {
	faJnlBatch := gtables.NewFAJournalBatch(src)
	if faJnlBatch.Get(map[string]interface{}{"journal_template_name": templateName, "name": batchName}) && faJnlBatch.No_series != faJnlBatch.Posting_no_series {
		return faJnlBatch.Posting_no_series
	}
	return ""
}

func GetGenNoSeries(src ftables.Source, templateName, batchName types.Code) types.Code {
	genJnlBatch := gtables.NewGenJournalBatch(src)
	if genJnlBatch.Get(GenJnlBatchKey(templateName, batchName)) && genJnlBatch.No_series != genJnlBatch.Posting_no_series {
		return genJnlBatch.Posting_no_series
	}
	return ""
}

// SetGenJnlTrailCodes (SetGenJnlTrailCodes): a general journal line of a batch function takes
// the template's source code and the batch's reason code; the batch has no balancing account
func SetGenJnlTrailCodes(genJnlLine *gtables.GenJournalLine) error {
	genJnlTemplate := gtables.NewGenJournalTemplate(genJnlLine)
	err := genJnlTemplate.GetOrError(genJnlLine.Journal_template_name)
	if err != nil {
		return err
	}
	genJnlBatch := gtables.NewGenJournalBatch(genJnlLine)
	err = genJnlBatch.GetOrError(GenJnlBatchKey(genJnlLine.Journal_template_name, genJnlLine.Journal_batch_name))
	if err != nil {
		return err
	}
	genJnlLine.Source_code = genJnlTemplate.Source_code
	genJnlLine.Reason_code = genJnlBatch.Reason_code
	err = genJnlBatch.TestField("bal_account_type", gtables.GenJournalBatch_Bal_account_type.GLAccount)
	if err != nil {
		return err
	}
	return genJnlBatch.TestField("bal_account_no", types.Code(""))
}

// InsuranceJnlName (InsuranceJnlName): the insurance journal batch of the book's FA Journal Setup
// and its last line no. (SetInsuranceJnlRange)
func InsuranceJnlName(src ftables.Source, deprBookCode types.Code) (templateName, batchName types.Code, nextLineNo int, err error) {
	faJnlSetup, err := getFAJnlSetup(src, deprBookCode)
	if err != nil {
		return
	}
	if err = faJnlSetup.TestField("insurance_jnl_template_name"); err != nil {
		return
	}
	if err = faJnlSetup.TestField("insurance_jnl_batch_name"); err != nil {
		return
	}
	templateName, batchName = faJnlSetup.Insurance_jnl_template_name, faJnlSetup.Insurance_jnl_batch_name
	if err = gtables.NewInsuranceJournalTemplate(src).GetOrError(templateName); err != nil {
		return
	}
	if err = gtables.NewInsuranceJournalBatch(src).GetOrError(map[string]interface{}{"journal_template_name": templateName, "name": batchName}); err != nil {
		return
	}
	insuranceJnlLine := gtables.NewInsuranceJournalLine(src)
	insuranceJnlLine.SetRange("journal_template_name", templateName)
	insuranceJnlLine.SetRange("journal_batch_name", batchName)
	if insuranceJnlLine.FindLast() {
		nextLineNo = insuranceJnlLine.Line_no
	}
	return
}

// GetInsuranceNoSeries (GetInsuranceNoSeries): the batch's Posting No. Series when it differs
// from its No. Series
func GetInsuranceNoSeries(src ftables.Source, templateName, batchName types.Code) types.Code {
	insuranceJnlBatch := gtables.NewInsuranceJournalBatch(src)
	if insuranceJnlBatch.Get(map[string]interface{}{"journal_template_name": templateName, "name": batchName}) && insuranceJnlBatch.No_series != insuranceJnlBatch.Posting_no_series {
		return insuranceJnlBatch.Posting_no_series
	}
	return ""
}
