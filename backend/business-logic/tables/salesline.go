package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/itemprice"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/resprice"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salestax"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// salesLineLogic is the business logic of table 37 Sales Line.
//
// Orders: the quantities to ship and invoice (all that is outstanding, by default) and the
// shipped/invoiced quantities, which posting fills. A quantity the user enters gives the
// availability warning (codeunit 311) and the credit limit warning (codeunit 312). Not part of
// the version followed: return orders, drop shipments and Fixed Asset lines; the line's Amount
// and Amount Including Tax are calculated, not typed.
type salesLineLogic struct{}

func init() { gtables.RegisterSalesLineLogic(salesLineLogic{}) }

var (
	hundred        = types.NewDecimalFromInt(100)
	roundPrecision = types.MustDecimal("0.00001")
)

// OnInsert: the line's document exists
func (salesLineLogic) OnInsert(rec *gtables.SalesLine) error {
	rec.LockTable()
	salesHeader := gtables.NewSalesHeader(rec)
	return salesHeader.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "no": rec.Document_no})
}

// OnDelete: an order line shipped but not invoiced stays
func (salesLineLogic) OnDelete(rec *gtables.SalesLine) error {
	err := rec.TestField("reserved_qty_base", types.Decimal{})
	if err != nil {
		return err
	}
	if rec.Document_type == gtables.SalesLine_Document_type.Order {
		err = rec.TestField("qty_shipped_not_invoiced", types.Decimal{})
		if err != nil {
			return err
		}
	}
	err = checkSalesBlanketOrderLineDelete(rec)
	if err != nil {
		return err
	}

	// the line's extended text lines go with it
	salesLine2 := gtables.NewSalesLine(rec)
	salesLine2.SetRange("document_type", rec.Document_type)
	salesLine2.SetRange("document_no", rec.Document_no)
	salesLine2.SetRange("attached_to_line_no", rec.Line_no)
	salesLine2.DeleteAll()
	return nil
}

// OnRename: a sales line is never renamed
func (salesLineLogic) OnRename(rec *gtables.SalesLine) error {
	return apperrors.RenameNotAllowed(gtables.SalesLineTableName)
}

// getSalesHeader (GetSalesHeader): the line's document and its currency's rounding (local
// currency: the General Ledger Setup's)
func getSalesHeader(rec *gtables.SalesLine) (*gtables.SalesHeader, *gtables.Currency, error) {
	err := rec.TestField("document_no")
	if err != nil {
		return nil, nil, err
	}
	salesHeader := gtables.NewSalesHeader(rec)
	err = salesHeader.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "no": rec.Document_no})
	if err != nil {
		return nil, nil, err
	}
	currency := gtables.NewCurrency(rec)
	if salesHeader.Currency_code == "" {
		InitRoundingPrecision(currency)
	} else {
		err = currency.GetOrError(salesHeader.Currency_code)
		if err != nil {
			return nil, nil, err
		}
		err = currency.TestField("amount_rounding_precision")
		if err != nil {
			return nil, nil, err
		}
	}
	return salesHeader, currency, nil
}

// salesLineDate (GetDate): the date of the line's currency calculations — the work date on a
// quote, else the posting date
func salesLineDate(salesHeader *gtables.SalesHeader) types.Date {
	if salesHeader.Document_type == gtables.SalesHeader_Document_type.Quote {
		return system.WorkDate()
	} else {
		return salesHeader.Posting_date
	}
}

// OnValidate_Type: a new type starts the line again (not on a line already shipped)
func (salesLineLogic) OnValidate_Type(rec *gtables.SalesLine) error {
	err := testSalesLineNotShipped(rec)
	if err == nil {
		err = rec.TestField("reserved_qty_base", types.Decimal{})
	}
	if err != nil {
		return err
	}
	lineType := rec.Type
	rec.Init()
	rec.Type = lineType
	return nil
}

