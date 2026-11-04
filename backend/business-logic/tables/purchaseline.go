package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reservmgt"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salestax"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// purchaseLineLogic is the business logic of table 39 Purchase Line.
//
// The direct unit cost of an item comes from the vendor's Item Vendor line (its starting date,
// unit of measure and currency) or the item's Last Direct Cost; the unit cost adds the indirect
// cost %. Orders: the quantities to receive and invoice (all that is outstanding, by default) and
// the received/invoiced quantities, which posting fills. Not migrated yet: Item Purch. Qty. Disc.
// (Quantity Disc. % stays 0), reservations, item tracking, bins, jobs, drop shipments, blanket
// orders, extended texts and Fixed Asset lines; Amount and Amount Including Tax are calculated.
type purchaseLineLogic struct{}

func init() { gtables.RegisterPurchaseLineLogic(purchaseLineLogic{}) }

// OnInsert: the line's document exists
func (purchaseLineLogic) OnInsert(rec *gtables.PurchaseLine) error {
	rec.LockTable()
	purchHeader := gtables.NewPurchaseHeader(rec)
	return purchHeader.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "no": rec.Document_no})
}

// OnDelete: a line received but not invoiced stays
func (purchaseLineLogic) OnDelete(rec *gtables.PurchaseLine) error {
	err := rec.TestField("reserved_qty_base", types.Decimal{})
	if err == nil {
		err = rec.TestField("qty_rcd_not_invoiced", types.Decimal{})
	}
	if err != nil {
		return err
	}
	return checkPurchBlanketOrderLineDelete(rec)
}

// OnRename: a purchase line is never renamed
func (purchaseLineLogic) OnRename(rec *gtables.PurchaseLine) error {
	return apperrors.RenameNotAllowed(gtables.PurchaseLineTableName)
}

// getPurchHeader (GetPurchHeader): the line's document and its currency's rounding (local
// currency: the General Ledger Setup's)
func getPurchHeader(rec *gtables.PurchaseLine) (*gtables.PurchaseHeader, *gtables.Currency, error) {
	err := rec.TestField("document_no")
	if err != nil {
		return nil, nil, err
	}
	purchHeader := gtables.NewPurchaseHeader(rec)
	err = purchHeader.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "no": rec.Document_no})
	if err != nil {
		return nil, nil, err
	}
	currency := gtables.NewCurrency(rec)
	if purchHeader.Currency_code == "" {
		InitRoundingPrecision(currency)
	} else {
		err = purchHeader.TestField("currency_factor")
		if err != nil {
			return nil, nil, err
		}
		err = currency.GetOrError(purchHeader.Currency_code)
		if err != nil {
			return nil, nil, err
		}
		err = currency.TestField("amount_rounding_precision")
		if err != nil {
			return nil, nil, err
		}
	}
	return purchHeader, currency, nil
}

// purchLineDate (GetDate): the date of the line's currency calculations — the work date on a
// quote, else the posting date
func purchLineDate(purchHeader *gtables.PurchaseHeader) types.Date {
	if purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Quote {
		return system.WorkDate()
	} else {
		return purchHeader.Posting_date
	}
}

// testPurchLineNotReceived: the line's type, number and unit stay once something is received
func testPurchLineNotReceived(rec *gtables.PurchaseLine) error {
	err := rec.TestField("qty_rcd_not_invoiced", types.Decimal{})
	if err != nil {
		return err
	}
	return rec.TestField("quantity_received", types.Decimal{})
}

// OnValidate_Type: a new type starts the line again (not on a line already received)
func (purchaseLineLogic) OnValidate_Type(rec *gtables.PurchaseLine) error {
	err := testPurchLineNotReceived(rec)
	if err == nil {
		err = rec.TestField("reserved_qty_base", types.Decimal{})
	}
	if err != nil {
		return err
	}
	lineType := rec.Type
	if lineType == 3 {
		// the classic option 3 (resources) cannot be purchased
		return apperrors.JnlOptionNotAllowed(gtables.PurchaseLineTableName, "type", rec.Type)
	}
	rec.Init()
	rec.Type = lineType
	return nil
}

