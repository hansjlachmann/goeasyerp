package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/noseries"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// insuranceLogic is the business logic of table 5628 Insurance
type insuranceLogic struct{}

func init() {
	gtables.RegisterInsuranceLogic(insuranceLogic{})
	// the No. field's "…" (Insurance Card): pick another number series
	functions.Register("table", gtables.InsuranceTableName, "AssistEdit", functions.ForAssistEdit(AssistEditInsurance), functions.WithAssistEdit())
}

// OnInsert: an insurance without No. gets the next number of the Insurance Nos. series of the
// FA Setup
func (insuranceLogic) OnInsert(rec *gtables.Insurance) error {
	if rec.No == "" {
		faSetup := gtables.GetFASetup(rec)
		err := faSetup.TestField("insurance_nos")
		if err != nil {
			return err
		}
		noSeriesMgt := noseries.New(rec)
		xRec := rec.XRec()
		err = noSeriesMgt.InitSeries(faSetup.Insurance_nos, xRec.No_series, types.Date{}, &rec.No, &rec.No_series)
		if err != nil {
			return err
		}
	}
	return nil
}

// OnModify sets Last Date Modified
func (insuranceLogic) OnModify(rec *gtables.Insurance) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnRename sets Last Date Modified
func (insuranceLogic) OnRename(rec *gtables.Insurance) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnDelete: the insurance's coverage entries lose it and their fixed asset
// (MoveInsuranceEntries); its comments are deleted
func (insuranceLogic) OnDelete(rec *gtables.Insurance) error {
	MoveInsuranceEntries(rec)
	DeleteComments(rec, gtables.CommentLine_Table_name.Insurance, rec.No)
	return nil
}

// OnValidate_No: a number typed by hand must be allowed by the Insurance Nos. series (Manual
// Nos.) and takes the insurance out of its series
func (insuranceLogic) OnValidate_No(rec *gtables.Insurance) error {
	xRec := rec.XRec()
	if rec.No != xRec.No {
		faSetup := gtables.GetFASetup(rec)
		noSeriesMgt := noseries.New(rec)
		series, issued := noSeriesMgt.IssuedBy(faSetup.Insurance_nos, rec.No)
		if issued && xRec.No == "" {
			rec.No_series = series
		} else {
			err := noSeriesMgt.TestManual(faSetup.Insurance_nos)
			if err != nil {
				return err
			}
			rec.No_series = ""
		}
	}
	return nil
}

// OnValidate_Description: Search Description follows the description while it was blank or the
// old description
func (insuranceLogic) OnValidate_Description(rec *gtables.Insurance) error {
	xRec := rec.XRec()
	if rec.Search_description == "" || string(rec.Search_description) == system.UpperCase(xRec.Description.String()) {
		rec.Search_description = types.NewCode(system.CopyStr(rec.Description.String(), 1, 50))
	}
	return nil
}

// AssistEditInsurance lets the user pick another number series for the insurance (AssistEdit
// of the No. field); false when the user cancelled
func AssistEditInsurance(rec *gtables.Insurance) (bool, error) {
	faSetup := gtables.GetFASetup(rec)
	err := faSetup.TestField("insurance_nos")
	if err != nil {
		return false, err
	}
	noSeriesMgt := noseries.New(rec)
	xRec := rec.XRec()
	noSeries := rec.No_series
	picked, err := noSeriesMgt.SelectSeries(faSetup.Insurance_nos, xRec.No_series, &noSeries)
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
