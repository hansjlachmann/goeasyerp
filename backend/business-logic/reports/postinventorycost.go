package reports

import (
	"sort"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/genjnl"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 1002 Post Inventory Cost to G/L: the part of the item ledger entries' adjusted cost not
// yet in the G/L is posted — the inventory account of the entry's inventory posting group
// against the COGS account (sales) or the inventory adjustment account of the General Posting
// Setup — per inventory posting group with one document no., or per item ledger entry with the
// entry's document no. The entries' Cost Posted to G/L follows.
//
// The additional-currency costs go with them (Add.-Curr. Cost Posted to G/L; a line of the
// additional-currency amount only where the LCY amount is 0).
//
// Not migrated yet: the printout (the posted amounts per item and per posting setup; the
// report's message gives the totals).
const PostInventoryCostToGLID = 1002

func init() {
	Register(PostInventoryCostToGLID, "post-inventory-cost-to-gl", func() Report { return &postInventoryCostToGL{} })
}

// The Post option (classic PostLevel)
const (
	postLevelPerPostingGroup = iota + 1
	postLevelPerEntry
)

// invtPostBufAmounts are a buffer line's Amount and Additional-Currency Amount
type invtPostBufAmounts struct {
	amount, addCurrAmount types.Decimal
}

// add (UpdateInvtPostingBuffer) adds to a buffer line
func (a invtPostBufAmounts) add(adjAmount, adjAmountAddCurr types.Decimal) invtPostBufAmounts {
	return invtPostBufAmounts{amount: a.amount.Add(adjAmount), addCurrAmount: a.addCurrAmount.Add(adjAmountAddCurr)}
}

// invtPostBufKey is a line of the inventory posting buffer (classic temporary table 48 Invt.
// Posting Buffer): the item side (an inventory posting group) or the matrix side (a general
// posting setup) of the amounts
type invtPostBufKey struct {
	postingGroupType    int // 0 Item (inventory account), 1 Matrix (COGS or adjustment account)
	entryType           int // item side: 0 Purchase/Pos. Adjmt./Neg. Adjmt., 1 Sale; matrix side: 2 Adjustment, 3 COGS
	postingGroup        types.Code
	genBusPostingGroup  types.Code
	genProdPostingGroup types.Code
	dimension1Code      types.Code
	dimension2Code      types.Code
}

const (
	bufTypeItem = iota
	bufTypeMatrix
)

const (
	bufEntryPurchAdjmt = iota
	bufEntrySale
	bufEntryAdjustment
	bufEntryCOGS
)

type postInventoryCostToGL struct {
	postLevel       int
	postingDate     types.Date
	postingDocNo    types.Code
	sourceCodeSetup *gtables.SourceCodeSetup
	genJnlPostLine  *genjnl.GenJnlPostLine
	invtPostBuf     map[invtPostBufKey]invtPostBufAmounts
	totalUsage      types.Decimal
	totalAdjAmount  types.Decimal
	window          *system.Window
}

func (r *postInventoryCostToGL) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "post_level", Type: "option", Default: "1", Options: []string{"PER_POSTING_GROUP", "PER_ENTRY"}},
			{Name: "posting_date", Type: "date", Required: true},
			{Name: "document_no", Type: "text"},
		},
		OnInitReport: r.onInitReport,
		OnPreReport:  r.onPreReport,
		OnPostReport: r.onPostReport,
		Transaction:  true,
		DataItems: []*DataItem{{
			Name: "Item_Ledger_Entry", Table: "Item_Ledger_Entry", Key: []string{"item_no"},
			RequestFilterFields: []string{"posting_date"},
			OnPreDataItem:       Trigger(r.itemLedgEntryOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.itemLedgEntryOnAfterGetRecord),
			OnPostDataItem:      Trigger(r.itemLedgEntryOnPostDataItem),
		}},
	}
}

// onInitReport: the posting date is the work date
func (r *postInventoryCostToGL) onInitReport(run *Run) error {
	run.SetOption("posting_date", system.WorkDate().String())
	return nil
}

// onPreReport: per posting group needs a document no., per entry takes the entries' own
func (r *postInventoryCostToGL) onPreReport(run *Run) error {
	r.postLevel = run.OptionInt("post_level")
	r.postingDate = run.OptionDate("posting_date")
	r.postingDocNo = types.Code(system.UpperCase(run.Option("document_no")))
	r.sourceCodeSetup = gtables.GetSourceCodeSetup(run)
	r.invtPostBuf = map[invtPostBufKey]invtPostBufAmounts{}
	r.genJnlPostLine = genjnl.NewGenJnlPostLine(run)

	language := fcodeunits.CurrentLanguage()
	documentNoCaption := i18n.GetInstance().FieldCaption(gtables.GenJournalLineTableName, "document_no", language)
	switch r.postLevel {
	case postLevelPerPostingGroup:
		if r.postingDocNo == "" {
			return system.Error(run.Text("ENTER_DOCUMENT_NO", documentNoCaption, run.Text("PER_POSTING_GROUP")))
		}
	case postLevelPerEntry:
		if r.postingDocNo != "" {
			return system.Error(run.Text("NO_DOCUMENT_NO", documentNoCaption, run.Text("PER_ENTRY")))
		}
	}
	return nil
}

