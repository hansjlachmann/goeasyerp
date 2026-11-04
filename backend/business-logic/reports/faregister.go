package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 5603 Fixed Asset Register: the FA registers in the filters, each with the FA ledger
// entries it wrote (a canceled entry under the asset it was canceled from) and their total, and
// the total of all (main menu Fixed Assets → Reports; FA Registers → Print).
const FixedAssetRegisterID = 5603

func init() {
	Register(FixedAssetRegisterID, "fixed-asset-register", func() Report { return &fixedAssetRegister{} })
}

type fixedAssetRegister struct {
	faRegister *gtables.FARegister
	total      types.Decimal
}

func (r *fixedAssetRegister) Definition() *Definition {
	return &Definition{
		OnPreReport: r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "FARegister",
			Table:               gtables.FARegisterTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no"},
			OnPreDataItem:       Trigger(r.faRegisterOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.faRegisterOnAfterGetRecord),
			DataItems: []*DataItem{{
				Name:             "FALedgerEntry",
				Table:            gtables.FALedgerEntryTableName,
				Key:              []string{"entry_no"},
				OnPreDataItem:    Trigger(r.faLedgEntryOnPreDataItem),
				OnAfterGetRecord: Trigger(r.faLedgEntryOnAfterGetRecord),
			}},
		}},
	}
}

func (r *fixedAssetRegister) onPreReport(run *Run) error {
	setReportHeadValues(run)
	return nil
}

// faRegisterOnPreDataItem: the registers' filters in the head (FARegFilter)
func (r *fixedAssetRegister) faRegisterOnPreDataItem(rec *gtables.FARegister, run *Run) error {
	setFilterText(run, rec)
	return nil
}

func (r *fixedAssetRegister) faRegisterOnAfterGetRecord(rec *gtables.FARegister, run *Run) error {
	r.faRegister = rec
	return nil
}

// faLedgEntryOnPreDataItem: the register's entries
func (r *fixedAssetRegister) faLedgEntryOnPreDataItem(rec *gtables.FALedgerEntry, run *Run) error {
	rec.SetRange("entry_no", r.faRegister.From_entry_no, r.faRegister.To_entry_no)
	return nil
}

// faLedgEntryOnAfterGetRecord: a canceled entry is printed under the asset it was canceled from;
// the asset's description; the total of all registers
func (r *fixedAssetRegister) faLedgEntryOnAfterGetRecord(rec *gtables.FALedgerEntry, run *Run) error {
	canceledLedgEntry := ""
	if rec.Fa_no == "" && rec.Canceled_from_fa_no != "" {
		canceledLedgEntry = run.Text("CANCELED")
		rec.Fa_no = rec.Canceled_from_fa_no
	}
	run.SetValue("canceled", canceledLedgEntry)
	fa := gtables.NewFixedAsset(run)
	if !fa.Get(rec.Fa_no) {
		fa.Init()
	}
	run.SetValue("fa_description", fa.Description.String())
	r.total = r.total.Add(rec.Amount)
	run.SetValue("total", r.total)
	return nil
}