// OnValidate_No: the line again from the document (customer, currency, dates, dimension 1/2,
// location, price group, posting and tax groups) and the account, item or resource: description,
// unit of measure, unit cost, posting groups, the price and the quantity it had
func (salesLineLogic) OnValidate_No(rec *gtables.SalesLine) error {
	err := testSalesLineNotShipped(rec)
	if err == nil {
		err = rec.TestField("reserved_qty_base", types.Decimal{})
	}
	if err != nil {
		return err
	}
	if rec.No == "" {
		return nil
	}
	xRec := rec.XRec()
	lineType := rec.Type
	no := rec.No
	rec.Init()
	rec.Type = lineType
	rec.No = no
	salesHeader, _, err := getSalesHeader(rec)
	if err != nil {
		return err
	}
	rec.Sell_to_customer_no = salesHeader.Sell_to_customer_no
	rec.Currency_code = salesHeader.Currency_code
	rec.Shipment_date = salesHeader.Shipment_date
	rec.Dimension_1_code = salesHeader.Dimension_1_code
	rec.Dimension_2_code = salesHeader.Dimension_2_code
	rec.Project_no = salesHeader.Project_no
	rec.Location_code = salesHeader.Location_code
	rec.Price_group_code = salesHeader.Price_group_code
	rec.Transaction_type = salesHeader.Transaction_type
	rec.Transport_method = salesHeader.Transport_method
	rec.Bill_to_customer_no = salesHeader.Bill_to_customer_no
	rec.Gen_bus_posting_group = salesHeader.Gen_bus_posting_group
	rec.Tax_bus_posting_group = salesHeader.Tax_bus_posting_group
	rec.Exit_point = salesHeader.Exit_point
	rec.Area = salesHeader.Area
	rec.Transaction_specification = salesHeader.Transaction_specification
	rec.Tax_area_code = salesHeader.Tax_area_code
	rec.Tax_liable = salesHeader.Tax_liable

	switch rec.Type {
	case gtables.SalesLine_Type.Blank:
		standardText := gtables.NewStandardText(rec)
		err = standardText.GetOrError(rec.No)
		if err != nil {
			return err
		}
		rec.Description = standardText.Description
	case gtables.SalesLine_Type.GLAccount:
		glAcc := gtables.NewGLAccount(rec)
		err = glAcc.GetOrError(rec.No)
		if err != nil {
			return err
		}
		err = CheckGLAcc(glAcc)
		if err != nil {
			return err
		}
		err = glAcc.TestField("direct_posting", true)
		if err != nil {
			return err
		}
		rec.Description = types.Text(system.CopyStr(glAcc.Name.String(), 1, rec.MaxStrLen("description")))
		rec.Gen_prod_posting_group = glAcc.Gen_prod_posting_group
		rec.Tax_prod_posting_group = glAcc.Tax_prod_posting_group
		rec.Tax_group_code = glAcc.Tax_group_code
		rec.Allow_invoice_disc = false
	case gtables.SalesLine_Type.Item:
		item := gtables.NewItem(rec)
		err = item.GetOrError(rec.No)
		if err != nil {
			return err
		}
		err = item.TestField("blocked", false)
		if err != nil {
			return err
		}
		err = item.TestField("inventory_posting_group")
		if err != nil {
			return err
		}
		err = item.TestField("gen_prod_posting_group")
		if err != nil {
			return err
		}
		rec.Posting_group = item.Inventory_posting_group
		rec.Quantity_disc_code = types.Code(item.Sales_qty_disc_code)
		rec.Description = item.Description
		rec.Description_2 = item.Description_2
		err = rec.Validate("unit_cost_lcy", item.Unit_cost)
		if err != nil {
			return err
		}
		rec.Allow_invoice_disc = item.Allow_invoice_disc
		rec.Units_per_parcel = item.Units_per_parcel
		rec.Gen_prod_posting_group = item.Gen_prod_posting_group
		rec.Tax_prod_posting_group = item.Tax_prod_posting_group
		rec.Tax_group_code = item.Tax_group_code
		if item.Reserve == gtables.Item_Reserve.Optional {
			rec.Reserve = gtables.SalesLineReserve(salesHeader.Reserve)
		} else {
			rec.Reserve = gtables.SalesLineReserve(item.Reserve)
		}
		if salesHeader.Language_code != "" {
			itemTranslation := gtables.NewItemTranslation(rec)
			if itemTranslation.Get(map[string]interface{}{"item_no": item.No, "variant_code": "", "language_code": salesHeader.Language_code}) {
				rec.Description = itemTranslation.Description
				rec.Description_2 = itemTranslation.Description_2
			}
		}
		err = rec.Validate("unit_of_measure_code", item.Sales_unit_of_measure)
		if err != nil {
			return err
		}
	case gtables.SalesLine_Type.Resource:
		res := gtables.NewResource(rec)
		err = res.GetOrError(rec.No)
		if err != nil {
			return err
		}
		err = res.TestField("blocked", false)
		if err != nil {
			return err
		}
		err = res.TestField("gen_prod_posting_group")
		if err != nil {
			return err
		}
		rec.Description = res.Name
		err = rec.Validate("unit_of_measure_code", res.Unit_of_measure_code)
		if err != nil {
			return err
		}
		rec.Unit_cost_lcy = res.Unit_cost
		rec.Gen_prod_posting_group = res.Gen_prod_posting_group
		rec.Tax_prod_posting_group = res.Tax_prod_posting_group
		rec.Tax_group_code = res.Tax_group_code
		err = findResUnitCost(rec)
		if err != nil {
			return err
		}
	}
	if rec.Type != gtables.SalesLine_Type.Blank {
		err = rec.Validate("tax_prod_posting_group", rec.Tax_prod_posting_group)
		if err != nil {
			return err
		}
		if rec.Type == gtables.SalesLine_Type.Item || rec.Type == gtables.SalesLine_Type.Resource {
			err = updateSalesLineUnitPrice(rec)
			if err != nil {
				return err
			}
		}
		return rec.Validate("quantity", xRec.Quantity)
	}
	return nil
}

// testSalesLineNotShipped: the line's type and number stay once something is shipped
func testSalesLineNotShipped(rec *gtables.SalesLine) error {
	err := rec.TestField("qty_shipped_not_invoiced", types.Decimal{})
	if err != nil {
		return err
	}
	return rec.TestField("quantity_shipped", types.Decimal{})
}

// OnValidate_Quantity: Quantity (Base) — not below what is shipped or invoiced —, the
// outstanding quantity and amount, the quantities to ship and invoice, and the item's quantity
// discount for the base quantity (when the document and the line allow it)
func (salesLineLogic) OnValidate_Quantity(rec *gtables.SalesLine) error {
	quantityBase, err := calcSalesLineBaseQty(rec, rec.Quantity)
	if err != nil {
		return err
	}
	rec.Quantity_base = quantityBase
	if rec.Quantity.Mul(rec.Quantity_shipped).IsNegative() || rec.Quantity.Abs().LessThan(rec.Quantity_shipped.Abs()) {
		return apperrors.SalesQtyLessThan(gtables.SalesLineTableName, "quantity", "quantity_shipped")
	}
	if rec.Quantity_base.Mul(rec.Qty_shipped_base).IsNegative() || rec.Quantity_base.Abs().LessThan(rec.Qty_shipped_base.Abs()) {
		return apperrors.SalesQtyLessThan(gtables.SalesLineTableName, "quantity_base", "qty_shipped_base")
	}
	if rec.Quantity.Mul(rec.Quantity_invoiced).IsNegative() || rec.Quantity.Abs().LessThan(rec.Quantity_invoiced.Abs()) {
		return apperrors.SalesQtyLessThan(gtables.SalesLineTableName, "quantity", "quantity_invoiced")
	}
	if rec.Quantity_base.Mul(rec.Qty_invoiced_base).IsNegative() || rec.Quantity_base.Abs().LessThan(rec.Qty_invoiced_base.Abs()) {
		return apperrors.SalesQtyLessThan(gtables.SalesLineTableName, "quantity_base", "qty_invoiced_base")
	}
	err = initSalesLineOutstanding(rec)
	if err == nil {
		err = checkReservedNotAbove(gtables.SalesLineTableName, "reserved_qty_base", rec.Outstanding_qty_base, rec.Reserved_qty_base)
	}
	if err != nil {
		return err
	}
	initSalesLineQtyToShip(rec)
	if rec.Reserve != gtables.SalesLine_Reserve.Always {
		err = checkSalesLineItemAvailable(rec)
		if err != nil {
			return err
		}
	}
	rec.Quantity_disc_percent = types.Decimal{}
	if rec.Type == gtables.SalesLine_Type.Item {
		salesHeader, _, err := getSalesHeader(rec)
		if err != nil {
			return err
		}
		if salesHeader.Allow_quantity_disc && rec.Allow_quantity_disc {
			itemSalesQtyDisc := gtables.NewItemSalesQtyDisc(rec)
			itemSalesQtyDisc.SetRange("code", rec.Quantity_disc_code)
			itemSalesQtyDisc.SetRange("minimum_quantity", types.Decimal{}, rec.Quantity_base.Abs())
			if itemSalesQtyDisc.FindLast() {
				rec.Quantity_disc_percent = itemSalesQtyDisc.Discount_percent
			}
		}
	}
	return validateSalesLineQuantityDisc(rec)
}

