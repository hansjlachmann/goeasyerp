package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reservmgt"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The lines' reservation triggers (classic Reserve, Reserved Qty. (Base) and the
// ReservMgt.…VerifyChange calls): a line with a reserved quantity is checked against its
// reservation entries when it is saved; Reserve Never only without a reservation, and a line
// whose item reserves Always keeps Always.

// reservedQty: a reserved base quantity in the line's unit of measure
func reservedQty(reservedQtyBase, qtyPerUnitOfMeasure types.Decimal) types.Decimal {
	if qtyPerUnitOfMeasure.IsZero() {
		return reservedQtyBase
	}
	return system.Round(reservedQtyBase.Div(qtyPerUnitOfMeasure), roundPrecision)
}

// checkReserveAlways (the Reserve fields' OnValidate): a line of an item that reserves Always
// stays Always
func checkReserveAlways(rec interface {
	TestField(field string, value ...interface{}) error
}, xRecReserveAlways bool, itemNo types.Code, src ftables.Source, always interface{}) error {
	if !xRecReserveAlways {
		return nil
	}
	item := gtables.NewItem(src)
	if item.Get(itemNo) && item.Reserve == gtables.Item_Reserve.Always {
		return rec.TestField("reserve", always)
	}
	return nil
}

// OnValidate_Reserve: only an item line reserves; Never only without a reservation; an Always
// item keeps Always
func (salesLineLogic) OnValidate_Reserve(rec *gtables.SalesLine) error {
	if rec.Reserve != gtables.SalesLine_Reserve.Never {
		err := rec.TestField("type", gtables.SalesLine_Type.Item)
		if err == nil {
			err = rec.TestField("no")
		}
		if err != nil {
			return err
		}
	}
	if rec.Reserve == gtables.SalesLine_Reserve.Never && rec.Reserved_qty_base.IsPositive() {
		return rec.TestField("reserved_qty_base", types.Decimal{})
	}
	return checkReserveAlways(rec, rec.XRec().Reserve == gtables.SalesLine_Reserve.Always, rec.No, rec, gtables.SalesLine_Reserve.Always)
}

// OnValidate_Reserved_qty_base: the reserved quantity in the line's unit; Planned when the
// whole outstanding quantity is reserved
func (salesLineLogic) OnValidate_Reserved_qty_base(rec *gtables.SalesLine) error {
	err := rec.TestField("qty_per_unit_of_measure")
	if err != nil {
		return err
	}
	rec.Reserved_quantity = reservedQty(rec.Reserved_qty_base, rec.Qty_per_unit_of_measure)
	rec.Planned = rec.Reserved_quantity.Equal(rec.Outstanding_quantity)
	return nil
}

// OnValidate_Reserve: as on the sales line
func (purchaseLineLogic) OnValidate_Reserve(rec *gtables.PurchaseLine) error {
	if rec.Reserve != gtables.PurchaseLine_Reserve.Never {
		err := rec.TestField("type", gtables.PurchaseLine_Type.Item)
		if err == nil {
			err = rec.TestField("no")
		}
		if err != nil {
			return err
		}
	}
	if rec.Reserve == gtables.PurchaseLine_Reserve.Never && rec.Reserved_qty_base.IsPositive() {
		return rec.TestField("reserved_qty_base", types.Decimal{})
	}
	return checkReserveAlways(rec, rec.XRec().Reserve == gtables.PurchaseLine_Reserve.Always, rec.No, rec, gtables.PurchaseLine_Reserve.Always)
}

// OnValidate_Reserved_qty_base: the reserved quantity in the line's unit
func (purchaseLineLogic) OnValidate_Reserved_qty_base(rec *gtables.PurchaseLine) error {
	err := rec.TestField("qty_per_unit_of_measure")
	if err != nil {
		return err
	}
	rec.Reserved_quantity = reservedQty(rec.Reserved_qty_base, rec.Qty_per_unit_of_measure)
	return nil
}

// OnModify: a line with reservations still matches them (ItemJnlLineVerifyChange)
func (itemJournalLineLogic) OnModify(rec *gtables.ItemJournalLine) error {
	return reservmgt.New(rec).ItemJnlLineVerifyChange(rec)
}

// OnDelete: a line with reservations is not deleted
func (itemJournalLineLogic) OnDelete(rec *gtables.ItemJournalLine) error {
	err := rec.TestField("positive_reserved_qty_base", types.Decimal{})
	if err != nil {
		return err
	}
	return rec.TestField("negative_reserved_qty_base", types.Decimal{})
}