// OnValidate_No: the line again from the document (vendor, currency, expected receipt date,
// dimension 1/2, location, posting and tax groups) and the account or item: description,
// unit of measure, sales price, indirect cost %, posting groups, the direct unit cost and the
// quantity it had
func (purchaseLineLogic) OnValidate_No(rec *gtables.PurchaseLine) error {
	err := testPurchLineNotReceived(rec)
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
	purchHeader, _, err := getPurchHeader(rec)
	if err != nil {
		return err
	}
	rec.Buy_from_vendor_no = purchHeader.Buy_from_vendor_no
	rec.Currency_code = purchHeader.Currency_code
	rec.Expected_receipt_date = purchHeader.Expected_receipt_date
	rec.Dimension_1_code = purchHeader.Dimension_1_code
	rec.Dimension_2_code = purchHeader.Dimension_2_code
	rec.Project_no = purchHeader.Project_no
	rec.Location_code = purchHeader.Location_code
	rec.Transaction_type = purchHeader.Transaction_type
	rec.Transport_method = purchHeader.Transport_method
	rec.Pay_to_vendor_no = purchHeader.Pay_to_vendor_no
	rec.Gen_bus_posting_group = purchHeader.Gen_bus_posting_group
	rec.Tax_bus_posting_group = purchHeader.Tax_bus_posting_group
	rec.Entry_point = purchHeader.Entry_point
	rec.Area = purchHeader.Area
	rec.Transaction_specification = purchHeader.Transaction_specification
	rec.Tax_area_code = purchHeader.Tax_area_code
	rec.Tax_liable = purchHeader.Tax_liable

	switch rec.Type {
	case gtables.PurchaseLine_Type.Blank:
		standardText := gtables.NewStandardText(rec)
		err = standardText.GetOrError(rec.No)
		if err != nil {
			return err
		}
		rec.Description = standardText.Description
	case gtables.PurchaseLine_Type.GLAccount:
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
	case gtables.PurchaseLine_Type.Item:
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
		rec.Description = item.Description
		rec.Description_2 = item.Description_2
		rec.Unit_price_lcy = item.Unit_price
		rec.Units_per_parcel = item.Units_per_parcel
		rec.Indirect_cost_percent = item.Indirect_cost_percent
		rec.Gen_prod_posting_group = item.Gen_prod_posting_group
		rec.Tax_prod_posting_group = item.Tax_prod_posting_group
		rec.Tax_group_code = item.Tax_group_code
		if item.Reserve == gtables.Item_Reserve.Optional {
			rec.Reserve = gtables.PurchaseLineReserve(purchHeader.Reserve)
		} else {
			rec.Reserve = gtables.PurchaseLineReserve(item.Reserve)
		}
		if item.Price_includes_tax {
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
			rec.Unit_price_lcy = system.Round(rec.Unit_price_lcy.Div(types.NewDecimalFromInt(1).Add(taxPostingSetup.Vat_percent.Div(hundred))), roundPrecision)
		}
		if purchHeader.Language_code != "" {
			itemTranslation := gtables.NewItemTranslation(rec)
			if itemTranslation.Get(map[string]interface{}{"item_no": item.No, "variant_code": "", "language_code": purchHeader.Language_code}) {
				rec.Description = itemTranslation.Description
				rec.Description_2 = itemTranslation.Description_2
			}
		}
		err = rec.Validate("unit_of_measure_code", item.Purch_unit_of_measure)
		if err != nil {
			return err
		}
	case gtables.PurchaseLine_Type.FixedAsset:
		fa := gtables.NewFixedAsset(rec)
		err = fa.GetOrError(rec.No)
		if err != nil {
			return err
		}
		err = fa.TestField("inactive", false)
		if err != nil {
			return err
		}
		err = fa.TestField("blocked", false)
		if err != nil {
			return err
		}
		err = getPurchLineFAPostingGroup(rec)
		if err != nil {
			return err
		}
		rec.Description = fa.Description
		rec.Allow_invoice_disc = false
	}
	if rec.Type != gtables.PurchaseLine_Type.Blank {
		err = rec.Validate("tax_prod_posting_group", rec.Tax_prod_posting_group)
		if err != nil {
			return err
		}
		if rec.Type == gtables.PurchaseLine_Type.Item {
			err = updatePurchLineDirectUnitCost(rec)
			if err != nil {
				return err
			}
			err = rec.Validate("direct_unit_cost", rec.Direct_unit_cost)
			if err != nil {
				return err
			}
		}
		return rec.Validate("quantity", xRec.Quantity)
	}
	return nil
}

// OnValidate_Quantity: Quantity (Base) — not below what is received —, the outstanding
// quantity and amount, the quantities to receive and invoice, and the quantity discount
func (purchaseLineLogic) OnValidate_Quantity(rec *gtables.PurchaseLine) error {
	quantityBase, err := calcPurchLineBaseQty(rec, rec.Quantity)
	if err != nil {
		return err
	}
	rec.Quantity_base = quantityBase
	if rec.Quantity.Mul(rec.Quantity_received).IsNegative() || rec.Quantity.Abs().LessThan(rec.Quantity_received.Abs()) {
		return apperrors.PurchQtyLessThan(gtables.PurchaseLineTableName, "quantity", "quantity_received")
	}
	if rec.Quantity_base.Mul(rec.Qty_received_base).IsNegative() || rec.Quantity_base.Abs().LessThan(rec.Qty_received_base.Abs()) {
		return apperrors.PurchQtyLessThan(gtables.PurchaseLineTableName, "quantity_base", "qty_received_base")
	}
	err = initPurchLineOutstanding(rec)
	if err == nil {
		err = checkReservedNotAbove(gtables.PurchaseLineTableName, "reserved_qty_base", rec.Outstanding_qty_base, rec.Reserved_qty_base)
	}
	if err != nil {
		return err
	}
	initPurchLineQtyToReceive(rec)
	// Item Purch. Qty. Disc. is not migrated yet: no quantity discount
	rec.Quantity_disc_percent = types.Decimal{}
	return validatePurchLineQuantityDisc(rec)
}

// calcPurchLineBaseQty (CalcBaseQty): a quantity in the base unit of measure
func calcPurchLineBaseQty(rec *gtables.PurchaseLine, qty types.Decimal) (types.Decimal, error) {
	err := rec.TestField("qty_per_unit_of_measure")
	if err != nil {
		return types.Decimal{}, err
	}
	return system.Round(qty.Mul(rec.Qty_per_unit_of_measure), roundPrecision), nil
}