// calcSalesLineBaseQty (CalcBaseQty): a quantity in the base unit of measure
func calcSalesLineBaseQty(rec *gtables.SalesLine, qty types.Decimal) (types.Decimal, error) {
	err := rec.TestField("qty_per_unit_of_measure")
	if err != nil {
		return types.Decimal{}, err
	}
	return system.Round(qty.Mul(rec.Qty_per_unit_of_measure), roundPrecision), nil
}

// initSalesLineOutstanding (InitOutstanding): what is not shipped yet is outstanding, what is
// shipped and not invoiced yet is shipped not invoiced
func initSalesLineOutstanding(rec *gtables.SalesLine) error {
	rec.Outstanding_quantity = rec.Quantity.Sub(rec.Quantity_shipped)
	rec.Outstanding_qty_base = rec.Quantity_base.Sub(rec.Qty_shipped_base)
	rec.Qty_shipped_not_invoiced = rec.Quantity_shipped.Sub(rec.Quantity_invoiced)
	rec.Qty_shpd_not_invd_base = rec.Qty_shipped_base.Sub(rec.Qty_invoiced_base)
	return initSalesLineOutstandingAmount(rec)
}

// initSalesLineOutstandingAmount (InitOutstandingAmount): the outstanding and the shipped not
// invoiced shares of Amount Including Tax, in the document's currency and in LCY
func initSalesLineOutstandingAmount(rec *gtables.SalesLine) error {
	if rec.Quantity.IsZero() {
		rec.Outstanding_amount = types.Decimal{}
		rec.Outstanding_amount_lcy = types.Decimal{}
		rec.Shipped_not_invoiced = types.Decimal{}
		rec.Shipped_not_invoiced_lcy = types.Decimal{}
	} else {
		salesHeader, currency, err := getSalesHeader(rec)
		if err != nil {
			return err
		}
		rec.Outstanding_amount = system.Round(rec.Amount_including_tax.Mul(rec.Outstanding_quantity).Div(rec.Quantity), currency.Amount_rounding_precision)
		err = validateSalesLineOutstandingAmount(rec, salesHeader)
		if err != nil {
			return err
		}
		rec.Shipped_not_invoiced = system.Round(rec.Amount_including_tax.Mul(rec.Qty_shipped_not_invoiced).Div(rec.Quantity), currency.Amount_rounding_precision)
		return validateSalesLineShippedNotInvoiced(rec, salesHeader)
	}
	return nil
}

// validateSalesLineOutstandingAmount (Outstanding Amount OnValidate): Outstanding Amount (LCY)
func validateSalesLineOutstandingAmount(rec *gtables.SalesLine, salesHeader *gtables.SalesHeader) error {
	amountLCY, err := salesLineAmountLCY(rec, salesHeader, rec.Outstanding_amount)
	if err != nil {
		return err
	}
	rec.Outstanding_amount_lcy = amountLCY
	return nil
}

// validateSalesLineShippedNotInvoiced (Shipped Not Invoiced OnValidate): Shipped Not Invoiced
// (LCY)
func validateSalesLineShippedNotInvoiced(rec *gtables.SalesLine, salesHeader *gtables.SalesHeader) error {
	amountLCY, err := salesLineAmountLCY(rec, salesHeader, rec.Shipped_not_invoiced)
	if err != nil {
		return err
	}
	rec.Shipped_not_invoiced_lcy = amountLCY
	return nil
}

// salesLineAmountLCY: an amount of the line in LCY at the document's currency factor, rounded
// to the local currency
func salesLineAmountLCY(rec *gtables.SalesLine, salesHeader *gtables.SalesHeader, amount types.Decimal) (types.Decimal, error) {
	currency2 := gtables.NewCurrency(rec)
	InitRoundingPrecision(currency2)
	if salesHeader.Currency_code != "" {
		amountLCY, err := ExchangeRates(rec).ExchangeAmtFCYToLCY(salesLineDate(salesHeader), rec.Currency_code, amount, salesHeader.Currency_factor)
		if err != nil {
			return types.Decimal{}, err
		}
		return system.Round(amountLCY, currency2.Amount_rounding_precision), nil
	} else {
		return system.Round(amount, currency2.Amount_rounding_precision), nil
	}
}

// initSalesLineQtyToShip (InitQtyToShip): all that is outstanding is to be shipped
func initSalesLineQtyToShip(rec *gtables.SalesLine) {
	rec.Qty_to_ship = rec.Outstanding_quantity
	rec.Qty_to_ship_base = rec.Outstanding_qty_base
	initSalesLineQtyToInvoice(rec)
}

// initSalesLineQtyToInvoice (InitQtyToInvoice): all that can be invoiced is to be invoiced
func initSalesLineQtyToInvoice(rec *gtables.SalesLine) {
	rec.Qty_to_invoice = maxSalesLineQtyToInvoice(rec)
	rec.Qty_to_invoice_base = maxSalesLineQtyToInvoiceBase(rec)
}

