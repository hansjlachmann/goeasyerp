package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reservmgt"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// requisitionLineLogic is the business logic of table 246 Requisition_Line.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/requisitionline_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type requisitionLineLogic struct{}

func init() { gtables.RegisterRequisitionLineLogic(requisitionLineLogic{}) }

// OnInsert: the line's template and worksheet exist
func (requisitionLineLogic) OnInsert(rec *gtables.RequisitionLine) error {
	reqWkshTemplate := gtables.NewReqWkshTemplate(rec)
	err := reqWkshTemplate.GetOrError(rec.Worksheet_template_name)
	if err != nil {
		return err
	}
	reqWkshName := gtables.NewRequisitionWkshName(rec)
	return reqWkshName.GetOrError(map[string]interface{}{"worksheet_template_name": rec.Worksheet_template_name, "name": rec.Journal_batch_name})
}

// OnModify: a line with reservations still matches them (ReqLineVerifyChange)
func (requisitionLineLogic) OnModify(rec *gtables.RequisitionLine) error {
	return reservmgt.New(rec).ReqLineVerifyChange(rec)
}

// OnDelete: a line with reservations is not deleted
func (requisitionLineLogic) OnDelete(rec *gtables.RequisitionLine) error {
	return rec.TestField("reserved_qty_base", types.Decimal{})
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (requisitionLineLogic) OnRename(rec *gtables.RequisitionLine) error {
	return nil
}

// OnValidate_Type: a new type clears the number and variant
func (requisitionLineLogic) OnValidate_Type(rec *gtables.RequisitionLine) error {
	rec.No = ""
	rec.Variant_code = ""
	return nil
}

// OnValidate_No: a G/L account (direct posting) gives its name; an item (not blocked) its
// description, its vendor and its purchase unit of measure
func (requisitionLineLogic) OnValidate_No(rec *gtables.RequisitionLine) error {
	rec.Variant_code = ""
	if rec.No == "" {
		return nil
	}
	err := rec.TestField("type")
	if err != nil {
		return err
	}
	switch rec.Type {
	case gtables.RequisitionLine_Type.AccountGL:
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
	case gtables.RequisitionLine_Type.Item:
		item, err := getReqLineItem(rec)
		if err != nil {
			return err
		}
		err = item.TestField("blocked", false)
		if err != nil {
			return err
		}
		rec.Reserve = gtables.RequisitionLineReserve(item.Reserve)
		err = updateReqLineDescription(rec)
		if err != nil {
			return err
		}
		err = rec.Validate("vendor_no", item.Vendor_no)
		if err != nil {
			return err
		}
		err = rec.Validate("unit_of_measure_code", item.Purch_unit_of_measure)
		if err != nil {
			return err
		}
	}
	return nil
}

// OnValidate_Quantity: the quantity in the base unit and the vendor's quantity discount on the
// item (Item Purch. Qty. Disc.)
func (requisitionLineLogic) OnValidate_Quantity(rec *gtables.RequisitionLine) error {
	quantityBase, err := calcReqLineBaseQty(rec, rec.Quantity)
	if err != nil {
		return err
	}
	rec.Quantity_base = quantityBase
	rec.Quantity_disc_percent = types.Decimal{}
	if rec.Type == gtables.RequisitionLine_Type.Item {
		itemPurchQtyDisc := gtables.NewItemPurchQtyDisc(rec)
		itemPurchQtyDisc.SetRange("item_no", rec.No)
		itemPurchQtyDisc.SetRange("vendor_no", rec.Vendor_no)
		itemPurchQtyDisc.SetRange("minimum_quantity", types.Decimal{}, rec.Quantity_base.Abs())
		if itemPurchQtyDisc.Find("+") {
			rec.Quantity_disc_percent = itemPurchQtyDisc.Discount_percent
		}
	}
	return checkReservedNotAbove(gtables.RequisitionLineTableName, "reserved_qty_base", rec.Quantity_base, rec.Reserved_qty_base)
}

// OnValidate_Vendor_no: the vendor's dimension 1, dimension 2 and currency (not blocked); an item's
// vendor terms — the vendor's item no., the expected receipt date by the lead time, the unit of
// measure and direct unit cost (in the vendor's currency)
func (requisitionLineLogic) OnValidate_Vendor_no(rec *gtables.RequisitionLine) error {
	vend := gtables.NewVendor(rec)
	if rec.Vendor_no != "" {
		err := vend.GetOrError(rec.Vendor_no)
		if err != nil {
			return err
		}
		err = vend.TestField("blocked", false)
		if err != nil {
			return err
		}
		rec.Dimension_1_code = vend.Dimension_1_code
		rec.Dimension_2_code = vend.Dimension_2_code
		err = SetReqLineCurrency(rec, vend.Currency_code)
		if err != nil {
			return err
		}
		if rec.Type == gtables.RequisitionLine_Type.Item {
			err = updateReqLineDescription(rec)
			if err != nil {
				return err
			}
		}
		err = rec.Validate("quantity", rec.Quantity)
		if err != nil {
			return err
		}
	}

	rec.Order_address_code = ""

	if rec.Type == gtables.RequisitionLine_Type.Item && rec.No != "" {
		item, err := getReqLineItem(rec)
		if err != nil {
			return err
		}
		itemVend := gtables.NewItemVendor(rec)
		itemVend.Vendor_no = rec.Vendor_no
		itemVend.Starting_date = rec.Order_date
		err = FindItemVend(item, itemVend, false)
		if err != nil {
			return err
		}
		rec.Vendor_item_no = itemVend.Vendor_item_no
		err = UpdateReqLineOrderReceiptDate(rec, itemVend.Lead_time_calculation)
		if err != nil {
			return err
		}
		err = rec.Validate("unit_of_measure_code", itemVend.Unit_of_measure_code)
		if err != nil {
			return err
		}
		if itemVend.Currency_code != rec.Currency_code {
			err = SetReqLineCurrency(rec, itemVend.Currency_code)
			if err != nil {
				return err
			}
		}
		rec.Direct_unit_cost = itemVend.Direct_unit_cost
		if rec.Vendor_no != "" && vend.Currency_code != itemVend.Currency_code {
			err = SetReqLineCurrency(rec, vend.Currency_code)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// OnValidate_Order_date: the vendor's terms on the new date
func (requisitionLineLogic) OnValidate_Order_date(rec *gtables.RequisitionLine) error {
	return rec.Validate("vendor_no", rec.Vendor_no)
}

// OnValidate_Currency_code: the currency's exchange rate on the work date (the direct unit cost
// converted from the currency as last saved)
func (requisitionLineLogic) OnValidate_Currency_code(rec *gtables.RequisitionLine) error {
	xRec := rec.XRec()
	currencyCode := rec.Currency_code
	rec.Currency_code = xRec.Currency_code
	rec.Currency_factor = xRec.Currency_factor
	return SetReqLineCurrency(rec, currencyCode)
}

// OnValidate_Currency_factor: a currency needs a factor; a new factor converts the direct unit
// cost (from the factor as last saved)
func (requisitionLineLogic) OnValidate_Currency_factor(rec *gtables.RequisitionLine) error {
	xRec := rec.XRec()
	return setReqLineCurrencyFactor(rec, xRec.Currency_code, xRec.Currency_factor)
}

// OnValidate_Unit_of_measure_code: an item's quantity per unit and direct unit cost for the
// unit; the quantity again
func (requisitionLineLogic) OnValidate_Unit_of_measure_code(rec *gtables.RequisitionLine) error {
	if rec.Type == gtables.RequisitionLine_Type.Item && rec.No != "" {
		item, err := getReqLineItem(rec)
		if err != nil {
			return err
		}
		rec.Qty_per_unit_of_measure, err = GetQtyPerUnitOfMeasure(item, rec.Unit_of_measure_code)
		if err != nil {
			return err
		}
		err = updateReqLineDirectUnitCost(rec, item)
		if err != nil {
			return err
		}
	} else {
		rec.Qty_per_unit_of_measure = types.NewDecimalFromInt(1)
	}
	return rec.Validate("quantity", rec.Quantity)
}

// OnValidate_Quantity_base: only in the base unit
func (requisitionLineLogic) OnValidate_Quantity_base(rec *gtables.RequisitionLine) error {
	err := rec.TestField("qty_per_unit_of_measure", types.NewDecimalFromInt(1))
	if err != nil {
		return err
	}
	return rec.Validate("quantity", rec.Quantity_base)
}

// OnValidate_Variant_code: the variant's description (an item's only)
func (requisitionLineLogic) OnValidate_Variant_code(rec *gtables.RequisitionLine) error {
	err := updateReqLineDescription(rec)
	if err != nil {
		return err
	}
	if rec.Variant_code == "" {
		return nil
	}
	return rec.TestField("type", gtables.RequisitionLine_Type.Item)
}

// calcReqLineBaseQty (CalcBaseQty): a quantity in the base unit
func calcReqLineBaseQty(rec *gtables.RequisitionLine, qty types.Decimal) (types.Decimal, error) {
	err := rec.TestField("qty_per_unit_of_measure")
	if err != nil {
		return types.Decimal{}, err
	}
	return system.Round(qty.Mul(rec.Qty_per_unit_of_measure), types.MustDecimal("0.00001")), nil
}

// getReqLineItem (GetItem): the line's item
func getReqLineItem(rec *gtables.RequisitionLine) (*gtables.Item, error) {
	err := rec.TestField("no")
	if err != nil {
		return nil, err
	}
	item := gtables.NewItem(rec)
	err = item.GetOrError(rec.No)
	if err != nil {
		return nil, err
	}
	return item, nil
}

// getReqLineCurrency (GetCurrency): the line's currency, the local rounding precision without one
func getReqLineCurrency(rec *gtables.RequisitionLine) (*gtables.Currency, error) {
	currency := gtables.NewCurrency(rec)
	if rec.Currency_code == "" {
		InitRoundingPrecision(currency)
		return currency, nil
	}
	err := currency.GetOrError(rec.Currency_code)
	if err != nil {
		return nil, err
	}
	return currency, nil
}

// SetReqLineCurrency (VALIDATE("Currency Code")): the currency and its exchange rate on the work
// date; the direct unit cost converted from the line's currency before
func SetReqLineCurrency(rec *gtables.RequisitionLine, currencyCode types.Code) error {
	oldCurrencyCode := rec.Currency_code
	oldCurrencyFactor := rec.Currency_factor
	rec.Currency_code = currencyCode
	_, err := getReqLineCurrency(rec)
	if err != nil {
		return err
	}
	if rec.Currency_code != "" {
		currExchRate := ExchangeRates(rec)
		rec.Currency_factor, err = currExchRate.ExchangeRate(system.WorkDate(), rec.Currency_code)
		if err != nil {
			return err
		}
	} else {
		rec.Currency_factor = types.Decimal{}
	}
	return setReqLineCurrencyFactor(rec, oldCurrencyCode, oldCurrencyFactor)
}

// setReqLineCurrencyFactor (Currency Factor OnValidate): a currency needs a factor; a changed
// factor converts the direct unit cost from the old currency and factor, rounded
func setReqLineCurrencyFactor(rec *gtables.RequisitionLine, oldCurrencyCode types.Code, oldCurrencyFactor types.Decimal) error {
	if rec.Currency_code != "" {
		err := rec.TestField("currency_factor")
		if err != nil {
			return err
		}
	}
	if !rec.Currency_factor.Equal(oldCurrencyFactor) {
		currExchRate := ExchangeRates(rec)
		var err error
		if !oldCurrencyFactor.IsZero() {
			rec.Direct_unit_cost, err = currExchRate.ExchangeAmtFCYToLCY(system.WorkDate(), oldCurrencyCode, rec.Direct_unit_cost, oldCurrencyFactor)
			if err != nil {
				return err
			}
		}
		if !rec.Currency_factor.IsZero() {
			rec.Direct_unit_cost, err = currExchRate.ExchangeAmtLCYToFCY(system.WorkDate(), rec.Currency_code, rec.Direct_unit_cost, rec.Currency_factor)
			if err != nil {
				return err
			}
		}
		currency, err := getReqLineCurrency(rec)
		if err != nil {
			return err
		}
		err = currency.TestField("unit_amount_rounding_precision")
		if err != nil {
			return err
		}
		rec.Direct_unit_cost = system.Round(rec.Direct_unit_cost, currency.Unit_amount_rounding_precision)
	}
	return nil
}

// updateReqLineDirectUnitCost (UpdateDirectUnitCost): an item's direct unit cost from the
// vendor's terms in effect on the order date for the line's unit of measure (else the item's
// Last Direct Cost), in the line's currency; the vendor's item no. and the dates by the lead time
func updateReqLineDirectUnitCost(rec *gtables.RequisitionLine, item *gtables.Item) error {
	itemVend := gtables.NewItemVendor(rec)
	itemVend.Vendor_no = rec.Vendor_no
	itemVend.Unit_of_measure_code = rec.Unit_of_measure_code
	itemVend.Starting_date = rec.Order_date
	err := FindItemVend(item, itemVend, true)
	if err != nil {
		return err
	}
	rec.Vendor_item_no = itemVend.Vendor_item_no
	err = UpdateReqLineOrderReceiptDate(rec, itemVend.Lead_time_calculation)
	if err != nil {
		return err
	}
	rec.Direct_unit_cost = itemVend.Direct_unit_cost
	currency, err := getReqLineCurrency(rec)
	if err != nil {
		return err
	}
	currExchRate := ExchangeRates(rec)
	if itemVend.Currency_code != rec.Currency_code {
		err = currency.TestField("unit_amount_rounding_precision")
		if err != nil {
			return err
		}
		if itemVend.Currency_code != "" {
			exchangeRate, err := currExchRate.ExchangeRate(system.WorkDate(), itemVend.Currency_code)
			if err != nil {
				return err
			}
			rec.Direct_unit_cost, err = currExchRate.ExchangeAmtFCYToLCY(system.WorkDate(), itemVend.Currency_code, rec.Direct_unit_cost, exchangeRate)
			if err != nil {
				return err
			}
			rec.Direct_unit_cost = system.Round(rec.Direct_unit_cost, currency.Unit_amount_rounding_precision)
		}
		if rec.Currency_code != "" && itemVend.Currency_code != "" {
			rec.Direct_unit_cost, err = currExchRate.ExchangeAmtLCYToFCY(system.WorkDate(), rec.Currency_code, rec.Direct_unit_cost, rec.Currency_factor)
			if err != nil {
				return err
			}
			rec.Direct_unit_cost = system.Round(rec.Direct_unit_cost, currency.Unit_amount_rounding_precision)
		}
	}
	if rec.Currency_code != "" && itemVend.Currency_code == "" {
		err = rec.TestField("currency_factor")
		if err != nil {
			return err
		}
		err = currency.TestField("unit_amount_rounding_precision")
		if err != nil {
			return err
		}
		rec.Direct_unit_cost, err = currExchRate.ExchangeAmtLCYToFCY(system.WorkDate(), rec.Currency_code, rec.Direct_unit_cost, rec.Currency_factor)
		if err != nil {
			return err
		}
		rec.Direct_unit_cost = system.Round(rec.Direct_unit_cost, currency.Unit_amount_rounding_precision)
	}
	return nil
}

// UpdateReqLineOrderReceiptDate (UpdateOrderReceiptDate): the expected receipt date is the order
// date (else the work date) plus the lead time; no lead time, no date
func UpdateReqLineOrderReceiptDate(rec *gtables.RequisitionLine, leadTimeCalc types.Code) error {
	if leadTimeCalc == "" {
		rec.Expected_receipt_date = types.Date{}
		return nil
	}
	orderDate := rec.Order_date
	if orderDate.IsZero() {
		orderDate = system.WorkDate()
	}
	expectedReceiptDate, err := system.CalcDate(string(leadTimeCalc), orderDate)
	if err != nil {
		return apperrors.DateFormulaInvalid(gtables.ItemVendorTableName, "lead_time_calculation", leadTimeCalc.String())
	}
	rec.Expected_receipt_date = expectedReceiptDate
	return nil
}

// updateReqLineDescription (UpdateDescription): an item's or its variant's description, in the
// vendor's language when the item has a translation
func updateReqLineDescription(rec *gtables.RequisitionLine) error {
	if rec.Type != gtables.RequisitionLine_Type.Item || rec.No == "" {
		return nil
	}
	if rec.Variant_code == "" {
		item, err := getReqLineItem(rec)
		if err != nil {
			return err
		}
		rec.Description = item.Description
		rec.Description_2 = item.Description_2
	} else {
		itemVariant := gtables.NewItemVariant(rec)
		err := itemVariant.GetOrError(map[string]interface{}{"item_no": rec.No, "code": rec.Variant_code})
		if err != nil {
			return err
		}
		rec.Description = itemVariant.Description
		rec.Description_2 = itemVariant.Description_2
	}
	if rec.Vendor_no != "" {
		vend := gtables.NewVendor(rec)
		err := vend.GetOrError(rec.Vendor_no)
		if err != nil {
			return err
		}
		if vend.Language_code != "" {
			itemTranslation := gtables.NewItemTranslation(rec)
			if itemTranslation.Get(map[string]interface{}{"item_no": rec.No, "variant_code": rec.Variant_code, "language_code": vend.Language_code}) {
				rec.Description = itemTranslation.Description
				rec.Description_2 = itemTranslation.Description_2
			}
		}
	}
	return nil
}
