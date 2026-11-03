package reservmgt

import (
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/functions"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// ReserveAlways: the line pages' trigger after the Reserve, Quantity or Unit of Measure Code of a
// line is entered (IF Reserve = Reserve::Always THEN AutoReserve(TRUE)) — the page saved the line
// first (CurrForm.SAVERECORD). The pages run it with on_after_validate.
func init() {
	functions.Register("table", gtables.SalesLineTableName, "ReserveAlways", functions.For(func(rec *gtables.SalesLine) (fcodeunits.Result, error) {
		if rec.Reserve != gtables.SalesLine_Reserve.Always {
			return fcodeunits.Result{Success: true}, nil
		}
		return autoReserveAlways(rec, autoReserveLine{
			source: SalesLineSource(rec),
			check: func() error {
				err := rec.TestField("type", gtables.SalesLine_Type.Item)
				if err == nil {
					err = rec.TestField("no")
				}
				return err
			},
			auto: func(s *gtables.ReservationSetup) bool {
				switch rec.Document_type {
				case gtables.SalesLine_Document_type.Quote:
					return s.Auto_sales_quote
				case gtables.SalesLine_Document_type.Order:
					return s.Auto_sales_order
				case gtables.SalesLine_Document_type.Invoice:
					return s.Auto_sales_invoice
				case gtables.SalesLine_Document_type.CreditMemo:
					return s.Auto_sales_credit_memo
				case gtables.SalesLine_Document_type.BlanketOrder:
					return s.Auto_blanket_sales_order
				}
				return false
			},
			reservQuantity: func() (types.Decimal, error) { return SalesLineReservQuantity(rec, false) },
		})
	}), functions.WithDialog())

	functions.Register("table", gtables.PurchaseLineTableName, "ReserveAlways", functions.For(func(rec *gtables.PurchaseLine) (fcodeunits.Result, error) {
		if rec.Reserve != gtables.PurchaseLine_Reserve.Always {
			return fcodeunits.Result{Success: true}, nil
		}
		return autoReserveAlways(rec, autoReserveLine{
			source: PurchLineSource(rec),
			check: func() error {
				err := rec.TestField("type", gtables.PurchaseLine_Type.Item)
				if err == nil {
					err = rec.TestField("no")
				}
				return err
			},
			auto: func(s *gtables.ReservationSetup) bool {
				switch rec.Document_type {
				case gtables.PurchaseLine_Document_type.Quote:
					return s.Auto_purchase_quote
				case gtables.PurchaseLine_Document_type.Order:
					return s.Auto_purchase_order
				case gtables.PurchaseLine_Document_type.Invoice:
					return s.Auto_purchase_invoice
				case gtables.PurchaseLine_Document_type.CreditMemo:
					return s.Auto_purchase_credit_memo
				case gtables.PurchaseLine_Document_type.BlanketOrder:
					return s.Auto_blanket_purchase_order
				}
				return false
			},
			reservQuantity: func() (types.Decimal, error) { return PurchLineReservQuantity(rec, false) },
		})
	}), functions.WithDialog())

	functions.Register("table", gtables.ItemJournalLineTableName, "ReserveAlways", functions.For(func(rec *gtables.ItemJournalLine) (fcodeunits.Result, error) {
		if rec.Reserve != gtables.ItemJournalLine_Reserve.Always {
			return fcodeunits.Result{Success: true}, nil
		}
		return autoReserveAlways(rec, autoReserveLine{
			source: ItemJnlLineSource(rec),
			check:  func() error { return nil },
			auto: func(s *gtables.ReservationSetup) bool {
				switch rec.Entry_type {
				case gtables.ItemJournalLine_Entry_type.Purchase:
					return s.Auto_item_jnl_line_purchases
				case gtables.ItemJournalLine_Entry_type.Sale:
					return s.Auto_item_jnl_line_sales
				case gtables.ItemJournalLine_Entry_type.PositiveAdjmt:
					return s.Auto_item_jnl_line_pos
				case gtables.ItemJournalLine_Entry_type.NegativeAdjmt:
					return s.Auto_item_jnl_line_neg
				case gtables.ItemJournalLine_Entry_type.Transfer:
					return s.Auto_item_jnl_line_transfer
				}
				return false
			},
			reservQuantity: func() (types.Decimal, error) { return ItemJnlLineReservQuantity(rec, false) },
		})
	}), functions.WithDialog())

	functions.Register("table", gtables.BOMJournalLineTableName, "ReserveAlways", functions.For(func(rec *gtables.BOMJournalLine) (fcodeunits.Result, error) {
		if rec.Reserve != gtables.BOMJournalLine_Reserve.Always {
			return fcodeunits.Result{Success: true}, nil
		}
		return autoReserveAlways(rec, autoReserveLine{
			source:         BOMJnlLineSource(rec),
			check:          func() error { return nil },
			auto:           func(s *gtables.ReservationSetup) bool { return s.Auto_bom_journal_line },
			reservQuantity: func() (types.Decimal, error) { return BOMJnlLineReservQuantity(rec, false) },
		})
	}), functions.WithDialog())

	functions.Register("table", gtables.RequisitionLineTableName, "ReserveAlways", functions.For(func(rec *gtables.RequisitionLine) (fcodeunits.Result, error) {
		if rec.Reserve != gtables.RequisitionLine_Reserve.Always {
			return fcodeunits.Result{Success: true}, nil
		}
		return autoReserveAlways(rec, autoReserveLine{
			source: ReqLineSource(rec),
			check: func() error {
				err := rec.TestField("type", gtables.RequisitionLine_Type.Item)
				if err == nil {
					err = rec.TestField("no")
				}
				return err
			},
			auto:           func(s *gtables.ReservationSetup) bool { return s.Auto_requisition_line },
			reservQuantity: func() (types.Decimal, error) { return ReqLineReservQuantity(rec, false) },
		})
	}), functions.WithDialog())
}

// autoReserveLine: what a line's AutoReserve needs — its source, its checks, the setup's Auto
// Reserve of its document or entry type and what it can reserve
type autoReserveLine struct {
	source         Source
	check          func() error
	auto           func(s *gtables.ReservationSetup) bool
	reservQuantity func() (types.Decimal, error)
}

// autoReserveAlways (the lines' AutoReserve(TRUE)): with the setup's Auto Reserve of the line's type
// and a quantity to reserve, the line reserves by the setup's priorities (valid to the setup's
// Valid-to Formula from its date); when not all could be reserved the user may reserve manually
// — the Reservation page opens
func autoReserveAlways(rec ftables.Source, l autoReserveLine) (fcodeunits.Result, error) {
	err := l.check()
	if err != nil {
		return fcodeunits.Result{}, err
	}
	reservSetup := gtables.GetReservationSetup(rec)
	qty, err := l.reservQuantity()
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if l.auto(reservSetup) && qty.IsPositive() {
		var fullAutoReservation bool
		err = inTransaction(rec, func(src ftables.Source) error {
			m := New(src)
			f, err := m.SetReservForSource(l.source)
			if err != nil {
				return err
			}
			fullAutoReservation, err = m.AutoReserve(f.ValidToDate, "", f.ShipmentDate)
			return err
		})
		if err != nil {
			return fcodeunits.Result{}, err
		}
		if !fullAutoReservation && system.GuiAllowed() {
			if system.Confirm(i18n.GetInstance().Message("RESERV_AUTO_NOT_POSSIBLE", fcodeunits.CurrentLanguage())) {
				return withRefresh(openPage(ReservationPageID, Params(l.source))), nil
			}
		}
		return fcodeunits.Result{Success: true, Refresh: true}, nil
	}
	return fcodeunits.Result{Success: true}, nil
}