// maxSalesLineQtyToInvoice (MaxQtyToInvoice): what is shipped or to be shipped, less what is
// invoiced
func maxSalesLineQtyToInvoice(rec *gtables.SalesLine) types.Decimal {
	return rec.Quantity_shipped.Add(rec.Qty_to_ship).Sub(rec.Quantity_invoiced)
}

// maxSalesLineQtyToInvoiceBase (MaxQtyToInvoiceBase): the same in the base unit of measure
func maxSalesLineQtyToInvoiceBase(rec *gtables.SalesLine) types.Decimal {
	return rec.Qty_shipped_base.Add(rec.Qty_to_ship_base).Sub(rec.Qty_invoiced_base)
}

// OnValidate_Qty_to_ship: its base quantity and the quantity to invoice; not more than is
// outstanding (and of the same sign)
func (salesLineLogic) OnValidate_Qty_to_ship(rec *gtables.SalesLine) error {
	qtyToShipBase, err := calcSalesLineBaseQty(rec, rec.Qty_to_ship)
	if err != nil {
		return err
	}
	rec.Qty_to_ship_base = qtyToShipBase
	initSalesLineQtyToInvoice(rec)
	if qtyToHandleAbove(rec.Qty_to_ship, rec.Quantity, rec.Outstanding_quantity) {
		return apperrors.SalesCannotShipMore(system.Format(rec.Outstanding_quantity), false)
	}
	if qtyToHandleAbove(rec.Qty_to_ship_base, rec.Quantity_base, rec.Outstanding_qty_base) {
		return apperrors.SalesCannotShipMore(system.Format(rec.Outstanding_qty_base), true)
	}
	return nil
}

// qtyToHandleAbove: a quantity to handle of another sign than the line's quantity, or above
// the most that can be handled (or that one of another sign)
func qtyToHandleAbove(qtyToHandle, quantity, maxQty types.Decimal) bool {
	if qtyToHandle.IsNegative() != quantity.IsNegative() && !quantity.IsZero() && !qtyToHandle.IsZero() {
		return true
	}
	if qtyToHandle.Abs().GreaterThan(maxQty.Abs()) {
		return true
	}
	return quantity.IsNegative() != maxQty.IsNegative() && !quantity.IsZero() && !maxQty.IsZero()
}

// OnValidate_Qty_to_ship_base: only in the base unit of measure; Qty. to Ship follows
func (salesLineLogic) OnValidate_Qty_to_ship_base(rec *gtables.SalesLine) error {
	err := rec.TestField("qty_per_unit_of_measure", types.NewDecimalFromInt(1))
	if err != nil {
		return err
	}
	return rec.Validate("qty_to_ship", rec.Qty_to_ship_base)
}

// OnValidate_Qty_to_invoice: its base quantity; not more than is shipped or to be shipped and
// not invoiced (and of the line's sign)
func (salesLineLogic) OnValidate_Qty_to_invoice(rec *gtables.SalesLine) error {
	if rec.Qty_to_invoice.Equal(maxSalesLineQtyToInvoice(rec)) {
		initSalesLineQtyToInvoice(rec)
	} else {
		qtyToInvoiceBase, err := calcSalesLineBaseQty(rec, rec.Qty_to_invoice)
		if err != nil {
			return err
		}
		rec.Qty_to_invoice_base = qtyToInvoiceBase
	}
	maxQtyToInvoice := maxSalesLineQtyToInvoice(rec)
	if rec.Qty_to_invoice.Mul(rec.Quantity).IsNegative() || rec.Qty_to_invoice.Abs().GreaterThan(maxQtyToInvoice.Abs()) {
		return apperrors.SalesCannotInvoiceMore(system.Format(maxQtyToInvoice), false)
	}
	maxQtyToInvoiceBase := maxSalesLineQtyToInvoiceBase(rec)
	if rec.Qty_to_invoice_base.Mul(rec.Quantity_base).IsNegative() || rec.Qty_to_invoice_base.Abs().GreaterThan(maxQtyToInvoiceBase.Abs()) {
		return apperrors.SalesCannotInvoiceMore(system.Format(maxQtyToInvoiceBase), true)
	}
	return nil
}

// OnValidate_Qty_to_invoice_base: only in the base unit of measure; Qty. to Invoice follows
func (salesLineLogic) OnValidate_Qty_to_invoice_base(rec *gtables.SalesLine) error {
	err := rec.TestField("qty_per_unit_of_measure", types.NewDecimalFromInt(1))
	if err != nil {
		return err
	}
	return rec.Validate("qty_to_invoice", rec.Qty_to_invoice_base)
}

// OnValidate_Unit_price: the discount and the amounts again
func (salesLineLogic) OnValidate_Unit_price(rec *gtables.SalesLine) error {
	return validateSalesLineDiscountPercent(rec)
}

// OnValidate_Unit_cost_lcy: Unit Cost in the document's currency
func (salesLineLogic) OnValidate_Unit_cost_lcy(rec *gtables.SalesLine) error {
	salesHeader, currency, err := getSalesHeader(rec)
	if err != nil {
		return err
	}
	if salesHeader.Currency_code != "" {
		err = currency.TestField("unit_amount_rounding_precision")
		if err != nil {
			return err
		}
		unitCost, err := ExchangeRates(rec).ExchangeAmtLCYToFCY(salesLineDate(salesHeader), salesHeader.Currency_code, rec.Unit_cost_lcy, salesHeader.Currency_factor)
		if err != nil {
			return err
		}
		rec.Unit_cost = system.Round(unitCost, currency.Unit_amount_rounding_precision)
	} else {
		rec.Unit_cost = rec.Unit_cost_lcy
	}
	return nil
}

