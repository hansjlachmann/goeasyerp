package reports

import (
	"sort"

	"github.com/hansjlachmann/openerp/backend/business-logic/genjnl"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 1002 Post Inventory Cost to G/L: the part of the item ledger entries' adjusted cost not
// yet in the G/L is posted — the inventory account of the entry's inventory posting group
// against the COGS account (sales) or the inventory adjustment account of the General Posting
// Setup — per inventory posting group with one document no., or per item ledger entry with the
// entry's document no. The entries' Cost Posted to G/L follows.
//
// Not migrated yet: the printout (the posted amounts per item and per posting setup; the
// report's message gives the totals) and the additional reporting currency.
const PostInventoryCostToGLID = 1002

func init() {
	Register(PostInventoryCostToGLID, "post-inventory-cost-to-gl", func() Report { return &postInventoryCostToGL{} })
}

// The Post option (classic PostLevel)
const (
	postLevelPerPostingGroup = iota + 1
	postLevelPerEntry
)

// invtPostBufKey is a line of the inventory posting buffer (classic temporary table 48 Invt.
// Posting Buffer): the item side (an inventory posting group) or the matrix side (a general
// posting setup) of the amounts
type invtPostBufKey struct {
	postingGroupType    int // 0 Item (inventory account), 1 Matrix (COGS or adjustment account)
	entryType           int // item side: 0 Purchase/Pos. Adjmt./Neg. Adjmt., 1 Sale; matrix side: 2 Adjustment, 3 COGS
	postingGroup        types.Code
	genBusPostingGroup  types.Code
	genProdPostingGroup types.Code
	departmentCode      types.Code
	projectCode         types.Code
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
	invtPostBuf     map[invtPostBufKey]types.Decimal
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
	r.invtPostBuf = map[invtPostBufKey]types.Decimal{}
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
	if adjAmount.IsZero() {
		return Skip()
	}
	invtPostBuf1 := invtPostBufKey{postingGroupType: bufTypeItem, postingGroup: itemLedgEntry.Inventory_posting_group}
	invtPostBuf2 := invtPostBufKey{postingGroupType: bufTypeMatrix,
		genBusPostingGroup: itemLedgEntry.Gen_bus_posting_group, genProdPostingGroup: itemLedgEntry.Gen_prod_posting_group,
		departmentCode: itemLedgEntry.Department_code, projectCode: itemLedgEntry.Project_code}
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
	r.invtPostBuf[invtPostBuf1] = r.invtPostBuf[invtPostBuf1].Add(adjAmount)
	r.invtPostBuf[invtPostBuf2] = r.invtPostBuf[invtPostBuf2].Sub(adjAmount)
	itemLedgEntry.Cost_posted_to_gl = itemLedgEntry.Adjusted_cost_invoiced_qty
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
		if r.invtPostBuf[key].IsZero() {
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
		err = genJnlLine.Validate("amount", r.invtPostBuf[key])
		if err != nil {
			return err
		}
		genJnlLine.Department_code = key.departmentCode
		genJnlLine.Project_code = key.projectCode
		err = r.genJnlPostLine.Run(genJnlLine)
		if err != nil {
			return err
		}
	}
	r.invtPostBuf = map[invtPostBufKey]types.Decimal{}
	return nil
}

// invtPostBufLess is the buffer's key order (posting group type, entry type, posting groups,
// department, project)
func invtPostBufLess(a, b invtPostBufKey) bool {
	if a.postingGroupType != b.postingGroupType {
		return a.postingGroupType < b.postingGroupType
	}
	if a.entryType != b.entryType {
		return a.entryType < b.entryType
	}
	for _, pair := range [][2]types.Code{
		{a.postingGroup, b.postingGroup}, {a.genBusPostingGroup, b.genBusPostingGroup},
		{a.genProdPostingGroup, b.genProdPostingGroup}, {a.departmentCode, b.departmentCode}, {a.projectCode, b.projectCode},
	} {
		if pair[0] != pair[1] {
			return pair[0] < pair[1]
		}
	}
	return false
}