// initPurchLineOutstanding (InitOutstanding): what is not received yet is outstanding, what is
// received and not invoiced yet is received not invoiced
func initPurchLineOutstanding(rec *gtables.PurchaseLine) error {
	rec.Outstanding_quantity = rec.Quantity.Sub(rec.Quantity_received)
	rec.Outstanding_qty_base = rec.Quantity_base.Sub(rec.Qty_received_base)
	rec.Qty_rcd_not_invoiced = rec.Quantity_received.Sub(rec.Quantity_invoiced)
	rec.Qty_rcd_not_invd_base = rec.Qty_received_base.Sub(rec.Qty_invoiced_base)
	return initPurchLineOutstandingAmount(rec)
}

// initPurchLineOutstandingAmount (InitOutstandingAmount): the outstanding and the received not
// invoiced shares of Amount Including Tax, in the document's currency and in LCY
func initPurchLineOutstandingAmount(rec *gtables.PurchaseLine) error {
	if rec.Quantity.IsZero() {
		rec.Outstanding_amount = types.Decimal{}
		rec.Outstanding_amount_lcy = types.Decimal{}
		rec.Amt_rcd_not_invoiced = types.Decimal{}
		rec.Amt_rcd_not_invoiced_lcy = types.Decimal{}
	} else {
		purchHeader, currency, err := getPurchHeader(rec)
		if err != nil {
			return err
		}
		rec.Outstanding_amount = system.Round(rec.Amount_including_tax.Mul(rec.Outstanding_quantity).Div(rec.Quantity), currency.Amount_rounding_precision)
		rec.Outstanding_amount_lcy, err = purchLineAmountLCY(rec, purchHeader, rec.Outstanding_amount)
		if err != nil {
			return err
		}
		rec.Amt_rcd_not_invoiced = system.Round(rec.Amount_including_tax.Mul(rec.Qty_rcd_not_invoiced).Div(rec.Quantity), currency.Amount_rounding_precision)
		rec.Amt_rcd_not_invoiced_lcy, err = purchLineAmountLCY(rec, purchHeader, rec.Amt_rcd_not_invoiced)
		if err != nil {
			return err
		}
	}
	return nil
}

// purchLineAmountLCY (Outstanding Amount / Amt. Rcd. Not Invoiced OnValidate): an amount of the
// line in LCY at the document's currency factor, rounded to the local currency
func purchLineAmountLCY(rec *gtables.PurchaseLine, purchHeader *gtables.PurchaseHeader, amount types.Decimal) (types.Decimal, error) {
	currency2 := gtables.NewCurrency(rec)
	InitRoundingPrecision(currency2)
	if purchHeader.Currency_code != "" {
		amountLCY, err := ExchangeRates(rec).ExchangeAmtFCYToLCY(purchLineDate(purchHeader), rec.Currency_code, amount, purchHeader.Currency_factor)
		if err != nil {
			return types.Decimal{}, err
		}
		return system.Round(amountLCY, currency2.Amount_rounding_precision), nil
	} else {
		return system.Round(amount, currency2.Amount_rounding_precision), nil
	}
}

// initPurchLineQtyToReceive (InitQtyToReceive): all that is outstanding is to be received
func initPurchLineQtyToReceive(rec *gtables.PurchaseLine) {
	rec.Qty_to_receive = rec.Outstanding_quantity
	rec.Qty_to_receive_base = rec.Outstanding_qty_base
	initPurchLineQtyToInvoice(rec)
}

// initPurchLineQtyToInvoice (InitQtyToInvoice): all that can be invoiced is to be invoiced
func initPurchLineQtyToInvoice(rec *gtables.PurchaseLine) {
	rec.Qty_to_invoice = maxPurchLineQtyToInvoice(rec)
	rec.Qty_to_invoice_base = maxPurchLineQtyToInvoiceBase(rec)
}

// maxPurchLineQtyToInvoice (MaxQtyToInvoice): what is received or to be received, less what is
// invoiced
func maxPurchLineQtyToInvoice(rec *gtables.PurchaseLine) types.Decimal {
	return rec.Quantity_received.Add(rec.Qty_to_receive).Sub(rec.Quantity_invoiced)
}

// maxPurchLineQtyToInvoiceBase (MaxQtyToInvoiceBase): the same in the base unit of measure
func maxPurchLineQtyToInvoiceBase(rec *gtables.PurchaseLine) types.Decimal {
	return rec.Qty_received_base.Add(rec.Qty_to_receive_base).Sub(rec.Qty_invoiced_base)
}

// OnValidate_Qty_to_receive: its base quantity and the quantity to invoice; not more than is
// outstanding (and of the same sign)
func (purchaseLineLogic) OnValidate_Qty_to_receive(rec *gtables.PurchaseLine) error {
	if rec.Qty_to_receive.Equal(rec.Quantity.Sub(rec.Quantity_received)) {
		initPurchLineQtyToReceive(rec)
	} else {
		qtyToReceiveBase, err := calcPurchLineBaseQty(rec, rec.Qty_to_receive)
		if err != nil {
			return err
		}
		rec.Qty_to_receive_base = qtyToReceiveBase
		initPurchLineQtyToInvoice(rec)
	}
	if qtyToHandleAbove(rec.Qty_to_receive, rec.Quantity, rec.Outstanding_quantity) {
		return apperrors.PurchCannotReceiveMore(system.Format(rec.Outstanding_quantity), false)
	}
	if qtyToHandleAbove(rec.Qty_to_receive_base, rec.Quantity_base, rec.Outstanding_qty_base) {
		return apperrors.PurchCannotReceiveMore(system.Format(rec.Outstanding_qty_base), true)
	}
	return nil
}