// validateSalesLineQuantityDisc (Quantity Disc. % OnValidate): Line Discount % from the quantity
// and customer/item discounts, added or multiplied as the setup says (at most 100)
func validateSalesLineQuantityDisc(rec *gtables.SalesLine) error {
	salesSetup := gtables.GetSalesReceivablesSetup(rec)
	if salesSetup.Line_discount_calculation == gtables.SalesReceivablesSetup_Line_discount_calculation.QtyDiscCustItemDisc {
		rec.Line_discount_percent = rec.Cust_item_disc_percent.Add(rec.Quantity_disc_percent)
	} else {
		one := types.NewDecimalFromInt(1)
		remaining := one.Sub(rec.Cust_item_disc_percent.Div(hundred)).Mul(one.Sub(rec.Quantity_disc_percent.Div(hundred)))
		rec.Line_discount_percent = system.Round(hundred.Sub(hundred.Mul(remaining)), roundPrecision)
	}
	if rec.Line_discount_percent.GreaterThan(hundred) {
		rec.Line_discount_percent = hundred
	}
	return validateSalesLineDiscountPercent(rec)
}

// OnValidate_Line_discount_percent: Line Discount Amount from the percent, the amounts again
func (salesLineLogic) OnValidate_Line_discount_percent(rec *gtables.SalesLine) error {
	return validateSalesLineDiscountPercent(rec)
}

// validateSalesLineDiscountPercent (Line Discount % OnValidate)
func validateSalesLineDiscountPercent(rec *gtables.SalesLine) error {
	originalAmount, currency, err := calcSalesLineOriginalAmount(rec)
	if err != nil {
		return err
	}
	rec.Line_discount_amount = system.Round(originalAmount.Mul(rec.Line_discount_percent).Div(hundred), currency.Amount_rounding_precision)
	rec.Inv_discount_amount = types.Decimal{}
	return UpdateSalesLineAmounts(rec)
}

// OnValidate_Line_discount_amount: Line Discount % from the amount, the amounts again
func (salesLineLogic) OnValidate_Line_discount_amount(rec *gtables.SalesLine) error {
	err := rec.TestField("quantity")
	if err != nil {
		return err
	}
	if !rec.Unit_price.IsZero() {
		originalAmount, _, err := calcSalesLineOriginalAmount(rec)
		if err != nil {
			return err
		}
		rec.Line_discount_percent = system.Round(rec.Line_discount_amount.Div(originalAmount).Mul(hundred), roundPrecision)
	} else {
		rec.Line_discount_percent = types.Decimal{}
	}
	rec.Inv_discount_amount = types.Decimal{}
	return UpdateSalesLineAmounts(rec)
}

// OnValidate_Allow_invoice_disc: a line no longer allowing it loses its invoice discount
func (salesLineLogic) OnValidate_Allow_invoice_disc(rec *gtables.SalesLine) error {
	xRec := rec.XRec()
	if rec.Allow_invoice_disc != xRec.Allow_invoice_disc && !rec.Allow_invoice_disc {
		rec.Inv_discount_amount = types.Decimal{}
		return updateSalesLineAmounts(rec, false)
	}
	return nil
}

// OnValidate_Inv_discount_amount: the amounts again (Sales-Calc. Discount sets it)
func (salesLineLogic) OnValidate_Inv_discount_amount(rec *gtables.SalesLine) error {
	err := rec.TestField("quantity")
	if err != nil {
		return err
	}
	return UpdateSalesLineAmounts(rec)
}

// OnValidate_Price_group_code: an item's price again
func (salesLineLogic) OnValidate_Price_group_code(rec *gtables.SalesLine) error {
	if rec.Type == gtables.SalesLine_Type.Item {
		err := updateSalesLineUnitPrice(rec)
		if err != nil {
			return err
		}
		return rec.Validate("quantity", rec.Quantity)
	}
	return nil
}

// OnValidate_Work_type_code: a resource's price and cost for the work type, and its unit of
// measure
// OnValidate_Project_no: a reserved line keeps its project; the unit price for the project
func (salesLineLogic) OnValidate_Project_no(rec *gtables.SalesLine) error {
	rec.CalcFields("reserved_qty_base")
	err := rec.TestField("reserved_qty_base", types.Decimal{})
	if err != nil {
		return err
	}
	return updateSalesLineUnitPrice(rec)
}

func (salesLineLogic) OnValidate_Work_type_code(rec *gtables.SalesLine) error {
	if rec.Type == gtables.SalesLine_Type.Resource {
		err := updateSalesLineUnitPrice(rec)
		if err != nil {
			return err
		}
		err = rec.Validate("unit_price", rec.Unit_price)
		if err != nil {
			return err
		}
		err = findResUnitCost(rec)
		if err != nil {
			return err
		}
		workType := gtables.NewWorkType(rec)
		if workType.Get(rec.Work_type_code) {
			return rec.Validate("unit_of_measure_code", workType.Unit_of_measure_code)
		}
	}
	return nil
}

// OnValidate_Cust_item_disc_percent: Line Discount % again
func (salesLineLogic) OnValidate_Cust_item_disc_percent(rec *gtables.SalesLine) error {
	if rec.Type == gtables.SalesLine_Type.Item {
		return validateSalesLineQuantityDisc(rec)
	}
	return nil
}

// OnValidate_Gen_bus_posting_group: its default tax business posting group
func (salesLineLogic) OnValidate_Gen_bus_posting_group(rec *gtables.SalesLine) error {
	xRec := rec.XRec()
	if xRec.Gen_bus_posting_group != rec.Gen_bus_posting_group {
		genBusPostingGroup := gtables.NewGenBusinessPostingGroup(rec)
		autoInsert, err := ValidateTaxBusPostingGroup(genBusPostingGroup, rec.Gen_bus_posting_group)
		if err != nil {
			return err
		}
		if autoInsert {
			return rec.Validate("tax_bus_posting_group", genBusPostingGroup.Def_tax_bus_posting_group)
		}
	}
	return nil
}