func (r *postInventoryCostToGL) itemLedgEntryOnPreDataItem(itemLedgEntry *gtables.ItemLedgerEntry, run *Run) error {
	itemLedgEntry.LockTable()
	r.window = system.OpenWindow(run.Text("WINDOW"))
	return nil
}

func (r *postInventoryCostToGL) itemLedgEntryOnAfterGetRecord(itemLedgEntry *gtables.ItemLedgerEntry, run *Run) error {
	r.window.Update(1, itemLedgEntry.Item_no)
	adjAmount := itemLedgEntry.Adjusted_cost_invoiced_qty.Sub(itemLedgEntry.Cost_posted_to_gl)
	adjAmountAddCurr := itemLedgEntry.Add_curr_adj_cost_inv_qty.Sub(itemLedgEntry.Add_curr_cost_posted_to_gl)
	if adjAmount.IsZero() && adjAmountAddCurr.IsZero() {
		return Skip()
	}
	invtPostBuf1 := invtPostBufKey{postingGroupType: bufTypeItem, postingGroup: itemLedgEntry.Inventory_posting_group}
	invtPostBuf2 := invtPostBufKey{postingGroupType: bufTypeMatrix,
		genBusPostingGroup: itemLedgEntry.Gen_bus_posting_group, genProdPostingGroup: itemLedgEntry.Gen_prod_posting_group,
		dimension1Code: itemLedgEntry.Dimension_1_code, dimension2Code: itemLedgEntry.Dimension_2_code}
	if itemLedgEntry.Entry_type != gtables.ItemLedgerEntry_Entry_type.Sale {
		invtPostBuf1.entryType = bufEntryPurchAdjmt
		invtPostBuf2.entryType = bufEntryAdjustment
	} else {
		invtPostBuf1.entryType = bufEntrySale
		invtPostBuf2.entryType = bufEntryCOGS
	}
	if invtPostBuf2.entryType == bufEntryCOGS {
		r.totalUsage = r.totalUsage.Add(adjAmount)
	} else {
		r.totalAdjAmount = r.totalAdjAmount.Add(adjAmount)
	}
	r.invtPostBuf[invtPostBuf1] = r.invtPostBuf[invtPostBuf1].add(adjAmount, adjAmountAddCurr)
	r.invtPostBuf[invtPostBuf2] = r.invtPostBuf[invtPostBuf2].add(adjAmount.Neg(), adjAmountAddCurr.Neg())
	itemLedgEntry.Cost_posted_to_gl = itemLedgEntry.Adjusted_cost_invoiced_qty
	itemLedgEntry.Add_curr_cost_posted_to_gl = itemLedgEntry.Add_curr_adj_cost_inv_qty
	err := itemLedgEntry.ModifyOrError(false)
	if err != nil {
		return err
	}
	if r.postLevel == postLevelPerEntry {
		language := fcodeunits.CurrentLanguage()
		entryType := i18n.GetInstance().OptionCaption(gtables.ItemLedgerEntryTableName, "entry_type", i18n.Key(itemLedgEntry.Entry_type.String()), language)
		description := run.Text("ENTRY_DESCRIPTION", entryType, itemLedgEntry.Source_no, itemLedgEntry.Posting_date)
		return r.postInvtPostingBuffer(run, itemLedgEntry.Document_no, description)
	}
	return nil
}

func (r *postInventoryCostToGL) itemLedgEntryOnPostDataItem(itemLedgEntry *gtables.ItemLedgerEntry, run *Run) error {
	r.window.Close()
	return nil
}

// onPostReport posts what is left in the buffer (per posting group: all of it) and tells the
// totals
func (r *postInventoryCostToGL) onPostReport(run *Run) error {
	err := r.postInvtPostingBuffer(run, r.postingDocNo, "")
	if err != nil {
		return err
	}
	run.Message(run.Text("POSTED", r.totalUsage.Neg(), r.totalAdjAmount.Neg()))
	return nil
}