// OnValidate_Qty_to_receive_base: only in the base unit of measure; Qty. to Receive follows
func (purchaseLineLogic) OnValidate_Qty_to_receive_base(rec *gtables.PurchaseLine) error {
	err := rec.TestField("qty_per_unit_of_measure", types.NewDecimalFromInt(1))
	if err != nil {
		return err
	}
	return rec.Validate("qty_to_receive", rec.Qty_to_receive_base)
}

// OnValidate_Qty_to_invoice: its base quantity; not more than is received or to be received
// and not invoiced (and of the line's sign)
func (purchaseLineLogic) OnValidate_Qty_to_invoice(rec *gtables.PurchaseLine) error {
	if rec.Qty_to_invoice.Equal(maxPurchLineQtyToInvoice(rec)) {
		initPurchLineQtyToInvoice(rec)
	} else {
		qtyToInvoiceBase, err := calcPurchLineBaseQty(rec, rec.Qty_to_invoice)
		if err != nil {
			return err
		}
		rec.Qty_to_invoice_base = qtyToInvoiceBase
	}
	maxQtyToInvoice := maxPurchLineQtyToInvoice(rec)
	if rec.Qty_to_invoice.Mul(rec.Quantity).IsNegative() || rec.Qty_to_invoice.Abs().GreaterThan(maxQtyToInvoice.Abs()) {
		return apperrors.PurchCannotInvoiceMore(system.Format(maxQtyToInvoice), false)
	}
	maxQtyToInvoiceBase := maxPurchLineQtyToInvoiceBase(rec)
	if rec.Qty_to_invoice_base.Mul(rec.Quantity_base).IsNegative() || rec.Qty_to_invoice_base.Abs().GreaterThan(maxQtyToInvoiceBase.Abs()) {
		return apperrors.PurchCannotInvoiceMore(system.Format(maxQtyToInvoiceBase), true)
	}
	return nil
}

// OnValidate_Qty_to_invoice_base: only in the base unit of measure; Qty. to Invoice follows
func (purchaseLineLogic) OnValidate_Qty_to_invoice_base(rec *gtables.PurchaseLine) error {
	err := rec.TestField("qty_per_unit_of_measure", types.NewDecimalFromInt(1))
	if err != nil {
		return err
	}
	return rec.Validate("qty_to_invoice", rec.Qty_to_invoice_base)
}

// OnValidate_Direct_unit_cost: the discount, unit cost and amounts again
func (purchaseLineLogic) OnValidate_Direct_unit_cost(rec *gtables.PurchaseLine) error {
	return validatePurchLineDiscountPercent(rec)
}

// OnValidate_Unit_cost_lcy: the sales cost; an item not at standard cost has no indirect cost
// then
func (purchaseLineLogic) OnValidate_Unit_cost_lcy(rec *gtables.PurchaseLine) error {
	err := rec.TestField("no")
	if err != nil {
		return err
	}
	err = rec.TestField("quantity")
	if err != nil {
		return err
	}
	if rec.Type == gtables.PurchaseLine_Type.Item {
		item := gtables.NewItem(rec)
		err = item.GetOrError(rec.No)
		if err != nil {
			return err
		}
		if item.Costing_method != gtables.Item_Costing_method.Standard {
			rec.Indirect_cost_percent = types.Decimal{}
		}
	}
	return nil
}

// OnValidate_Indirect_cost_percent: the unit cost again (not for an item at standard cost)
func (purchaseLineLogic) OnValidate_Indirect_cost_percent(rec *gtables.PurchaseLine) error {
	err := rec.TestField("no")
	if err != nil {
		return err
	}
	if rec.Type == gtables.PurchaseLine_Type.Item {
		item := gtables.NewItem(rec)
		err = item.GetOrError(rec.No)
		if err != nil {
			return err
		}
		if item.Costing_method == gtables.Item_Costing_method.Standard {
			ts := i18n.GetInstance()
			lang := fcodeunits.CurrentLanguage()
			return apperrors.PurchIndirectCostStandard(gtables.PurchaseLineTableName, "indirect_cost_percent",
				ts.FieldCaption(gtables.ItemTableName, "costing_method", lang),
				ts.OptionCaption(gtables.ItemTableName, "costing_method", i18n.Key(item.Costing_method.String()), lang))
		}
	}
	return updatePurchLineUnitCost(rec)
}

// validatePurchLineQuantityDisc (Quantity Disc. % OnValidate): the quantity discount is the line
// discount
func validatePurchLineQuantityDisc(rec *gtables.PurchaseLine) error {
	rec.Line_discount_percent = rec.Quantity_disc_percent
	return validatePurchLineDiscountPercent(rec)
}

// OnValidate_Line_discount_percent: Line Discount Amount from the percent, the unit cost and the
// amounts again
func (purchaseLineLogic) OnValidate_Line_discount_percent(rec *gtables.PurchaseLine) error {
	return validatePurchLineDiscountPercent(rec)
}

// validatePurchLineDiscountPercent (Line Discount % OnValidate)
func validatePurchLineDiscountPercent(rec *gtables.PurchaseLine) error {
	_, currency, err := getPurchHeader(rec)
	if err != nil {
		return err
	}
	lineAmount := system.Round(rec.Quantity.Mul(rec.Direct_unit_cost), currency.Amount_rounding_precision)
	rec.Line_discount_amount = system.Round(lineAmount.Mul(rec.Line_discount_percent).Div(hundred), currency.Amount_rounding_precision)
	rec.Inv_discount_amount = types.Decimal{}
	err = updatePurchLineUnitCost(rec)
	if err != nil {
		return err
	}
	return UpdatePurchLineAmounts(rec)
}

