// Package fareclass is the FA reclassification journal: codeunits 5641 FA Reclass. Check Line,
// 5642 FA Reclass. Transfer Line, 5643 FA Reclass. Transfer Batch and 5646 FA Reclass. Jnl.
// Management. A reclassification posts nothing itself: it moves a share of one asset's amounts
// in a depreciation book to another asset as FA or general journal lines (FA Reclassification
// Entry), which the user posts.
package fareclass

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/fajnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The FA posting types a reclassification moves (classic arrays TransferType/Amounts, indexed by
// posting type + 1: 1..6 and 9)
var (
	acquisition  = gtables.FAJournalLine_Fa_posting_type.AcquisitionCost
	depr         = gtables.FAJournalLine_Fa_posting_type.Depreciation
	writeDown    = gtables.FAJournalLine_Fa_posting_type.WriteDown
	appr         = gtables.FAJournalLine_Fa_posting_type.Appreciation
	custom1      = gtables.FAJournalLine_Fa_posting_type.Custom1
	custom2      = gtables.FAJournalLine_Fa_posting_type.Custom2
	salvageValue = gtables.FAJournalLine_Fa_posting_type.SalvageValue
)

// FAReclassTransferLine is codeunit 5642 FA Reclass. Transfer Line: one instance per batch (the
// journals' next line numbers and document numbers carry over from line to line)
type FAReclassTransferLine struct {
	oldFA, newFA                      *gtables.FixedAsset
	faDeprBook, faDeprBook2           *gtables.FADepreciationBook
	transferType                      [9]bool
	amounts                           [9]types.Decimal
	transferToGenJnl                  bool
	templateName, batchName           types.Code
	faJnlNextLineNo, genJnlNextLineNo int
	genJnlUsedOnce, faJnlUsedOnce     bool
	faJnlDocumentNo, genJnlDocumentNo types.Code
}

// FAReclassLine (FAReclassLine): the old asset's amounts of the chosen types (its share) out —
// salvage value first, acquisition cost last — and into the new asset — acquisition cost first;
// done tells whether a line was made
func (t *FAReclassTransferLine) FAReclassLine(faReclassJnlLine *gtables.FAReclassJournalLine, done *bool) error {
	if faReclassJnlLine.Fa_no == "" && faReclassJnlLine.New_fa_no == "" {
		return nil
	}
	t.oldFA = gtables.NewFixedAsset(faReclassJnlLine)
	err := t.oldFA.GetOrError(faReclassJnlLine.Fa_no)
	if err != nil {
		return err
	}
	t.newFA = gtables.NewFixedAsset(faReclassJnlLine)
	err = t.newFA.GetOrError(faReclassJnlLine.New_fa_no)
	if err != nil {
		return err
	}
	t.faDeprBook = gtables.NewFADepreciationBook(faReclassJnlLine)
	err = t.faDeprBook.GetOrError(map[string]interface{}{"fa_no": faReclassJnlLine.Fa_no, "depreciation_book_code": faReclassJnlLine.Depreciation_book_code})
	if err != nil {
		return err
	}
	t.faDeprBook2 = gtables.NewFADepreciationBook(faReclassJnlLine)
	err = t.faDeprBook2.GetOrError(map[string]interface{}{"fa_no": faReclassJnlLine.New_fa_no, "depreciation_book_code": faReclassJnlLine.Depreciation_book_code})
	if err != nil {
		return err
	}
	for _, fa := range []*gtables.FixedAsset{t.oldFA, t.newFA} {
		err = fa.TestField("blocked", false)
		if err != nil {
			return err
		}
	}
	for _, fa := range []*gtables.FixedAsset{t.oldFA, t.newFA} {
		err = fa.TestField("inactive", false)
		if err != nil {
			return err
		}
	}

	if t.oldFA.Budgeted_asset && !t.newFA.Budgeted_asset {
		return apperrors.FAReclassBudgeted(gtables.FAReclassJournalLineTableName, "fa_no", "new_fa_no")
	}
	if t.newFA.Budgeted_asset && !t.oldFA.Budgeted_asset {
		return apperrors.FAReclassBudgeted(gtables.FAReclassJournalLineTableName, "new_fa_no", "fa_no")
	}
	if faReclassJnlLine.Fa_no == faReclassJnlLine.New_fa_no {
		return apperrors.FAReclassSameAsset(gtables.FAReclassJournalLineTableName)
	}
	if !t.faDeprBook.Disposal_date.IsZero() {
		return apperrors.FAReclassDisposed(t.oldFA.No.String(), faReclassJnlLine.Depreciation_book_code.String())
	}
	if !t.faDeprBook2.Disposal_date.IsZero() {
		return apperrors.FAReclassDisposed(t.newFA.No.String(), faReclassJnlLine.Depreciation_book_code.String())
	}

	t.setFAReclassType(faReclassJnlLine)
	err = t.calcAmounts(faReclassJnlLine)
	if err != nil {
		return err
	}
	for oldNewFA := 0; oldNewFA <= 1; oldNewFA++ {
		for _, j := range []int{1, 2, 3, 4, 5, 6, 9} {
			faPostingType, sign, faNo := t.convert(oldNewFA, j)
			i := int(faPostingType) + 1
			t.templateName = ""
			t.batchName = ""
			if t.transferType[i-1] && !t.amounts[i-1].IsZero() {
				err = t.jnlName(faReclassJnlLine, faReclassJnlLine.Depreciation_book_code, t.oldFA.Budgeted_asset, faPostingType)
				if err != nil {
					return err
				}
				amount := t.amounts[i-1].Mul(types.NewDecimalFromInt(int64(sign)))
				if t.transferToGenJnl {
					err = t.insertGenJnlLine(faReclassJnlLine, faPostingType, faNo, amount, faReclassJnlLine.Insert_bal_account)
				} else {
					err = t.insertFAJnlLine(faReclassJnlLine, faPostingType, faNo, amount)
				}
				if err != nil {
					return err
				}
				*done = true
			}
		}
	}
	return nil
}

// calcAmounts (CalcAmounts): the old asset's amounts in the book, its share — Reclassify Acq.
// Cost Amount (as a % of the acquisition cost) or Reclassify Acq. Cost %
func (t *FAReclassTransferLine) calcAmounts(faReclassJnlLine *gtables.FAReclassJournalLine) error {
	faDeprBook := t.faDeprBook
	faDeprBook.CalcFields("acquisition_cost", "depreciation", "write_down", "appreciation", "custom_1", "custom_2", "salvage_value")
	t.amounts[0] = faDeprBook.Acquisition_cost
	t.amounts[1] = faDeprBook.Depreciation
	t.amounts[2] = faDeprBook.Write_down
	t.amounts[3] = faDeprBook.Appreciation
	t.amounts[4] = faDeprBook.Custom_1
	t.amounts[5] = faDeprBook.Custom_2
	t.amounts[8] = faDeprBook.Salvage_value
	if t.amounts[0].IsZero() {
		return apperrors.FAReclassNoAcqCost(t.oldFA.No.String(), faReclassJnlLine.Depreciation_book_code.String())
	}

	if !faReclassJnlLine.Reclassify_acq_cost_amount.IsZero() {
		if faReclassJnlLine.Reclassify_acq_cost_amount.GreaterThan(t.amounts[0]) {
			return apperrors.FAReclassAmountTooLarge(gtables.FAReclassJournalLineTableName, t.oldFA.No.String(), faReclassJnlLine.Depreciation_book_code.String())
		}
		faReclassJnlLine.Reclassify_acq_cost_percent = faReclassJnlLine.Reclassify_acq_cost_amount.Div(t.amounts[0]).Mul(types.NewDecimalFromInt(100))
	}
	hundred := types.NewDecimalFromInt(100)
	if !faReclassJnlLine.Reclassify_acq_cost_amount.IsZero() {
		t.amounts[0] = faReclassJnlLine.Reclassify_acq_cost_amount
	} else {
		t.amounts[0] = system.Round(t.amounts[0].Mul(faReclassJnlLine.Reclassify_acq_cost_percent).Div(hundred))
	}
	for i := 2; i <= 9; i++ {
		t.amounts[i-1] = system.Round(t.amounts[i-1].Mul(faReclassJnlLine.Reclassify_acq_cost_percent).Div(hundred))
	}
	return nil
}

// setFAReclassType (SetFAReclassType): the posting types the line reclassifies
func (t *FAReclassTransferLine) setFAReclassType(faReclassJnlLine *gtables.FAReclassJournalLine) {
	t.transferType[0] = faReclassJnlLine.Reclassify_acquisition_cost
	t.transferType[1] = faReclassJnlLine.Reclassify_depreciation
	t.transferType[2] = faReclassJnlLine.Reclassify_write_down
	t.transferType[3] = faReclassJnlLine.Reclassify_appreciation
	t.transferType[4] = faReclassJnlLine.Reclassify_custom_1
	t.transferType[5] = faReclassJnlLine.Reclassify_custom_2
	t.transferType[8] = faReclassJnlLine.Reclassify_salvage_value
}

// jnlName (codeunit 5639 FA Get Journal JnlName, SetJnlRange): the general journal of the book's
// FA Journal Setup when the book posts the type to the G/L (not for budgeted assets), else its FA
// journal; the journal's last line no. the first time it is used
func (t *FAReclassTransferLine) jnlName(faReclassJnlLine *gtables.FAReclassJournalLine, deprBookCode types.Code, budgetedAsset bool, faPostingType gtables.FAJournalLineFa_posting_type) error {
	deprBook := gtables.NewDepreciationBook(faReclassJnlLine)
	err := deprBook.GetOrError(deprBookCode)
	if err != nil {
		return err
	}
	t.transferToGenJnl = !budgetedAsset && tables.DeprBookGLIntegration(deprBook, faPostingType)
	var nextLineNo int
	if t.transferToGenJnl {
		t.templateName, t.batchName, nextLineNo, err = tables.GenJnlName(faReclassJnlLine, deprBookCode)
	} else {
		t.templateName, t.batchName, nextLineNo, err = tables.FAJnlName(faReclassJnlLine, deprBookCode)
	}
	if err != nil {
		return err
	}
	if t.faJnlNextLineNo == 0 && !t.transferToGenJnl {
		gtables.NewFAJournalLine(faReclassJnlLine).LockTable()
		t.faJnlNextLineNo = nextLineNo
	}
	if t.genJnlNextLineNo == 0 && t.transferToGenJnl {
		gtables.NewGenJournalLine(faReclassJnlLine).LockTable()
		t.genJnlNextLineNo = nextLineNo
	}
	return nil
}

// convert (Convert): the posting type, sign and asset of step j — the old asset's amounts go out
// (sign -1) salvage value first, acquisition cost last; the new asset's come in (sign 1)
// acquisition cost first, depreciation last
func (t *FAReclassTransferLine) convert(oldNewFA, j int) (faPostingType gtables.FAJournalLineFa_posting_type, sign int, faNo types.Code) {
	if oldNewFA == 0 {
		sign = -1
		faNo = t.oldFA.No
	} else {
		sign = 1
		faNo = t.newFA.No
	}
	if oldNewFA == 0 {
		switch j {
		case 1:
			faPostingType = salvageValue
		case 2:
			faPostingType = depr
		case 3:
			faPostingType = writeDown
		case 4:
			faPostingType = custom1
		case 5:
			faPostingType = custom2
		case 6:
			faPostingType = appr
		case 9:
			faPostingType = acquisition
		}
	}
	if oldNewFA == 1 {
		switch j {
		case 1:
			faPostingType = acquisition
		case 2:
			faPostingType = salvageValue
		case 3:
			faPostingType = appr
		case 4:
			faPostingType = writeDown
		case 5:
			faPostingType = custom1
		case 6:
			faPostingType = custom2
		case 9:
			faPostingType = depr
		}
	}
	return faPostingType, sign, faNo
}

// insertFAJnlLine (InsertFAJnlLine): an FA journal line of the reclassification
func (t *FAReclassTransferLine) insertFAJnlLine(faReclassJnlLine *gtables.FAReclassJournalLine, faPostingType gtables.FAJournalLineFa_posting_type, faNo types.Code, entryAmount types.Decimal) error {
	var err error
	if !t.faJnlUsedOnce {
		t.faJnlUsedOnce = true
		t.faJnlDocumentNo, err = tables.GetFAJnlDocumentNo(faReclassJnlLine, t.templateName, t.batchName, faReclassJnlLine.Fa_posting_date)
		if err != nil {
			return err
		}
	}
	faJnlLine := gtables.NewFAJournalLine(faReclassJnlLine)
	faJnlLine.Init()
	faJnlLine.Journal_template_name = t.templateName
	faJnlLine.Journal_batch_name = t.batchName
	faJnlLine.Fa_posting_type = faPostingType
	err = faJnlLine.Validate("fa_no", faNo)
	if err != nil {
		return err
	}
	faJnlLine.Fa_posting_date = faReclassJnlLine.Fa_posting_date
	faJnlLine.Posting_date = faReclassJnlLine.Posting_date
	if faJnlLine.Posting_date.Equal(faJnlLine.Fa_posting_date) {
		faJnlLine.Posting_date = types.Date{}
	}

	faJnlLine.Document_no = faReclassJnlLine.Document_no
	if faJnlLine.Document_no == "" {
		faJnlLine.Document_no = t.faJnlDocumentNo
	}
	if faJnlLine.Document_no == "" {
		err = faReclassJnlLine.TestField("document_no")
		if err != nil {
			return err
		}
	}

	faJnlLine.Posting_no_series = tables.GetFANoSeries(faReclassJnlLine, t.templateName, t.batchName)
	err = faJnlLine.Validate("depreciation_book_code", faReclassJnlLine.Depreciation_book_code)
	if err != nil {
		return err
	}
	err = faJnlLine.Validate("amount", entryAmount)
	if err != nil {
		return err
	}
	faJnlLine.Description = faReclassJnlLine.Description
	faJnlLine.Fa_reclassification_entry = true
	t.faJnlNextLineNo += 10000
	faJnlLine.Line_no = t.faJnlNextLineNo
	return faJnlLine.InsertOrError(true)
}

// insertGenJnlLine (InsertGenJnlLine): a general journal line of the reclassification, with
// Insert Bal. Account its balancing lines
func (t *FAReclassTransferLine) insertGenJnlLine(faReclassJnlLine *gtables.FAReclassJournalLine, faPostingType gtables.FAJournalLineFa_posting_type, faNo types.Code, entryAmount types.Decimal, balAccount bool) error {
	var err error
	if !t.genJnlUsedOnce {
		t.genJnlUsedOnce = true
		t.genJnlDocumentNo, err = tables.GetGenJnlDocumentNo(faReclassJnlLine, t.templateName, t.batchName, faReclassJnlLine.Fa_posting_date)
		if err != nil {
			return err
		}
	}
	genJnlLine := gtables.NewGenJournalLine(faReclassJnlLine)
	genJnlLine.Init()
	genJnlLine.Journal_template_name = t.templateName
	genJnlLine.Journal_batch_name = t.batchName
	genJnlLine.Account_type = gtables.GenJournalLine_Account_type.FixedAsset
	genJnlLine.Fa_posting_type = gtables.GenJournalLineFa_posting_type(faPostingType + 1)
	err = genJnlLine.Validate("account_no", faNo)
	if err != nil {
		return err
	}
	err = genJnlLine.Validate("depreciation_book_code", faReclassJnlLine.Depreciation_book_code)
	if err != nil {
		return err
	}
	genJnlLine.Fa_posting_date = faReclassJnlLine.Fa_posting_date
	genJnlLine.Posting_date = faReclassJnlLine.Posting_date
	if genJnlLine.Posting_date.IsZero() {
		genJnlLine.Posting_date = faReclassJnlLine.Fa_posting_date
	}
	if genJnlLine.Posting_date.Equal(genJnlLine.Fa_posting_date) {
		genJnlLine.Fa_posting_date = types.Date{}
	}

	genJnlLine.Document_no = faReclassJnlLine.Document_no
	if genJnlLine.Document_no == "" {
		genJnlLine.Document_no = t.genJnlDocumentNo
	}
	if genJnlLine.Document_no == "" {
		err = faReclassJnlLine.TestField("document_no")
		if err != nil {
			return err
		}
	}

	genJnlLine.Posting_no_series = tables.GetGenNoSeries(faReclassJnlLine, t.templateName, t.batchName)
	err = genJnlLine.Validate("amount", entryAmount)
	if err != nil {
		return err
	}
	genJnlLine.Description = faReclassJnlLine.Description
	genJnlLine.Fa_reclassification_entry = true
	t.genJnlNextLineNo += 10000
	genJnlLine.Line_no = t.genJnlNextLineNo
	err = tables.SetGenJnlTrailCodes(genJnlLine)
	if err != nil {
		return err
	}
	err = genJnlLine.InsertOrError(true)
	if err != nil {
		return err
	}
	if balAccount {
		err = fajnl.GetBalAcc(genJnlLine)
		if err != nil {
			return err
		}
		last := gtables.NewGenJournalLine(faReclassJnlLine)
		last.SetRange("journal_template_name", t.templateName)
		last.SetRange("journal_batch_name", t.batchName)
		if last.FindLast() {
			t.genJnlNextLineNo = last.Line_no
		}
	}
	return nil
}
