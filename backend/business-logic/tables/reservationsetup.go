package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// reservationSetupLogic is the business logic of table 336 Reservation_Setup.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/reservationsetup_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type reservationSetupLogic struct{}

func init() { gtables.RegisterReservationSetupLogic(reservationSetupLogic{}) }

// OnInsert runs before a new record is inserted
func (reservationSetupLogic) OnInsert(rec *gtables.ReservationSetup) error {
	return nil
}

// OnModify runs before a record is modified
func (reservationSetupLogic) OnModify(rec *gtables.ReservationSetup) error {
	return nil
}

// OnDelete runs before a record is deleted
func (reservationSetupLogic) OnDelete(rec *gtables.ReservationSetup) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (reservationSetupLogic) OnRename(rec *gtables.ReservationSetup) error {
	return nil
}
