package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// SetResCapacity enters a resource's Capacity in its Date Filter (classic: a value typed into the
// FlowField of Resource Capacity 213): a Res. Capacity Entry with the difference to the capacity
// now, on the first date of the filter, for the resource and its group
func SetResCapacity(rec *gtables.Resource, capacity types.Decimal) error {
	date, ok := dateFilterMin(rec.Date_filter)
	if !ok {
		return apperrors.BudgetNotOneValue(gtables.ResourceTableName, "date_filter")
	}
	rec.CalcFields("capacity")
	if capacity.Equal(rec.Capacity) {
		return nil
	}
	resCapacityEntry := newResCapacityEntry(rec, date, capacity.Sub(rec.Capacity))
	err := resCapacityEntry.Validate("resource_no", rec.No)
	if err != nil {
		return err
	}
	err = resCapacityEntry.InsertOrError(true)
	if err != nil {
		return err
	}
	rec.Capacity = capacity
	return nil
}

// SetResGroupCapacity enters a resource group's Capacity in its Date Filter (Res. Group Capacity
// 214): a Res. Capacity Entry of the group alone with the difference
func SetResGroupCapacity(rec *gtables.ResourceGroup, capacity types.Decimal) error {
	date, ok := dateFilterMin(rec.Date_filter)
	if !ok {
		return apperrors.BudgetNotOneValue(gtables.ResourceGroupTableName, "date_filter")
	}
	rec.CalcFields("capacity")
	if capacity.Equal(rec.Capacity) {
		return nil
	}
	resCapacityEntry := newResCapacityEntry(rec, date, capacity.Sub(rec.Capacity))
	resCapacityEntry.Resource_group_no = rec.No
	err := resCapacityEntry.InsertOrError(true)
	if err != nil {
		return err
	}
	rec.Capacity = capacity
	return nil
}

// newResCapacityEntry: the next capacity entry for a date and capacity
func newResCapacityEntry(src ftables.Source, date types.Date, capacity types.Decimal) *gtables.ResCapacityEntry {
	resCapacityEntry := gtables.NewResCapacityEntry(src)
	resCapacityEntry.LockTable()
	entryNo := 1
	if resCapacityEntry.FindLast() {
		entryNo = resCapacityEntry.Entry_no + 1
	}
	resCapacityEntry.Init()
	resCapacityEntry.Entry_no = entryNo
	resCapacityEntry.Date = date
	resCapacityEntry.Capacity = capacity
	return resCapacityEntry
}