// OnValidate_Gen_prod_posting_group: its default tax product posting group
func (salesLineLogic) OnValidate_Gen_prod_posting_group(rec *gtables.SalesLine) error {
	xRec := rec.XRec()
	if xRec.Gen_prod_posting_group != rec.Gen_prod_posting_group {
		genProdPostingGroup := gtables.NewGenProductPostingGroup(rec)
		autoInsert, err := ValidateTaxProdPostingGroup(genProdPostingGroup, rec.Gen_prod_posting_group)
		if err != nil {
			return err
		}
		if autoInsert {
			return rec.Validate("tax_prod_posting_group", genProdPostingGroup.Def_tax_prod_posting_group)
		}
	}
	return nil
}

// Tax Area Code, Tax Liable, Tax Group Code: the amounts again
func (salesLineLogic) OnValidate_Tax_area_code(rec *gtables.SalesLine) error {
	return UpdateSalesLineAmounts(rec)
}

func (salesLineLogic) OnValidate_Tax_liable(rec *gtables.SalesLine) error {
	return UpdateSalesLineAmounts(rec)
}

func (salesLineLogic) OnValidate_Tax_group_code(rec *gtables.SalesLine) error {
	return UpdateSalesLineAmounts(rec)
}

// OnValidate_Tax_bus_posting_group: the tax setup again
func (salesLineLogic) OnValidate_Tax_bus_posting_group(rec *gtables.SalesLine) error {
	return rec.Validate("tax_prod_posting_group", rec.Tax_prod_posting_group)
}

// OnValidate_Tax_prod_posting_group: Tax % and Tax Calculation Type from the Tax Posting Setup
// (none: Normal VAT 0 %), then the amounts
func (salesLineLogic) OnValidate_Tax_prod_posting_group(rec *gtables.SalesLine) error {
	taxPostingSetup := gtables.NewTaxPostingSetup(rec)
	if !taxPostingSetup.Get(map[string]interface{}{"tax_bus_posting_group": rec.Tax_bus_posting_group, "tax_prod_posting_group": rec.Tax_prod_posting_group}) {
		taxPostingSetup.Init()
	}
	rec.Tax_percent = taxPostingSetup.Vat_percent
	rec.Tax_calculation_type = gtables.SalesLineTax_calculation_type(taxPostingSetup.Tax_calculation_type)
	switch rec.Tax_calculation_type {
	case gtables.SalesLine_Tax_calculation_type.ReverseChargeVAT, gtables.SalesLine_Tax_calculation_type.SalesTax:
		rec.Tax_percent = types.Decimal{}
	case gtables.SalesLine_Tax_calculation_type.FullVAT:
		err := rec.TestField("type", gtables.SalesLine_Type.GLAccount)
		if err != nil {
			return err
		}
		err = taxPostingSetup.TestField("sales_vat_account")
		if err != nil {
			return err
		}
		err = rec.TestField("no", taxPostingSetup.Sales_vat_account)
		if err != nil {
			return err
		}
	}
	return UpdateSalesLineAmounts(rec)
}

// OnValidate_Variant_code: the variant's description (translated by the document's language)
func (salesLineLogic) OnValidate_Variant_code(rec *gtables.SalesLine) error {
	if rec.Variant_code == "" {
		return nil
	}
	err := rec.TestField("type", gtables.SalesLine_Type.Item)
	if err != nil {
		return err
	}
	itemVariant := gtables.NewItemVariant(rec)
	err = itemVariant.GetOrError(map[string]interface{}{"item_no": rec.No, "code": rec.Variant_code})
	if err != nil {
		return err
	}
	rec.Description = itemVariant.Description
	rec.Description_2 = itemVariant.Description_2
	err = checkSalesLineItemAvailable(rec)
	if err != nil {
		return err
	}
	salesHeader, _, err := getSalesHeader(rec)
	if err != nil {
		return err
	}
	if salesHeader.Language_code != "" {
		itemTranslation := gtables.NewItemTranslation(rec)
		if itemTranslation.Get(map[string]interface{}{"item_no": rec.No, "variant_code": rec.Variant_code, "language_code": salesHeader.Language_code}) {
			rec.Description = itemTranslation.Description
			rec.Description_2 = itemTranslation.Description_2
		}
	}
	return nil
}

// OnValidate_Unit_of_measure_code: the unit's description (translated by the document's
// language); an item's quantity per unit, weights, volume, unit cost and price for the unit;
// the quantity again
func (salesLineLogic) OnValidate_Unit_of_measure_code(rec *gtables.SalesLine) error {
	if rec.Unit_of_measure_code == "" {
		rec.Unit_of_measure = ""
	} else {
		unitOfMeasure := gtables.NewUnitOfMeasure(rec)
		if !unitOfMeasure.Get(rec.Unit_of_measure_code) {
			unitOfMeasure.Init()
		}
		rec.Unit_of_measure = types.Text(system.CopyStr(unitOfMeasure.Description.String(), 1, rec.MaxStrLen("unit_of_measure")))
		salesHeader, _, err := getSalesHeader(rec)
		if err != nil {
			return err
		}
		if salesHeader.Language_code != "" {
			unitOfMeasureTranslation := gtables.NewUnitOfMeasureTranslation(rec)
			unitOfMeasureTranslation.SetRange("code", rec.Unit_of_measure_code)
			unitOfMeasureTranslation.SetRange("language_code", salesHeader.Language_code)
			if unitOfMeasureTranslation.FindFirst() {
				rec.Unit_of_measure = types.Text(system.CopyStr(unitOfMeasureTranslation.Description.String(), 1, rec.MaxStrLen("unit_of_measure")))
			}
		}
	}
	if rec.Type == gtables.SalesLine_Type.Item {
		item := gtables.NewItem(rec)
		err := item.GetOrError(rec.No)
		if err != nil {
			return err
		}
		rec.Qty_per_unit_of_measure, err = GetQtyPerUnitOfMeasure(item, rec.Unit_of_measure_code)
		if err != nil {
			return err
		}
		rec.Gross_weight = item.Gross_weight.Mul(rec.Qty_per_unit_of_measure)
		rec.Net_weight = item.Net_weight.Mul(rec.Qty_per_unit_of_measure)
		rec.Unit_volume = item.Unit_volume.Mul(rec.Qty_per_unit_of_measure)
		err = rec.Validate("unit_cost_lcy", item.Unit_cost.Mul(rec.Qty_per_unit_of_measure))
		if err != nil {
			return err
		}
		err = updateSalesLineUnitPrice(rec)
		if err != nil {
			return err
		}
	} else {
		rec.Qty_per_unit_of_measure = types.NewDecimalFromInt(1)
	}
	err := rec.Validate("quantity", rec.Quantity)
	if err != nil {
		return err
	}
	return rec.Validate("reserved_qty_base", rec.Reserved_qty_base)
}