// OnValidate_Line_discount_amount: Line Discount % from the amount, the unit cost and the
// amounts again
func (purchaseLineLogic) OnValidate_Line_discount_amount(rec *gtables.PurchaseLine) error {
	err := rec.TestField("quantity")
	if err != nil {
		return err
	}
	if !rec.Direct_unit_cost.IsZero() {
		rec.Line_discount_percent = system.Round(rec.Line_discount_amount.Div(rec.Quantity).Div(rec.Direct_unit_cost).Mul(hundred), roundPrecision)
	} else {
		rec.Line_discount_percent = types.Decimal{}
	}
	rec.Inv_discount_amount = types.Decimal{}
	err = updatePurchLineUnitCost(rec)
	if err != nil {
		return err
	}
	return UpdatePurchLineAmounts(rec)
}

// OnValidate_Allow_invoice_disc: a line no longer allowing it loses its invoice discount
func (purchaseLineLogic) OnValidate_Allow_invoice_disc(rec *gtables.PurchaseLine) error {
	xRec := rec.XRec()
	if rec.Allow_invoice_disc != xRec.Allow_invoice_disc && !rec.Allow_invoice_disc {
		rec.Inv_discount_amount = types.Decimal{}
		return UpdatePurchLineAmounts(rec)
	}
	return nil
}

// OnValidate_Inv_discount_amount: the unit cost and the amounts again (Purch.-Calc. Discount sets
// it)
func (purchaseLineLogic) OnValidate_Inv_discount_amount(rec *gtables.PurchaseLine) error {
	err := rec.TestField("quantity")
	if err != nil {
		return err
	}
	err = updatePurchLineUnitCost(rec)
	if err != nil {
		return err
	}
	return UpdatePurchLineAmounts(rec)
}

