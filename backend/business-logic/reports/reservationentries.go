package reports

import (
	"strconv"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reservmgt"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 787 Delete Reservation Entries: the reservation entries that are no longer open (kept
// by Track Changes) with a valid-to date between the Starting and Ending Date are deleted.
// Report 788 Change Reservation Status: the open reservations valid up to the Ending Date are
// closed Rejected (Overdue) — the lines reserve them no more.
const (
	DeleteReservationEntriesID = 787
	ChangeReservationStatusID  = 788
)

func init() {
	Register(DeleteReservationEntriesID, "delete-reservation-entries", func() Report { return &deleteReservationEntries{} })
	Register(ChangeReservationStatusID, "change-reservation-status", func() Report { return &changeReservationStatus{} })
}

type deleteReservationEntries struct {
	dateCompression
	window *system.Window
}

func (r *deleteReservationEntries) Definition() *Definition {
	r.tableID = gtables.ReservationEntryTableID
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "starting_date", Type: "date"},
			{Name: "ending_date", Type: "date", Required: true},
		},
		Transaction: true,
		OnInitReport: func(run *Run) error {
			run.SetOption("ending_date", system.Today().String())
			return nil
		},
		OnPreReport: func(run *Run) error {
			r.read(run)
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Reservation_Entry", Table: gtables.ReservationEntryTableName,
			RequestFilterFields: []string{"item_no"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem:      Trigger(r.onPostDataItem),
		}},
	}
}

func (r *deleteReservationEntries) onPreDataItem(reservEntry *gtables.ReservationEntry, run *Run) error {
	err := r.confirm(run)
	if err != nil {
		return err
	}
	r.window = system.OpenWindow(run.Text("WINDOW"))
	reservEntry2 := gtables.NewReservationEntry(run)
	lastEntryNo := 0
	if reservEntry2.FindLast() {
		lastEntryNo = reservEntry2.Entry_no
	}
	reservEntry.SetFilter("reservation_status", "<>"+strconv.Itoa(int(gtables.ReservationEntry_Reservation_status.Open)))
	reservEntry.SetRange("entry_no", 0, lastEntryNo)
	reservEntry.SetRange("valid_to_date", r.entrdDateComprJnl.Starting_date, r.entrdDateComprJnl.Ending_date)
	return nil
}

// onAfterGetRecord: the closed entry is deleted
func (r *deleteReservationEntries) onAfterGetRecord(reservEntry *gtables.ReservationEntry, run *Run) error {
	r.window.Update(1, reservEntry.Item_no)
	r.window.Update(2, reservEntry.Valid_to_date)
	err := reservEntry.DeleteOrError(false)
	if err != nil {
		return err
	}
	r.noOfDeleted++
	r.window.Update(3, r.noOfDeleted)
	return nil
}

func (r *deleteReservationEntries) onPostDataItem(reservEntry *gtables.ReservationEntry, run *Run) error {
	r.window.Close()
	return nil
}

type changeReservationStatus struct {
	endDate               types.Date
	reservationManagement *reservmgt.ReservationManagement
}

func (r *changeReservationStatus) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "ending_date", Type: "date", Required: true},
		},
		Transaction: true,
		OnInitReport: func(run *Run) error {
			run.SetOption("ending_date", system.WorkDate().String())
			return nil
		},
		OnPreReport: func(run *Run) error {
			r.endDate = run.OptionDate("ending_date")
			r.reservationManagement = reservmgt.New(run)
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Reservation_Entry", Table: gtables.ReservationEntryTableName,
			Key:                 []string{"reservation_status", "item_no", "valid_to_date"},
			TableView:           []ftables.FieldFilter{{Field: "reservation_status", Expression: strconv.Itoa(int(gtables.ReservationEntry_Reservation_status.Open))}},
			RequestFilterFields: []string{"item_no"},
			OnPreDataItem: Trigger(func(reservEntry *gtables.ReservationEntry, run *Run) error {
				if system.GuiAllowed() && !system.Confirm(run.Text("CONFIRM"), r.endDate) {
					return Break()
				}
				reservEntry.SetRange("valid_to_date", types.Date{}, r.endDate)
				return nil
			}),
			OnAfterGetRecord: Trigger(func(reservEntry *gtables.ReservationEntry, run *Run) error {
				return r.reservationManagement.CloseReservEntry(
					reservEntry.Entry_no, gtables.ReservationEntry_Reservation_status.RejectedOverdue, true, true)
			}),
		}},
	}
}
