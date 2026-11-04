package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// reservationEntryLogic is the business logic of table 337 Reservation_Entry.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/reservationentry_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type reservationEntryLogic struct{}

func init() { gtables.RegisterReservationEntryLogic(reservationEntryLogic{}) }

// OnInsert runs before a new record is inserted
func (reservationEntryLogic) OnInsert(rec *gtables.ReservationEntry) error {
	return nil
}

// OnModify runs before a record is modified
func (reservationEntryLogic) OnModify(rec *gtables.ReservationEntry) error {
	return nil
}

// OnDelete runs before a record is deleted
func (reservationEntryLogic) OnDelete(rec *gtables.ReservationEntry) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (reservationEntryLogic) OnRename(rec *gtables.ReservationEntry) error {
	return nil
}
