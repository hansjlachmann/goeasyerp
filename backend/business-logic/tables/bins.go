package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Bins on the lines: a line's Bin Code is a bin of its location — a new location clears it, a
// bin needs a location (the item journal: only when one is given; a transfer's New Bin Code
// is a bin of its new location)

// OnValidate_Location_code: the line's bin belonged to the old location
func (salesLineLogic) OnValidate_Location_code(rec *gtables.SalesLine) error {
	err0 := testSalesLineNotAssociated(rec, "location_code")
	if err0 != nil {
		return err0
	}
	rec.Bin_code = ""
	if rec.Reserve != gtables.SalesLine_Reserve.Always {
		return checkSalesLineItemAvailable(rec)
	}
	return nil
}

func (purchaseLineLogic) OnValidate_Location_code(rec *gtables.PurchaseLine) error {
	err0 := testPurchLineNotDropShipment(rec, "location_code")
	if err0 != nil {
		return err0
	}
	rec.Bin_code = ""
	return nil
}

func (itemJournalLineLogic) OnValidate_Location_code(rec *gtables.ItemJournalLine) error {
	rec.Bin_code = ""
	if rec.Reserve != gtables.ItemJournalLine_Reserve.Always {
		return checkItemJnlItemAvailable(rec)
	}
	return nil
}

func (bomJournalLineLogic) OnValidate_Location_code(rec *gtables.BOMJournalLine) error {
	rec.Bin_code = ""
	if rec.Reserve != gtables.BOMJournalLine_Reserve.Always {
		return checkBOMJnlItemAvailable(rec)
	}
	return nil
}

// OnValidate_Bin_code: a bin needs the line's location (an item line on purchase documents and
// requisition lines)
func (salesLineLogic) OnValidate_Bin_code(rec *gtables.SalesLine) error {
	err0 := testSalesLineNotAssociated(rec, "bin_code")
	if err0 != nil {
		return err0
	}
	err := rec.TestField("location_code")
	if err != nil {
		return err
	}
	if rec.Reserve != gtables.SalesLine_Reserve.Always {
		return checkSalesLineItemAvailable(rec)
	}
	return nil
}

func (purchaseLineLogic) OnValidate_Bin_code(rec *gtables.PurchaseLine) error {
	err0 := testPurchLineNotDropShipment(rec, "bin_code")
	if err0 != nil {
		return err0
	}
	err := rec.TestField("type", gtables.PurchaseLine_Type.Item)
	if err != nil {
		return err
	}
	return rec.TestField("location_code")
}

func (itemJournalLineLogic) OnValidate_Bin_code(rec *gtables.ItemJournalLine) error {
	if rec.Bin_code != "" {
		return rec.TestField("location_code")
	}
	return nil
}

func (itemJournalLineLogic) OnValidate_New_bin_code(rec *gtables.ItemJournalLine) error {
	err := rec.TestField("entry_type", gtables.ItemJournalLine_Entry_type.Transfer)
	if err != nil {
		return err
	}
	return rec.TestField("new_location_code")
}

func (bomJournalLineLogic) OnValidate_Bin_code(rec *gtables.BOMJournalLine) error {
	return rec.TestField("location_code")
}

func (requisitionLineLogic) OnValidate_Bin_code(rec *gtables.RequisitionLine) error {
	err := rec.TestField("type", gtables.RequisitionLine_Type.Item)
	if err != nil {
		return err
	}
	return rec.TestField("location_code")
}
