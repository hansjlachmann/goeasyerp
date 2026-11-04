package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration046Reservations{})
}

// Migration046Reservations goes with reservations: Reservation Setup 336 and Reservation Entry
// 337. The READER role may read them, as all business tables; the tables come from the table
// sync (Entry Summary 338 is only used as a temporary table).
type Migration046Reservations struct{}

func (m *Migration046Reservations) Version() int {
	return 46
}

func (m *Migration046Reservations) Name() string {
	return "reservations"
}

func (m *Migration046Reservations) Description() string {
	return "READER reads the reservation setup and entries"
}

func (m *Migration046Reservations) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"RESERVATION_SETUP", "RESERVATION_ENTRY"})
}