// postInvtPostingBuffer (PostInvtPostingBuffer): a G/L line per buffer line, then the buffer is
// empty. Per entry the description is given, per posting group it names the posting group.
func (r *postInventoryCostToGL) postInvtPostingBuffer(run *Run, documentNo types.Code, description string) error {
	if len(r.invtPostBuf) == 0 {
		return nil
	}
	keys := make([]invtPostBufKey, 0, len(r.invtPostBuf))
	for key := range r.invtPostBuf {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return invtPostBufLess(keys[i], keys[j]) })

	for _, key := range keys {
		buf := r.invtPostBuf[key]
		if buf.amount.IsZero() && buf.addCurrAmount.IsZero() {
			continue
		}
		genJnlLine := gtables.NewGenJournalLine(run)
		genJnlLine.Init()
		err := genJnlLine.Validate("posting_date", r.postingDate)
		if err != nil {
			return err
		}
		genJnlLine.Document_no = documentNo
		genJnlLine.Source_code = r.sourceCodeSetup.Inventory_post_cost
		genJnlLine.System_created_entry = true
		genJnlLine.Description = types.Text(description)
		switch key.postingGroupType {
		case bufTypeItem:
			invtPostingGr := gtables.NewInventoryPostingGroup(run)
			err = invtPostingGr.GetOrError(key.postingGroup)
			if err != nil {
				return err
			}
			err = invtPostingGr.TestField("inventory_account")
			if err != nil {
				return err
			}
			genJnlLine.Account_no = invtPostingGr.Inventory_account
			if r.postLevel == postLevelPerPostingGroup {
				if key.entryType == bufEntrySale {
					genJnlLine.Description = types.Text(run.Text("COGS_POSTING_GROUP", key.postingGroup))
				} else {
					genJnlLine.Description = types.Text(run.Text("ADJ_POSTING_GROUP", key.postingGroup))
				}
			}
		case bufTypeMatrix:
			genPostingSetup := gtables.NewGeneralPostingSetup(run)
			err = genPostingSetup.GetOrError(map[string]interface{}{"gen_bus_posting_group": key.genBusPostingGroup, "gen_prod_posting_group": key.genProdPostingGroup})
			if err != nil {
				return err
			}
			switch key.entryType {
			case bufEntryAdjustment:
				err = genPostingSetup.TestField("inventory_adjmt_account")
				if err != nil {
					return err
				}
				genJnlLine.Account_no = genPostingSetup.Inventory_adjmt_account
				if r.postLevel == postLevelPerPostingGroup {
					genJnlLine.Description = types.Text(run.Text("ADJ_GEN_SETUP", key.genBusPostingGroup, key.genProdPostingGroup))
				}
			case bufEntryCOGS:
				err = genPostingSetup.TestField("cogs_account")
				if err != nil {
					return err
				}
				genJnlLine.Account_no = genPostingSetup.Cogs_account
				if r.postLevel == postLevelPerPostingGroup {
					genJnlLine.Description = types.Text(run.Text("COGS_GEN_SETUP", key.genBusPostingGroup, key.genProdPostingGroup))
				}
			}
		}
		genJnlLine.Additional_currency_posting = gtables.GenJournalLine_Additional_currency_posting.None
		err = genJnlLine.Validate("amount", buf.amount)
		if err != nil {
			return err
		}
		glSetup := gtables.GetGeneralLedgerSetup(run)
		if glSetup.Additional_reporting_currency != "" {
			genJnlLine.Source_currency_code = glSetup.Additional_reporting_currency
			genJnlLine.Source_currency_amount = buf.addCurrAmount
			if genJnlLine.Amount.IsZero() && !genJnlLine.Source_currency_amount.IsZero() {
				genJnlLine.Additional_currency_posting = gtables.GenJournalLine_Additional_currency_posting.AdditionalCurrencyAmountOnly
				err = genJnlLine.Validate("amount", genJnlLine.Source_currency_amount)
				if err != nil {
					return err
				}
				genJnlLine.Source_currency_amount = types.Decimal{}
			}
		}
		genJnlLine.Dimension_1_code = key.dimension1Code
		genJnlLine.Dimension_2_code = key.dimension2Code
		err = r.genJnlPostLine.Run(genJnlLine)
		if err != nil {
			return err
		}
	}
	r.invtPostBuf = map[invtPostBufKey]invtPostBufAmounts{}
	return nil
}

// invtPostBufLess is the buffer's key order (posting group type, entry type, posting groups,
// dimension 1, dimension 2)
func invtPostBufLess(a, b invtPostBufKey) bool {
	if a.postingGroupType != b.postingGroupType {
		return a.postingGroupType < b.postingGroupType
	}
	if a.entryType != b.entryType {
		return a.entryType < b.entryType
	}
	for _, pair := range [][2]types.Code{
		{a.postingGroup, b.postingGroup}, {a.genBusPostingGroup, b.genBusPostingGroup},
		{a.genProdPostingGroup, b.genProdPostingGroup}, {a.dimension1Code, b.dimension1Code}, {a.dimension2Code, b.dimension2Code},
	} {
		if pair[0] != pair[1] {
			return pair[0] < pair[1]
		}
	}
	return false
}