// OnValidate_Quantity_base: only in the base unit of measure; the quantity follows
func (salesLineLogic) OnValidate_Quantity_base(rec *gtables.SalesLine) error {
	err := rec.TestField("qty_per_unit_of_measure", types.NewDecimalFromInt(1))
	if err != nil {
		return err
	}
	return rec.Validate("quantity", rec.Quantity_base)
}

// updateSalesLineUnitPrice (UpdateUnitPrice): an item's price (Item-Find Price for the price
// group, currency, unit of measure and the order or posting date) with its discount settings
// and the customer/item discount, a resource's price (Resource-Find Price) — converted to the
// document's currency, per unit of measure, with or without tax as the document's prices are,
// rounded to the currency's unit-amount precision
func updateSalesLineUnitPrice(rec *gtables.SalesLine) error {
	err := rec.TestField("qty_per_unit_of_measure")
	if err != nil {
		return err
	}
	salesHeader, currency, err := getSalesHeader(rec)
	if err != nil {
		return err
	}
	priceInCurrency := false
	itemPriceInclTax := false
	item := gtables.NewItem(rec)
	itemPrice := gtables.NewItemPrice(rec)
	switch rec.Type {
	case gtables.SalesLine_Type.Item:
		err = item.GetOrError(rec.No)
		if err != nil {
			return err
		}
		itemPrice.Item_no = rec.No
		itemPrice.Price_group_code = rec.Price_group_code
		itemPrice.Currency_code = salesHeader.Currency_code
		itemPrice.Unit_of_measure_code = rec.Unit_of_measure_code
		if salesHeader.Document_type == gtables.SalesHeader_Document_type.Quote || salesHeader.Document_type == gtables.SalesHeader_Document_type.Order {
			itemPrice.Starting_date = salesHeader.Order_date
		} else {
			itemPrice.Starting_date = salesHeader.Posting_date
		}
		err = itemprice.FindPrice(itemPrice)
		if err != nil {
			return err
		}
		rec.Unit_price = itemPrice.Unit_price
		item.Tax_bus_posting_gr_price = itemPrice.Tax_bus_posting_gr_price
		rec.Allow_quantity_disc = itemPrice.Allow_quantity_disc
		rec.Allow_invoice_disc = itemPrice.Allow_invoice_disc
		if itemPrice.Allow_cust_item_disc {
			custItemDisc := gtables.NewCustItemDiscount(rec)
			if custItemDisc.Get(map[string]interface{}{"customer_disc_group": salesHeader.Cust_item_disc_gr, "item_discount_group": item.Item_cust_disc_gr}) {
				rec.Cust_item_disc_percent = custItemDisc.Discount_percent
			}
		} else {
			rec.Cust_item_disc_percent = types.Decimal{}
		}
		priceInCurrency = itemPrice.Currency_code != ""
		itemPriceInclTax = itemPrice.Price_includes_tax
	case gtables.SalesLine_Type.Resource:
		resPrice := gtables.NewResourcePrice(rec)
		resPrice.Project_no = rec.Project_no
		resPrice.Code = rec.No
		resPrice.Work_type_code = rec.Work_type_code
		resPrice.Currency_code = rec.Currency_code
		err = resprice.FindResPrice(resPrice)
		if err != nil {
			return err
		}
		rec.Unit_price = resPrice.Unit_price
		priceInCurrency = resPrice.Currency_code != ""
	}
	if salesHeader.Currency_code != "" && !priceInCurrency {
		err = salesHeader.TestField("currency_factor")
		if err != nil {
			return err
		}
		rec.Unit_price, err = ExchangeRates(rec).ExchangeAmtLCYToFCY(salesLineDate(salesHeader), rec.Currency_code, rec.Unit_price, salesHeader.Currency_factor)
		if err != nil {
			return err
		}
	}
	if itemPrice.Unit_of_measure_code == "" {
		rec.Unit_price = rec.Unit_price.Mul(rec.Qty_per_unit_of_measure)
	}
	if itemPriceInclTax {
		taxPostingSetup := gtables.NewTaxPostingSetup(rec)
		if !taxPostingSetup.Get(map[string]interface{}{"tax_bus_posting_group": item.Tax_bus_posting_gr_price, "tax_prod_posting_group": item.Tax_prod_posting_group}) {
			taxPostingSetup.Init()
		}
		switch taxPostingSetup.Tax_calculation_type {
		case gtables.TaxPostingSetup_Tax_calculation_type.ReverseChargeVAT:
			taxPostingSetup.Vat_percent = types.Decimal{}
		case gtables.TaxPostingSetup_Tax_calculation_type.SalesTax:
			return apperrors.ItemPriceInclTaxSalesTax(gtables.TaxPostingSetupTableName, "tax_calculation_type", taxPostingSetup.Tax_calculation_type)
		}
		one := types.NewDecimalFromInt(1)
		switch rec.Tax_calculation_type {
		case gtables.SalesLine_Tax_calculation_type.NormalVAT, gtables.SalesLine_Tax_calculation_type.FullVAT, gtables.SalesLine_Tax_calculation_type.SalesTax:
			if salesHeader.Prices_including_tax {
				if rec.Tax_bus_posting_group != item.Tax_bus_posting_gr_price {
					rec.Unit_price = rec.Unit_price.Mul(hundred.Add(rec.Tax_percent)).Div(hundred.Add(taxPostingSetup.Vat_percent))
				}
			} else {
				rec.Unit_price = rec.Unit_price.Div(one.Add(taxPostingSetup.Vat_percent.Div(hundred)))
			}
		case gtables.SalesLine_Tax_calculation_type.ReverseChargeVAT:
			rec.Unit_price = rec.Unit_price.Div(one.Add(taxPostingSetup.Vat_percent.Div(hundred)))
		}
	} else {
		if salesHeader.Prices_including_tax {
			rec.Unit_price = rec.Unit_price.Mul(types.NewDecimalFromInt(1).Add(rec.Tax_percent.Div(hundred)))
		}
	}
	err = currency.TestField("unit_amount_rounding_precision")
	if err != nil {
		return err
	}
	rec.Unit_price = system.Round(rec.Unit_price, currency.Unit_amount_rounding_precision)
	return nil
}

