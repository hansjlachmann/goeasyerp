package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/noseries"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// fixedAssetLogic is the business logic of table 5600 Fixed Asset
type fixedAssetLogic struct{}

func init() {
	gtables.RegisterFixedAssetLogic(fixedAssetLogic{})
	// the No. field's "…" (Fixed Asset Card): pick another number series
	functions.Register("table", gtables.FixedAssetTableName, "AssistEdit", functions.ForAssistEdit(AssistEditFixedAsset), functions.WithAssistEdit())
}

// OnInsert: a fixed asset without No. gets the next number of the Fixed Asset Nos. series of
// the FA Setup; a new asset is neither a main asset nor a component
func (fixedAssetLogic) OnInsert(rec *gtables.FixedAsset) error {
	if rec.No == "" {
		faSetup := gtables.GetFASetup(rec)
		err := faSetup.TestField("fixed_asset_nos")
		if err != nil {
			return err
		}
		noSeriesMgt := noseries.New(rec)
		xRec := rec.XRec()
		err = noSeriesMgt.InitSeries(faSetup.Fixed_asset_nos, xRec.No_series, types.Date{}, &rec.No, &rec.No_series)
		if err != nil {
			return err
		}
	}
	rec.Main_asset_component = gtables.FixedAsset_Main_asset_component.Blank
	rec.Component_of_main_asset = ""
	return nil
}

// OnModify sets Last Date Modified
func (fixedAssetLogic) OnModify(rec *gtables.FixedAsset) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnRename sets Last Date Modified
func (fixedAssetLogic) OnRename(rec *gtables.FixedAsset) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnDelete: a main asset cannot be deleted. The asset's insurance coverage entries lose it,
// its depreciation books are deleted (their entries lose it, MoveFAEntries), it leaves its main
// asset (which is no main asset any more without components), and its maintenance
// registrations and comments are deleted.
func (fixedAssetLogic) OnDelete(rec *gtables.FixedAsset) error {
	fADeprBook := gtables.NewFADepreciationBook(rec)
	mainAssetComp := gtables.NewMainAssetComponent(rec)
	rec.LockTable()
	fADeprBook.LockTable()
	mainAssetComp.LockTable()
	if rec.Main_asset_component == gtables.FixedAsset_Main_asset_component.MainAsset {
		return apperrors.FAMainAssetDelete()
	}
	MoveFAInsuranceEntries(rec, rec.No)
	fADeprBook.SetCurrentKey("fa_no")
	fADeprBook.SetRange("fa_no", rec.No)
	for found := fADeprBook.FindSetBuffered(); found; found = fADeprBook.Next() {
		err := fADeprBook.DeleteOrError(true)
		if err != nil {
			return err
		}
	}
	if fADeprBook.FindFirst() {
		return apperrors.FAHasDeprBooks(gtables.FixedAssetTableName, rec.No.String())
	}

	mainAssetComp.SetCurrentKey("fa_no")
	mainAssetComp.SetRange("fa_no", rec.No)
	mainAssetComp.DeleteAll()
	if rec.Main_asset_component == gtables.FixedAsset_Main_asset_component.Component {
		mainAssetComp.Reset()
		mainAssetComp.SetRange("main_asset_no", rec.Component_of_main_asset)
		mainAssetComp.SetRange("fa_no", types.Code(""))
		mainAssetComp.DeleteAll()
		mainAssetComp.SetRange("fa_no")
		if !mainAssetComp.FindFirst() {
			fa := gtables.NewFixedAsset(rec)
			err := fa.GetOrError(rec.Component_of_main_asset)
			if err != nil {
				return err
			}
			fa.Main_asset_component = gtables.FixedAsset_Main_asset_component.Blank
			fa.Component_of_main_asset = ""
			err = fa.ModifyOrError(false)
			if err != nil {
				return err
			}
		}
	}

	maintenanceRegistration := gtables.NewMaintenanceRegistration(rec)
	maintenanceRegistration.SetRange("fa_no", rec.No)
	maintenanceRegistration.DeleteAll()

	DeleteComments(rec, gtables.CommentLine_Table_name.FixedAsset, rec.No)
	return nil
}

// OnValidate_No: a number typed by hand must be allowed by the Fixed Asset Nos. series (Manual
// Nos.) and takes the asset out of its series; a new asset saved with the number its series
// just issued (AssistEdit) keeps that series
func (fixedAssetLogic) OnValidate_No(rec *gtables.FixedAsset) error {
	xRec := rec.XRec()
	if rec.No != xRec.No {
		faSetup := gtables.GetFASetup(rec)
		noSeriesMgt := noseries.New(rec)
		series, issued := noSeriesMgt.IssuedBy(faSetup.Fixed_asset_nos, rec.No)
		if issued && xRec.No == "" {
			rec.No_series = series
		} else {
			err := noSeriesMgt.TestManual(faSetup.Fixed_asset_nos)
			if err != nil {
				return err
			}
			rec.No_series = ""
		}
	}
	return nil
}

// OnValidate_Description: Search Description follows the description while it was blank or the
// old description; the asset's depreciation books get the new description
func (fixedAssetLogic) OnValidate_Description(rec *gtables.FixedAsset) error {
	xRec := rec.XRec()
	if rec.Search_description == "" || string(rec.Search_description) == system.UpperCase(xRec.Description.String()) {
		rec.Search_description = types.NewCode(system.CopyStr(rec.Description.String(), 1, 50))
	}
	if rec.Description != xRec.Description {
		fADeprBook := gtables.NewFADepreciationBook(rec)
		fADeprBook.SetCurrentKey("fa_no")
		fADeprBook.SetRange("fa_no", rec.No)
		fADeprBook.ModifyAll("description", rec.Description)
	}
	return nil
}

// OnValidate_Budgeted_asset: not for an asset with entries (ChangeBudget)
func (fixedAssetLogic) OnValidate_Budgeted_asset(rec *gtables.FixedAsset) error {
	return ChangeBudget(rec)
}

// AssistEditFixedAsset lets the user pick another number series for the fixed asset — the
// Fixed Asset Nos. series or one related to it — and gives the asset that series' next number
// (AssistEdit of the No. field); false when the user cancelled
func AssistEditFixedAsset(rec *gtables.FixedAsset) (bool, error) {
	faSetup := gtables.GetFASetup(rec)
	err := faSetup.TestField("fixed_asset_nos")
	if err != nil {
		return false, err
	}
	noSeriesMgt := noseries.New(rec)
	xRec := rec.XRec()
	noSeries := rec.No_series
	picked, err := noSeriesMgt.SelectSeries(faSetup.Fixed_asset_nos, xRec.No_series, &noSeries)
	if err != nil {
		return false, err
	}
	if picked {
		err = noSeriesMgt.SetSeries(&rec.No)
		if err != nil {
			return false, err
		}
		rec.No_series = noSeries
	}
	return picked, nil
}
