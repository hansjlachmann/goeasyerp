package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// mainAssetComponentLogic is the business logic of table 5640 Main Asset Component: a line
// makes a fixed asset a component of the main asset, which becomes a main asset
type mainAssetComponentLogic struct{}

func init() { gtables.RegisterMainAssetComponentLogic(mainAssetComponentLogic{}) }

// OnDelete: the fixed asset is no component any more
func (mainAssetComponentLogic) OnDelete(rec *gtables.MainAssetComponent) error {
	lockFixedAsset(rec)
	if rec.Fa_no != "" {
		fa := gtables.NewFixedAsset(rec)
		err := fa.GetOrError(rec.Fa_no)
		if err != nil {
			return err
		}
		return updateMainAsset(rec, fa, gtables.FixedAsset_Main_asset_component.Blank)
	}
	return nil
}

// OnRename: a main asset component cannot be renamed
func (mainAssetComponentLogic) OnRename(rec *gtables.MainAssetComponent) error {
	return apperrors.RenameNotAllowed(gtables.MainAssetComponentTableName)
}

// OnValidate_Fa_no: the fixed asset becomes a component of the main asset — not the main asset
// itself, not a main asset, and not yet a component of another main asset
func (mainAssetComponentLogic) OnValidate_Fa_no(rec *gtables.MainAssetComponent) error {
	if rec.Fa_no == "" || rec.Main_asset_no == "" {
		return nil
	}
	lockFixedAsset(rec)
	fa := gtables.NewFixedAsset(rec)
	err := fa.GetOrError(rec.Fa_no)
	if err != nil {
		return err
	}
	if rec.Fa_no == rec.Main_asset_no {
		return faIsA(rec.Fa_no, gtables.FixedAsset_Main_asset_component.MainAsset)
	}
	rec.Description = fa.Description
	mainAssetComp := gtables.NewMainAssetComponent(rec)
	mainAssetComp.SetRange("main_asset_no", rec.Fa_no)
	if mainAssetComp.FindFirst() {
		return faIsA(rec.Fa_no, gtables.FixedAsset_Main_asset_component.MainAsset)
	}
	mainAssetComp.SetRange("main_asset_no")
	mainAssetComp.SetCurrentKey("fa_no")
	mainAssetComp.SetRange("fa_no", rec.Fa_no)
	if mainAssetComp.FindFirst() {
		return faIsA(rec.Fa_no, gtables.FixedAsset_Main_asset_component.Component)
	}
	mainAssetComp.SetRange("fa_no", rec.Main_asset_no)
	if mainAssetComp.FindFirst() {
		return faIsA(rec.Main_asset_no, gtables.FixedAsset_Main_asset_component.MainAsset)
	}
	err = updateMainAsset(rec, fa, gtables.FixedAsset_Main_asset_component.Component)
	if err != nil {
		return err
	}
	err = fa.GetOrError(rec.Main_asset_no)
	if err != nil {
		return err
	}
	if fa.Main_asset_component != gtables.FixedAsset_Main_asset_component.MainAsset {
		return apperrors.FAIsNotA(gtables.FixedAssetTableName, "main_asset_component", rec.Main_asset_no.String(), gtables.FixedAsset_Main_asset_component.MainAsset)
	}
	return nil
}

func lockFixedAsset(rec *gtables.MainAssetComponent) {
	gtables.NewFixedAsset(rec).LockTable()
	gtables.NewFADepreciationBook(rec).LockTable()
}

// updateMainAsset makes the fixed asset a component of the line's main asset (or no
// component), and the main asset a main asset while it has components
func updateMainAsset(rec *gtables.MainAssetComponent, fa *gtables.FixedAsset, componentType gtables.FixedAssetMain_asset_component) error {
	if componentType == gtables.FixedAsset_Main_asset_component.Blank {
		fa.Main_asset_component = gtables.FixedAsset_Main_asset_component.Blank
		fa.Component_of_main_asset = ""
	}
	if componentType == gtables.FixedAsset_Main_asset_component.Component {
		fa.Component_of_main_asset = rec.Main_asset_no
		fa.Main_asset_component = gtables.FixedAsset_Main_asset_component.Component
	}
	err := fa.ModifyOrError(true)
	if err != nil {
		return err
	}
	err = updateFADeprBooks(fa)
	if err != nil {
		return err
	}
	components := gtables.NewFixedAsset(rec)
	components.SetCurrentKey("component_of_main_asset")
	components.SetRange("component_of_main_asset", rec.Main_asset_no)
	components.SetRange("main_asset_component", gtables.FixedAsset_Main_asset_component.Component)
	fa2 := gtables.NewFixedAsset(rec)
	err = fa2.GetOrError(rec.Main_asset_no)
	if err != nil {
		return err
	}
	if components.FindFirst() {
		if fa2.Main_asset_component != gtables.FixedAsset_Main_asset_component.MainAsset {
			fa2.Main_asset_component = gtables.FixedAsset_Main_asset_component.MainAsset
			fa2.Component_of_main_asset = fa2.No
			err = fa2.ModifyOrError(true)
			if err != nil {
				return err
			}
			return updateFADeprBooks(fa2)
		}
		return nil
	}
	fa2.Main_asset_component = gtables.FixedAsset_Main_asset_component.Blank
	fa2.Component_of_main_asset = ""
	err = fa2.ModifyOrError(true)
	if err != nil {
		return err
	}
	return updateFADeprBooks(fa2)
}

// updateFADeprBooks: the fixed asset's depreciation books get its main asset fields
func updateFADeprBooks(fa *gtables.FixedAsset) error {
	fADeprBook := gtables.NewFADepreciationBook(fa)
	fADeprBook.SetCurrentKey("fa_no")
	fADeprBook.SetRange("fa_no", fa.No)
	for found := fADeprBook.FindSetBuffered(); found; found = fADeprBook.Next() {
		fADeprBook.Main_asset_component = gtables.FADepreciationBookMain_asset_component(fa.Main_asset_component)
		fADeprBook.Component_of_main_asset = fa.Component_of_main_asset
		err := fADeprBook.ModifyOrError(true)
		if err != nil {
			return err
		}
	}
	return nil
}

// faIsA: "Fixed Asset <no> is a <Main Asset|Component>." (CreateError)
func faIsA(faNo types.Code, mainAssetComponent gtables.FixedAssetMain_asset_component) error {
	return apperrors.FAIsA(gtables.FixedAssetTableName, "main_asset_component", faNo.String(), mainAssetComponent)
}
