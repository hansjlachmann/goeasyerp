package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// fAPostingTypeSetupLogic is the business logic of table 5604 FA Posting Type Setup: per
// depreciation book, whether write-downs, appreciations and custom 1/2 are part of the book value
// and the depreciable basis, count in the depreciation and the gain/loss, are reversed before a
// disposal, and their sign. The book creates its lines (Depreciation Book OnInsert).
type fAPostingTypeSetupLogic struct{}

func init() { gtables.RegisterFAPostingTypeSetupLogic(fAPostingTypeSetupLogic{}) }

// OnValidate_Part_of_book_value: an amount not in the book value is not reversed before a
// disposal
func (fAPostingTypeSetupLogic) OnValidate_Part_of_book_value(rec *gtables.FAPostingTypeSetup) error {
	if !rec.Part_of_book_value {
		return rec.TestField("reverse_before_disposal", false)
	}
	return nil
}

// OnValidate_Reverse_before_disposal: only an amount in the book value
func (fAPostingTypeSetupLogic) OnValidate_Reverse_before_disposal(rec *gtables.FAPostingTypeSetup) error {
	if rec.Reverse_before_disposal {
		return rec.TestField("part_of_book_value", true)
	}
	return nil
}

// OnValidate_Depreciation_type: not an acquisition type at the same time
func (fAPostingTypeSetupLogic) OnValidate_Depreciation_type(rec *gtables.FAPostingTypeSetup) error {
	if rec.Depreciation_type {
		rec.Acquisition_type = false
	}
	return nil
}

// OnValidate_Acquisition_type: not a depreciation type at the same time
func (fAPostingTypeSetupLogic) OnValidate_Acquisition_type(rec *gtables.FAPostingTypeSetup) error {
	if rec.Acquisition_type {
		rec.Depreciation_type = false
	}
	return nil
}
