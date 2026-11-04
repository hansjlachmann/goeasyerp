package reservmgt

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// ModifyReservedQuantity is codeunit 405 Item Entry-Edit: an item ledger entry the Available -
// Item Ledg. Entries page reserved from gets its new Reserved Quantity — on the entry as stored
// (locked), when it still has that much available and is the reservation's item, variant,
// location, bin, serial and lot number
func (m *ReservationManagement) ModifyReservedQuantity(itemLedgEntry *gtables.ItemLedgerEntry, reservEntry *gtables.ReservationEntry) error {
	itemLedgEntry2 := gtables.NewItemLedgerEntry(m.src)
	itemLedgEntry2.LockTable()
	err := itemLedgEntry2.GetOrError(itemLedgEntry.Entry_no)
	if err != nil {
		return err
	}
	if itemLedgEntry2.Remaining_quantity.Add(itemLedgEntry.Reserved_quantity).IsNegative() {
		return apperrors.ReservAvailableQty(itemLedgEntry2.Remaining_quantity.Add(itemLedgEntry2.Reserved_quantity).String())
	}
	for _, check := range []struct {
		field string
		value types.Code
	}{
		{"item_no", reservEntry.Item_no},
		{"variant_code", reservEntry.Variant_code},
		{"location_code", reservEntry.Location_code},
		{"bin_code", reservEntry.Bin_code},
		{"serial_no", reservEntry.Serial_no},
		{"lot_no", reservEntry.Lot_no},
	} {
		err = itemLedgEntry2.TestField(check.field, check.value)
		if err != nil {
			return err
		}
	}
	itemLedgEntry2.Reserved_quantity = itemLedgEntry.Reserved_quantity
	return itemLedgEntry2.ModifyOrError(false)
}