// OnValidate_Gen_bus_posting_group: its default tax business posting group
func (purchaseLineLogic) OnValidate_Gen_bus_posting_group(rec *gtables.PurchaseLine) error {
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
func (purchaseLineLogic) OnValidate_Gen_prod_posting_group(rec *gtables.PurchaseLine) error {
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

// Tax Area Code, Tax Liable, Tax Group Code, Use Tax: the amounts again
func (purchaseLineLogic) OnValidate_Tax_area_code(rec *gtables.PurchaseLine) error {
	return UpdatePurchLineAmounts(rec)
}

func (purchaseLineLogic) OnValidate_Tax_liable(rec *gtables.PurchaseLine) error {
	return UpdatePurchLineAmounts(rec)
}

func (purchaseLineLogic) OnValidate_Tax_group_code(rec *gtables.PurchaseLine) error {
	return UpdatePurchLineAmounts(rec)
}

func (purchaseLineLogic) OnValidate_Use_tax(rec *gtables.PurchaseLine) error {
	return UpdatePurchLineAmounts(rec)
}

// OnValidate_Tax_bus_posting_group: the tax setup again
func (purchaseLineLogic) OnValidate_Tax_bus_posting_group(rec *gtables.PurchaseLine) error {
	return rec.Validate("tax_prod_posting_group", rec.Tax_prod_posting_group)
}

// OnValidate_Tax_prod_posting_group: Tax % and Tax Calculation Type from the Tax Posting Setup
// (none: Normal VAT 0 %), then the amounts
func (purchaseLineLogic) OnValidate_Tax_prod_posting_group(rec *gtables.PurchaseLine) error {
	taxPostingSetup := gtables.NewTaxPostingSetup(rec)
	if !taxPostingSetup.Get(map[string]interface{}{"tax_bus_posting_group": rec.Tax_bus_posting_group, "tax_prod_posting_group": rec.Tax_prod_posting_group}) {
		taxPostingSetup.Init()
	}
	rec.Tax_percent = taxPostingSetup.Vat_percent
	rec.Tax_calculation_type = gtables.PurchaseLineTax_calculation_type(taxPostingSetup.Tax_calculation_type)
	switch rec.Tax_calculation_type {
	case gtables.PurchaseLine_Tax_calculation_type.ReverseChargeVAT, gtables.PurchaseLine_Tax_calculation_type.SalesTax:
		rec.Tax_percent = types.Decimal{}
	case gtables.PurchaseLine_Tax_calculation_type.FullVAT:
		err := rec.TestField("type", gtables.PurchaseLine_Type.GLAccount)
		if err != nil {
			return err
		}
		err = taxPostingSetup.TestField("purchase_vat_account")
		if err != nil {
			return err
		}
		err = rec.TestField("no", taxPostingSetup.Purchase_vat_account)
		if err != nil {
			return err
		}
	}
	return UpdatePurchLineAmounts(rec)
}

// OnValidate_Variant_code: the variant's description (translated by the document's language)
func (purchaseLineLogic) OnValidate_Variant_code(rec *gtables.PurchaseLine) error {
	if rec.Variant_code == "" {
		return nil
	}
	err := rec.TestField("type", gtables.PurchaseLine_Type.Item)
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
	purchHeader, _, err := getPurchHeader(rec)
	if err != nil {
		return err
	}
	if purchHeader.Language_code != "" {
		itemTranslation := gtables.NewItemTranslation(rec)
		if itemTranslation.Get(map[string]interface{}{"item_no": rec.No, "variant_code": rec.Variant_code, "language_code": purchHeader.Language_code}) {
			rec.Description = itemTranslation.Description
			rec.Description_2 = itemTranslation.Description_2
		}
	}
	return nil
}

// OnValidate_Unit_of_measure_code: the unit's description (translated by the document's
// language); an item's quantity per unit, weights, volume, unit cost and direct unit cost for
// the unit; the quantity again (not on a line already received)
func (purchaseLineLogic) OnValidate_Unit_of_measure_code(rec *gtables.PurchaseLine) error {
	err := rec.TestField("quantity_received", types.Decimal{})
	if err != nil {
		return err
	}
	err = rec.TestField("qty_received_base", types.Decimal{})
	if err != nil {
		return err
	}
	err = rec.TestField("qty_rcd_not_invoiced", types.Decimal{})
	if err != nil {
		return err
	}
	purchHeader, _, err := getPurchHeader(rec)
	if err != nil {
		return err
	}
	if rec.Unit_of_measure_code == "" {
		rec.Unit_of_measure = ""
	} else {
		unitOfMeasure := gtables.NewUnitOfMeasure(rec)
		err = unitOfMeasure.GetOrError(rec.Unit_of_measure_code)
		if err != nil {
			return err
		}
		rec.Unit_of_measure = types.Text(system.CopyStr(unitOfMeasure.Description.String(), 1, rec.MaxStrLen("unit_of_measure")))
		if purchHeader.Language_code != "" {
			unitOfMeasureTranslation := gtables.NewUnitOfMeasureTranslation(rec)
			unitOfMeasureTranslation.SetRange("code", rec.Unit_of_measure_code)
			unitOfMeasureTranslation.SetRange("language_code", purchHeader.Language_code)
			if unitOfMeasureTranslation.FindFirst() {
				rec.Unit_of_measure = types.Text(system.CopyStr(unitOfMeasureTranslation.Description.String(), 1, rec.MaxStrLen("unit_of_measure")))
			}
		}
	}
	if rec.Type == gtables.PurchaseLine_Type.Item && rec.No != "" {
		item := gtables.NewItem(rec)
		err = item.GetOrError(rec.No)
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
		err = updatePurchLineUOMQtyPerStockQty(rec, item, purchHeader)
		if err != nil {
			return err
		}
	} else {
		rec.Qty_per_unit_of_measure = types.NewDecimalFromInt(1)
	}
	err = rec.Validate("quantity", rec.Quantity)
	if err != nil {
		return err
	}
	return rec.Validate("reserved_qty_base", rec.Reserved_qty_base)
}

// updatePurchLineUOMQtyPerStockQty (UpdateUOMQtyPerStockQty): the item's unit cost for the
// line's unit of measure, in the document's currency, and its direct unit cost for the unit
func updatePurchLineUOMQtyPerStockQty(rec *gtables.PurchaseLine, item *gtables.Item, purchHeader *gtables.PurchaseHeader) error {
	rec.Unit_cost_lcy = item.Unit_cost.Mul(rec.Qty_per_unit_of_measure)
	if purchHeader.Currency_code != "" {
		unitCost, err := ExchangeRates(rec).ExchangeAmtLCYToFCY(purchLineDate(purchHeader), purchHeader.Currency_code, rec.Unit_cost_lcy, purchHeader.Currency_factor)
		if err != nil {
			return err
		}
		rec.Unit_cost = unitCost
	} else {
		rec.Unit_cost = rec.Unit_cost_lcy
	}
	return updatePurchLineDirectUnitCost(rec)
}

// OnValidate_Quantity_base: only in the base unit of measure; the quantity follows
func (purchaseLineLogic) OnValidate_Quantity_base(rec *gtables.PurchaseLine) error {
	err := rec.TestField("qty_per_unit_of_measure", types.NewDecimalFromInt(1))
	if err != nil {
		return err
	}
	return rec.Validate("quantity", rec.Quantity_base)
}

// updatePurchLineDirectUnitCost (UpdateDirectUnitCost): an item's direct unit cost from the
// vendor's purchase terms (Item Vendor in effect on the order or posting date, for the line's
// unit of measure — else the item's Last Direct Cost) in the document's currency, and the
// vendor's item no.
func updatePurchLineDirectUnitCost(rec *gtables.PurchaseLine) error {
	purchHeader, currency, err := getPurchHeader(rec)
	if err != nil {
		return err
	}
	item := gtables.NewItem(rec)
	err = item.GetOrError(rec.No)
	if err != nil {
		return err
	}
	itemVend := gtables.NewItemVendor(rec)
	itemVend.Vendor_no = rec.Buy_from_vendor_no
	itemVend.Unit_of_measure_code = rec.Unit_of_measure_code
	if purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Quote || purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Order {
		itemVend.Starting_date = purchHeader.Order_date
	} else {
		itemVend.Starting_date = purchHeader.Posting_date
	}
	err = FindItemVend(item, itemVend, true)
	if err != nil {
		return err
	}
	rec.Direct_unit_cost = itemVend.Direct_unit_cost
	if itemVend.Currency_code != purchHeader.Currency_code {
		exchangeRates := ExchangeRates(rec)
		if itemVend.Currency_code != "" {
			rate, err := exchangeRates.ExchangeRate(purchLineDate(purchHeader), itemVend.Currency_code)
			if err != nil {
				return err
			}
			rec.Direct_unit_cost, err = exchangeRates.ExchangeAmtFCYToLCY(purchLineDate(purchHeader), itemVend.Currency_code, rec.Direct_unit_cost, rate)
			if err != nil {
				return err
			}
		}
		if purchHeader.Currency_code != "" {
			rec.Direct_unit_cost, err = exchangeRates.ExchangeAmtLCYToFCY(purchLineDate(purchHeader), rec.Currency_code, rec.Direct_unit_cost, purchHeader.Currency_factor)
			if err != nil {
				return err
			}
			err = currency.TestField("unit_amount_rounding_precision")
			if err != nil {
				return err
			}
		}
		rec.Direct_unit_cost = system.Round(rec.Direct_unit_cost, currency.Unit_amount_rounding_precision)
	}
	rec.Vendor_item_no = itemVend.Vendor_item_no
	return nil
}

// updatePurchLineUnitCost (UpdateUnitCost): Unit Cost = the direct unit cost less the line and
// invoice discounts per unit, plus the indirect cost %; Unit Cost (LCY) at the document's
// currency factor (an item at standard cost: its unit cost per unit of measure, no indirect
// cost)
func updatePurchLineUnitCost(rec *gtables.PurchaseLine) error {
	one := types.NewDecimalFromInt(1)
	indirect := one.Add(rec.Indirect_cost_percent.Div(hundred))
	if !rec.Quantity.IsZero() {
		rec.Unit_cost = rec.Direct_unit_cost.Sub(rec.Line_discount_amount.Add(rec.Inv_discount_amount).Div(rec.Quantity)).Mul(indirect)
	} else {
		rec.Unit_cost = rec.Direct_unit_cost.Mul(indirect)
	}
	purchHeader, currency, err := getPurchHeader(rec)
	if err != nil {
		return err
	}
	if purchHeader.Currency_code != "" {
		err = purchHeader.TestField("currency_factor")
		if err != nil {
			return err
		}
		rec.Unit_cost_lcy, err = ExchangeRates(rec).ExchangeAmtFCYToLCY(purchLineDate(purchHeader), rec.Currency_code, rec.Unit_cost, purchHeader.Currency_factor)
		if err != nil {
			return err
		}
	} else {
		rec.Unit_cost_lcy = rec.Unit_cost
	}
	if rec.Type == gtables.PurchaseLine_Type.Item {
		item := gtables.NewItem(rec)
		err = item.GetOrError(rec.No)
		if err != nil {
			return err
		}
		if item.Costing_method == gtables.Item_Costing_method.Standard {
			rec.Unit_cost_lcy = item.Unit_cost.Mul(rec.Qty_per_unit_of_measure)
			rec.Indirect_cost_percent = types.Decimal{}
		}
	}
	rec.Unit_cost_lcy = system.Round(rec.Unit_cost_lcy, roundPrecision)
	if purchHeader.Currency_code != "" {
		err = currency.TestField("unit_amount_rounding_precision")
		if err != nil {
			return err
		}
	}
	rec.Unit_cost = system.Round(rec.Unit_cost, currency.Unit_amount_rounding_precision)
	return nil
}

// UpdatePurchLineAmounts (UpdateAmounts): Amount after the line and invoice discounts, the tax
// (VAT % or sales tax of the line's tax area and group — none on the invoice with use tax),
// Amount Including Tax and the outstanding amounts
func UpdatePurchLineAmounts(rec *gtables.PurchaseLine) error {
	purchHeader, currency, err := getPurchHeader(rec)
	if err != nil {
		return err
	}
	err = rec.TestField("type")
	if err != nil {
		return err
	}
	lineAmount := system.Round(rec.Quantity.Mul(rec.Direct_unit_cost), currency.Amount_rounding_precision)
	switch rec.Tax_calculation_type {
	case gtables.PurchaseLine_Tax_calculation_type.NormalVAT, gtables.PurchaseLine_Tax_calculation_type.ReverseChargeVAT:
		rec.Amount = lineAmount.Sub(rec.Line_discount_amount).Sub(rec.Inv_discount_amount)
		rec.Tax_base_amount = system.Round(rec.Amount, roundPrecision)
		rec.Amount_including_tax = system.Round(rec.Amount.Add(rec.Tax_base_amount.Mul(rec.Tax_percent).Div(hundred)), roundPrecision)
	case gtables.PurchaseLine_Tax_calculation_type.FullVAT:
		rec.Amount = types.Decimal{}
		rec.Tax_base_amount = types.Decimal{}
		rec.Amount_including_tax = lineAmount
	case gtables.PurchaseLine_Tax_calculation_type.SalesTax:
		rec.Amount = lineAmount.Sub(rec.Line_discount_amount).Sub(rec.Inv_discount_amount)
		rec.Tax_base_amount = system.Round(rec.Amount, roundPrecision)
		if rec.Use_tax {
			rec.Amount_including_tax = system.Round(rec.Tax_base_amount, roundPrecision)
		} else {
			salesTaxCalculate := salestax.New(rec)
			taxAmount, err := salesTaxCalculate.CalculateTax(rec.Tax_area_code, rec.Tax_group_code, rec.Tax_liable, purchHeader.Posting_date, rec.Tax_base_amount, rec.Quantity, purchHeader.Currency_factor)
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
	}
	return initPurchLineOutstandingAmount(rec)
}

// InitPurchLineOutstanding (InitOutstanding) for purchase posting: the outstanding and
// received-not-invoiced quantities and amounts after the line's received/invoiced quantities
// changed
func InitPurchLineOutstanding(rec *gtables.PurchaseLine) error {
	return initPurchLineOutstanding(rec)
}

// InitPurchLineQtyToReceive (InitQtyToReceive) for purchase posting: all that is still
// outstanding is to be received, all that can be invoiced to be invoiced
func InitPurchLineQtyToReceive(rec *gtables.PurchaseLine) {
	initPurchLineQtyToReceive(rec)
}

// getPurchLineFAPostingGroup (GetFAPostingGroup): a fixed asset line takes the FA Setup's default
// book (when the asset has it) and Acquisition Cost; the asset's FA posting group, and the
// general and tax product posting groups of its acquisition cost (or maintenance expense) account
func getPurchLineFAPostingGroup(rec *gtables.PurchaseLine) error {
	if rec.Type != gtables.PurchaseLine_Type.FixedAsset || rec.No == "" {
		return nil
	}
	faDeprBook := gtables.NewFADepreciationBook(rec)
	if rec.Depreciation_book_code == "" {
		faSetup := gtables.GetFASetup(rec)
		rec.Depreciation_book_code = faSetup.Default_depr_book
		if !faDeprBook.Get(map[string]interface{}{"fa_no": rec.No, "depreciation_book_code": rec.Depreciation_book_code}) {
			rec.Depreciation_book_code = ""
		}
		if rec.Depreciation_book_code == "" {
			return nil
		}
	}
	if rec.Fa_posting_type == gtables.PurchaseLine_Fa_posting_type.Blank {
		rec.Fa_posting_type = gtables.PurchaseLine_Fa_posting_type.AcquisitionCost
	}
	err := faDeprBook.GetOrError(map[string]interface{}{"fa_no": rec.No, "depreciation_book_code": rec.Depreciation_book_code})
	if err != nil {
		return err
	}
	err = faDeprBook.TestField("fa_posting_group")
	if err != nil {
		return err
	}
	faPostingGr := gtables.NewFAPostingGroup(rec)
	err = faPostingGr.GetOrError(faDeprBook.Fa_posting_group)
	if err != nil {
		return err
	}
	field, accountNo := "acquisition_cost_account", faPostingGr.Acquisition_cost_account
	if rec.Fa_posting_type != gtables.PurchaseLine_Fa_posting_type.AcquisitionCost {
		field, accountNo = "maintenance_expense_account", faPostingGr.Maintenance_expense_account
	}
	err = faPostingGr.TestField(field)
	if err != nil {
		return err
	}
	glAcc := gtables.NewGLAccount(rec)
	err = glAcc.GetOrError(accountNo)
	if err != nil {
		return err
	}
	err = glAcc.TestField("blocked", false)
	if err == nil {
		err = glAcc.TestField("account_type", gtables.GLAccount_Account_type.Posting)
	}
	if err == nil {
		err = glAcc.TestField("gen_prod_posting_group")
	}
	if err != nil {
		return err
	}
	rec.Posting_group = faDeprBook.Fa_posting_group
	err = rec.Validate("gen_prod_posting_group", glAcc.Gen_prod_posting_group)
	if err != nil {
		return err
	}
	rec.Tax_group_code = glAcc.Tax_group_code
	return rec.Validate("tax_prod_posting_group", glAcc.Tax_prod_posting_group)
}

// OnValidate_Fa_posting_type: a fixed asset line posts an acquisition cost or a maintenance (its
// accounts' posting groups); other lines have none of the FA fields
// OnValidate_Project_no: not on fixed asset lines; a reserved line keeps its project
func (purchaseLineLogic) OnValidate_Project_no(rec *gtables.PurchaseLine) error {
	if rec.Type == gtables.PurchaseLine_Type.FixedAsset {
		if rec.Project_no != "" {
			return apperrors.FANotTogetherWith(gtables.PurchaseLineTableName, "project_no", "type", rec.Type)
		}
	}
	return reservmgt.New(rec).PurchLineVerifyChange(rec)
}

func (purchaseLineLogic) OnValidate_Fa_posting_type(rec *gtables.PurchaseLine) error {
	if rec.Type == gtables.PurchaseLine_Type.FixedAsset {
		err := rec.TestField("project_no", types.Code(""))
		if err != nil {
			return err
		}
		if rec.Fa_posting_type == gtables.PurchaseLine_Fa_posting_type.Blank {
			rec.Fa_posting_type = gtables.PurchaseLine_Fa_posting_type.AcquisitionCost
		}
		return getPurchLineFAPostingGroup(rec)
	}
	rec.Depreciation_book_code = ""
	rec.Fa_posting_date = types.Date{}
	rec.Salvage_value = types.Decimal{}
	rec.Depr_until_fa_posting_date = false
	rec.Depr_acquisition_cost = false
	rec.Maintenance_code = ""
	rec.Insurance_no = ""
	rec.Duplicate_in_depreciation_book = ""
	rec.Use_duplication_list = false
	return nil
}

// OnValidate_Depreciation_book_code: the asset's posting group in the book
func (purchaseLineLogic) OnValidate_Depreciation_book_code(rec *gtables.PurchaseLine) error {
	return getPurchLineFAPostingGroup(rec)
}

// OnValidate_Duplicate_in_depreciation_book: not with the duplication list
func (purchaseLineLogic) OnValidate_Duplicate_in_depreciation_book(rec *gtables.PurchaseLine) error {
	rec.Use_duplication_list = false
	return nil
}

// OnValidate_Use_duplication_list: not with a book to duplicate in
func (purchaseLineLogic) OnValidate_Use_duplication_list(rec *gtables.PurchaseLine) error {
	rec.Duplicate_in_depreciation_book = ""
	return nil
}