// findResUnitCost (FindResUnitCost): the resource's cost for the work type (Resource-Find Cost)
func findResUnitCost(rec *gtables.SalesLine) error {
	resCost := gtables.NewResourceCost(rec)
	resCost.Code = rec.No
	resCost.Work_type_code = rec.Work_type_code
	err := resprice.FindResCost(resCost)
	if err != nil {
		return err
	}
	return rec.Validate("unit_cost_lcy", resCost.Unit_cost)
}

// UpdateSalesLineAmounts (UpdateAmounts): Amount after the line and invoice discounts, the tax
// (VAT % or sales tax of the line's tax area and group), Amount Including Tax and the
// outstanding amount
func UpdateSalesLineAmounts(rec *gtables.SalesLine) error {
	return updateSalesLineAmounts(rec, true)
}

func updateSalesLineAmounts(rec *gtables.SalesLine, testType bool) error {
	if testType {
		err := rec.TestField("type")
		if err != nil {
			return err
		}
	}
	salesHeader, _, err := getSalesHeader(rec)
	if err != nil {
		return err
	}
	originalAmount, _, err := calcSalesLineOriginalAmount(rec)
	if err != nil {
		return err
	}
	switch rec.Tax_calculation_type {
	case gtables.SalesLine_Tax_calculation_type.NormalVAT, gtables.SalesLine_Tax_calculation_type.ReverseChargeVAT:
		rec.Amount = system.Round(originalAmount.Sub(rec.Line_discount_amount).Sub(rec.Inv_discount_amount), roundPrecision)
		rec.Tax_base_amount = system.Round(rec.Amount, roundPrecision)
		rec.Amount_including_tax = system.Round(rec.Amount.Add(rec.Tax_base_amount.Mul(rec.Tax_percent).Div(hundred)), roundPrecision)
	case gtables.SalesLine_Tax_calculation_type.FullVAT:
		rec.Amount = types.Decimal{}
		rec.Tax_base_amount = types.Decimal{}
		rec.Amount_including_tax = system.Round(originalAmount)
	case gtables.SalesLine_Tax_calculation_type.SalesTax:
		rec.Amount = system.Round(originalAmount.Sub(rec.Line_discount_amount).Sub(rec.Inv_discount_amount), roundPrecision)
		rec.Tax_base_amount = system.Round(rec.Amount, roundPrecision)
		salesTaxCalculate := salestax.New(rec)
		taxAmount, err := salesTaxCalculate.CalculateTax(rec.Tax_area_code, rec.Tax_group_code, rec.Tax_liable, salesHeader.Posting_date, rec.Tax_base_amount, rec.Quantity, salesHeader.Currency_factor)
		if err != nil {
			return err
		}
		rec.Amount_including_tax = rec.Amount.Add(taxAmount)
		if !rec.Tax_base_amount.IsZero() {
			rec.Tax_percent = system.Round(hundred.Mul(rec.Amount_including_tax.Sub(rec.Tax_base_amount)).Div(rec.Tax_base_amount), roundPrecision)
		} else {
			rec.Tax_percent = types.Decimal{}
		}
	}
	err = initSalesLineOutstandingAmount(rec)
	if err != nil {
		return err
	}
	if fcodeunits.CurrFieldNo() != 0 && rec.Document_type <= gtables.SalesLine_Document_type.Invoice &&
		rec.Outstanding_amount.Add(rec.Shipped_not_invoiced).IsPositive() {
		return CustCheckCrLimitSalesLine(rec)
	}
	return nil
}

// calcSalesLineOriginalAmount (CalcOriginalAmount): Quantity × Unit Price — without the tax when
// the document's prices include it (Full VAT: as it is) — rounded to the currency
func calcSalesLineOriginalAmount(rec *gtables.SalesLine) (types.Decimal, *gtables.Currency, error) {
	salesHeader, currency, err := getSalesHeader(rec)
	if err != nil {
		return types.Decimal{}, nil, err
	}
	originalAmount := rec.Quantity.Mul(rec.Unit_price)
	if salesHeader.Prices_including_tax {
		switch rec.Tax_calculation_type {
		case gtables.SalesLine_Tax_calculation_type.NormalVAT, gtables.SalesLine_Tax_calculation_type.ReverseChargeVAT, gtables.SalesLine_Tax_calculation_type.SalesTax:
			originalAmount = rec.Quantity.Mul(rec.Unit_price).Div(types.NewDecimalFromInt(1).Add(rec.Tax_percent.Div(hundred)))
		case gtables.SalesLine_Tax_calculation_type.FullVAT:
			originalAmount = rec.Quantity.Mul(rec.Unit_price)
		}
	} else {
		originalAmount = rec.Quantity.Mul(rec.Unit_price)
	}
	return system.Round(originalAmount, currency.Amount_rounding_precision), currency, nil
}

// InitSalesLineOutstanding (InitOutstanding) for sales posting: the outstanding and
// shipped-not-invoiced quantities and amounts after the line's shipped/invoiced quantities
// changed
func InitSalesLineOutstanding(rec *gtables.SalesLine) error {
	return initSalesLineOutstanding(rec)
}

// InitSalesLineQtyToShip (InitQtyToShip) for sales posting: all that is still outstanding is
// to be shipped, all that can be invoiced to be invoiced
func InitSalesLineQtyToShip(rec *gtables.SalesLine) {
	initSalesLineQtyToShip(rec)
}
