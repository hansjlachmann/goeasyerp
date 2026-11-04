package itemjnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

const itemJnlLine = gtables.ItemJournalLineTableName

// ItemJnlCheckLine is codeunit 21 Item Jnl.-Check Line: it checks an item journal line before
// it is posted. It keeps the user's allowed posting period between lines.
type ItemJnlCheckLine struct {
	allowPostingFrom types.Date
	allowPostingTo   types.Date
	invtSetup        *gtables.InventorySetup
}

// RunCheck checks a journal line (OnRun)
func (c *ItemJnlCheckLine) RunCheck(rec *gtables.ItemJournalLine) error {
	if tables.ItemJnlLineEmptyLine(rec) {
		return nil
	}
	err := rec.TestField("item_no")
	if err != nil {
		return err
	}
	err = rec.TestField("posting_date")
	if err != nil {
		return err
	}
	err = rec.TestField("document_no")
	if err != nil {
		return err
	}
	if rec.Posting_date.IsClosing() {
		return apperrors.JnlClosingDate(itemJnlLine, "posting_date")
	}

	if c.allowPostingFrom.IsZero() && c.allowPostingTo.IsZero() {
		c.invtSetup = gtables.GetInventorySetup(rec)
		glSetup := gtables.GetGeneralLedgerSetup(rec)
		if system.UserId() != "" {
			userSetup := gtables.NewUserSetup(rec)
			if userSetup.Get(types.Code(system.UserId())) {
				c.allowPostingFrom = userSetup.Allow_posting_from
				c.allowPostingTo = userSetup.Allow_posting_to
			}
		}
		if c.allowPostingFrom.IsZero() && c.allowPostingTo.IsZero() {
			c.allowPostingFrom = glSetup.Allow_posting_from
			c.allowPostingTo = glSetup.Allow_posting_to
		}
		if c.allowPostingTo.IsZero() {
			c.allowPostingTo, _ = system.DMY2Date(31, 12, 9999)
		}
	}
	if rec.Posting_date.Before(c.allowPostingFrom) || c.allowPostingTo.Before(rec.Posting_date) {
		return apperrors.JnlDateNotAllowed(itemJnlLine, "posting_date")
	}

	if !rec.Document_date.IsZero() {
		if rec.Document_date.IsClosing() {
			return apperrors.JnlClosingDate(itemJnlLine, "document_date")
		}
	}

	err = rec.TestField("gen_prod_posting_group")
	if err != nil {
		return err
	}

	if c.invtSetup.Location_mandatory {
		err = rec.TestField("location_code")
		if err != nil {
			return err
		}
		if rec.Entry_type == gtables.ItemJournalLine_Entry_type.Transfer {
			err = rec.TestField("new_location_code")
			if err != nil {
				return err
			}
		}
	}

	if rec.Entry_type == gtables.ItemJournalLine_Entry_type.PositiveAdjmt || rec.Entry_type == gtables.ItemJournalLine_Entry_type.NegativeAdjmt {
		err = rec.TestField("discount_amount", types.Decimal{})
		if err != nil {
			return err
		}
	}

	if rec.Serial_no != "" {
		one := types.NewDecimalFromInt(1)
		if rec.Quantity.Equal(rec.Invoiced_quantity) || rec.Invoiced_quantity.IsZero() {
			if !rec.Quantity.Abs().Equal(one) {
				return apperrors.SerialQtyMustBeOne(gtables.ItemJournalLineTableName, "quantity")
			}
		} else {
			if !rec.Invoiced_quantity.Abs().Equal(one) {
				return apperrors.SerialQtyMustBeOne(gtables.ItemJournalLineTableName, "invoiced_quantity")
			}
		}
	}

	if rec.Entry_type == gtables.ItemJournalLine_Entry_type.Transfer {
		err = rec.TestField("amount", types.Decimal{})
		if err != nil {
			return err
		}
		err = rec.TestField("discount_amount", types.Decimal{})
		if err != nil {
			return err
		}
		if rec.Quantity.IsNegative() {
			return apperrors.ItemJnlNotNegative(itemJnlLine, "quantity", "entry_type", rec.Entry_type)
		}
		if !rec.Quantity.Equal(rec.Invoiced_quantity) {
			return apperrors.ItemJnlSameValue(itemJnlLine, "invoiced_quantity", "quantity")
		}
	}

	if !rec.Phys_inventory {
		err = rec.TestField("qty_calculated", types.Decimal{})
		if err != nil {
			return err
		}
		err = rec.TestField("qty_phys_inventory", types.Decimal{})
		if err != nil {
			return err
		}
	} else {
		if rec.Entry_type != gtables.ItemJournalLine_Entry_type.PositiveAdjmt && rec.Entry_type != gtables.ItemJournalLine_Entry_type.NegativeAdjmt {
			return apperrors.ItemJnlPhysEntryType(itemJnlLine, "entry_type", "phys_inventory")
		}
		if rec.Entry_type == gtables.ItemJournalLine_Entry_type.PositiveAdjmt &&
			!rec.Qty_phys_inventory.Sub(rec.Qty_calculated).Equal(rec.Quantity) {
			return apperrors.ItemJnlPhysQty(itemJnlLine, "quantity")
		}
		if rec.Entry_type == gtables.ItemJournalLine_Entry_type.NegativeAdjmt &&
			!rec.Qty_calculated.Sub(rec.Qty_phys_inventory).Equal(rec.Quantity) {
			return apperrors.ItemJnlPhysQty(itemJnlLine, "quantity")
		}
	}
	return nil
}