// OnValidate_Reserve: Never only without a reservation; an Always item keeps Always
func (itemJournalLineLogic) OnValidate_Reserve(rec *gtables.ItemJournalLine) error {
	if rec.Reserve == gtables.ItemJournalLine_Reserve.Never && rec.Positive_reserved_qty_base.IsPositive() {
		return rec.TestField("positive_reserved_qty_base", types.Decimal{})
	}
	return checkReserveAlways(rec, rec.XRec().Reserve == gtables.ItemJournalLine_Reserve.Always, rec.Item_no, rec, gtables.ItemJournalLine_Reserve.Always)
}

// OnValidate_Positive_reserved_qty_base / Negative: the reserved quantities in the line's unit
func (itemJournalLineLogic) OnValidate_Positive_reserved_qty_base(rec *gtables.ItemJournalLine) error {
	rec.Positive_reserved_quantity = reservedQty(rec.Positive_reserved_qty_base, rec.Qty_per_unit_of_measure)
	return nil
}

func (itemJournalLineLogic) OnValidate_Negative_reserved_qty_base(rec *gtables.ItemJournalLine) error {
	rec.Negative_reserved_quantity = reservedQty(rec.Negative_reserved_qty_base, rec.Qty_per_unit_of_measure)
	return nil
}

// OnModify: a line with reservations still matches them (BOMJnlLineVerifyChange)
func (bomJournalLineLogic) OnModify(rec *gtables.BOMJournalLine) error {
	return reservmgt.New(rec).BOMJnlLineVerifyChange(rec)
}

// OnDelete: a line with reservations is not deleted
func (bomJournalLineLogic) OnDelete(rec *gtables.BOMJournalLine) error {
	return rec.TestField("reserved_qty_base", types.Decimal{})
}

// OnValidate_Reserve: Never only without a reservation; an Always item keeps Always
func (bomJournalLineLogic) OnValidate_Reserve(rec *gtables.BOMJournalLine) error {
	if rec.Reserve == gtables.BOMJournalLine_Reserve.Never && rec.Reserved_qty_base.IsPositive() {
		return rec.TestField("reserved_qty_base", types.Decimal{})
	}
	return checkReserveAlways(rec, rec.XRec().Reserve == gtables.BOMJournalLine_Reserve.Always, rec.Item_no, rec, gtables.BOMJournalLine_Reserve.Always)
}

// OnValidate_Reserved_qty_base: the reserved quantity in the line's unit
func (bomJournalLineLogic) OnValidate_Reserved_qty_base(rec *gtables.BOMJournalLine) error {
	rec.Reserved_quantity = reservedQty(rec.Reserved_qty_base, rec.Qty_per_unit_of_measure)
	return nil
}

// OnValidate_Reserve: only an item line reserves; Never only without a reservation
func (requisitionLineLogic) OnValidate_Reserve(rec *gtables.RequisitionLine) error {
	if rec.Reserve != gtables.RequisitionLine_Reserve.Never {
		err := rec.TestField("type", gtables.RequisitionLine_Type.Item)
		if err == nil {
			err = rec.TestField("no")
		}
		if err != nil {
			return err
		}
	}
	if rec.Reserve == gtables.RequisitionLine_Reserve.Never && rec.Reserved_qty_base.IsPositive() {
		return rec.TestField("reserved_qty_base", types.Decimal{})
	}
	return checkReserveAlways(rec, rec.XRec().Reserve == gtables.RequisitionLine_Reserve.Always, rec.No, rec, gtables.RequisitionLine_Reserve.Always)
}

// OnValidate_Reserved_qty_base: the reserved quantity in the line's unit
func (requisitionLineLogic) OnValidate_Reserved_qty_base(rec *gtables.RequisitionLine) error {
	rec.Reserved_quantity = reservedQty(rec.Reserved_qty_base, rec.Qty_per_unit_of_measure)
	return nil
}

// checkReservedNotAbove (the Quantity OnValidate's "… must be reduced"): the reserved quantity
// is not more than the line's (outstanding) quantity
func checkReservedNotAbove(table, field string, qtyBase, reservedQtyBase types.Decimal) error {
	if qtyBase.Abs().LessThan(reservedQtyBase.Abs()) {
		return apperrors.ReservMustBeReduced(table, field)
	}
	return nil
}
