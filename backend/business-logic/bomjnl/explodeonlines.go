package bomjnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Explode BOM on the sales and purchase lines (the subforms' Functions): codeunits 63
// Sales-Explode BOM and 73 Purch.-Explode BOM
func init() {
	functions.Register("table", gtables.SalesLineTableName, "ExplodeBOM", functions.For(SalesExplodeBOM), functions.WithDialog())
	functions.Register("table", gtables.PurchaseLineTableName, "ExplodeBOM", functions.For(PurchExplodeBOM), functions.WithDialog())
}

// bomLineDescription: a component's description on a document in a language — the component's,
// unless the item has a translation into the document's language (the line keeps the translated
// description its No. gave it)
func bomLineDescription(fromBOMComp *gtables.BOMComponent, languageCode types.Code, current types.Text) types.Text {
	if languageCode == "" {
		return fromBOMComp.Description
	}
	itemTranslation := gtables.NewItemTranslation(fromBOMComp)
	if !itemTranslation.Get(map[string]interface{}{"item_no": fromBOMComp.No, "variant_code": fromBOMComp.Variant_code, "language_code": languageCode}) {
		return fromBOMComp.Description
	}
	return current
}

// bomComponentQty: the line's base quantity × the component's quantity per, in the component
// line's unit of measure
func bomComponentQty(quantityBase types.Decimal, fromBOMComp *gtables.BOMComponent, qtyPerUnitOfMeasure types.Decimal) (types.Decimal, error) {
	item := gtables.NewItem(fromBOMComp)
	err := item.GetOrError(fromBOMComp.No)
	if err != nil {
		return types.Decimal{}, err
	}
	qtyPerBOMUnit, err := tables.GetQtyPerUnitOfMeasure(item, fromBOMComp.Unit_of_measure_code)
	if err != nil {
		return types.Decimal{}, err
	}
	if qtyPerUnitOfMeasure.IsZero() {
		qtyPerUnitOfMeasure = types.NewDecimalFromInt(1)
	}
	return system.Round(quantityBase.Mul(fromBOMComp.Quantity_per).Mul(qtyPerBOMUnit).Div(qtyPerUnitOfMeasure), unitPrecision), nil
}

// SalesExplodeBOM (codeunit 63 Sales-Explode BOM): a sales line of a bill of materials not shipped
// or reserved becomes a line per component (items and resources, the quantities multiplied by
// the line's); the line itself keeps only its description
func SalesExplodeBOM(rec *gtables.SalesLine) (fcodeunits.Result, error) {
	err := rec.TestField("type", gtables.SalesLine_Type.Item)
	if err == nil {
		err = rec.TestField("quantity_shipped", types.Decimal{})
	}
	if err == nil {
		err = rec.TestField("reserved_qty_base", types.Decimal{})
	}
	if err != nil {
		return fcodeunits.Result{}, err
	}
	salesHeader := gtables.NewSalesHeader(rec)
	err = salesHeader.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "no": rec.Document_no})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	fromBOMComp := gtables.NewBOMComponent(rec)
	fromBOMComp.SetRange("parent_item_no", rec.No)
	noOfBOMComp := fromBOMComp.Count()
	if noOfBOMComp == 0 {
		return fcodeunits.Result{}, apperrors.BOMNotBillOfMaterials(rec.No.String())
	}
	if !system.Confirm(i18n.GetInstance().Message("BOM_EXPLODE_CONFIRM", fcodeunits.CurrentLanguage())) {
		return fcodeunits.Result{Success: true}, nil
	}

	toSalesLine := gtables.NewSalesLine(rec)
	toSalesLine.SetRange("document_type", rec.Document_type)
	toSalesLine.SetRange("document_no", rec.Document_no)
	toSalesLine.Document_type = rec.Document_type
	toSalesLine.Document_no = rec.Document_no
	toSalesLine.Line_no = rec.Line_no
	var lineSpacing int
	if toSalesLine.Find(">") {
		lineSpacing = (toSalesLine.Line_no - rec.Line_no) / (1 + noOfBOMComp)
		if lineSpacing == 0 {
			return fcodeunits.Result{}, apperrors.BOMNoSpaceToExplode()
		}
	} else {
		lineSpacing = 10000
	}

	// The components the order or invoice takes: the availability warning (codeunit 311)
	if rec.Document_type == gtables.SalesLine_Document_type.Order || rec.Document_type == gtables.SalesLine_Document_type.Invoice {
		availBOMComp := gtables.NewBOMComponent(rec)
		availBOMComp.SetRange("parent_item_no", rec.No)
		availBOMComp.SetRange("type", gtables.BOMComponent_Type.Item)
		availBOMComp.SetFilter("no", "<>''")
		for found := availBOMComp.FindSetBuffered(); found; found = availBOMComp.Next() {
			item := gtables.NewItem(rec)
			err = item.GetOrError(availBOMComp.No)
			if err != nil {
				return fcodeunits.Result{}, err
			}
			toSalesLine = gtables.NewSalesLine(rec)
			toSalesLine.Copy(rec)
			toSalesLine.Line_no = 0
			toSalesLine.No = availBOMComp.No
			toSalesLine.Variant_code = availBOMComp.Variant_code
			toSalesLine.Unit_of_measure_code = availBOMComp.Unit_of_measure_code
			toSalesLine.Qty_per_unit_of_measure, err = tables.GetQtyPerUnitOfMeasure(item, availBOMComp.Unit_of_measure_code)
			if err != nil {
				return fcodeunits.Result{}, err
			}
			toSalesLine.Outstanding_quantity = system.Round(rec.Quantity_base.Mul(availBOMComp.Quantity_per), unitPrecision)
			if toSalesLine.Outstanding_quantity.IsPositive() {
				err = tables.ItemCheckAvailSalesLine(toSalesLine)
				if err != nil {
					return fcodeunits.Result{}, err
				}
			}
		}
	}

	// The line keeps its description only
	toSalesLine = gtables.NewSalesLine(rec)
	err = toSalesLine.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "document_no": rec.Document_no, "line_no": rec.Line_no})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	quantityBase := toSalesLine.Quantity_base
	description, description2 := toSalesLine.Description, toSalesLine.Description_2
	toSalesLine.Init()
	toSalesLine.Description = description
	toSalesLine.Description_2 = description2
	err = toSalesLine.ModifyOrError(false)
	if err != nil {
		return fcodeunits.Result{}, err
	}

	nextLineNo := rec.Line_no
	for found := fromBOMComp.FindSetBuffered(); found; found = fromBOMComp.Next() {
		toSalesLine = gtables.NewSalesLine(rec)
		toSalesLine.Init()
		toSalesLine.Document_type = rec.Document_type
		toSalesLine.Document_no = rec.Document_no
		nextLineNo = nextLineNo + lineSpacing
		toSalesLine.Line_no = nextLineNo
		switch fromBOMComp.Type {
		case gtables.BOMComponent_Type.Item:
			toSalesLine.Type = gtables.SalesLine_Type.Item
		case gtables.BOMComponent_Type.Resource:
			toSalesLine.Type = gtables.SalesLine_Type.Resource
		}
		if toSalesLine.Type != gtables.SalesLine_Type.Blank {
			err = toSalesLine.Validate("no", fromBOMComp.No)
			if err == nil {
				err = toSalesLine.Validate("variant_code", fromBOMComp.Variant_code)
			}
			if err != nil {
				return fcodeunits.Result{}, err
			}
			var quantity types.Decimal
			if toSalesLine.Type == gtables.SalesLine_Type.Item {
				quantity, err = bomComponentQty(quantityBase, fromBOMComp, toSalesLine.Qty_per_unit_of_measure)
				if err != nil {
					return fcodeunits.Result{}, err
				}
			} else {
				quantity = system.Round(quantityBase.Mul(fromBOMComp.Quantity_per), unitPrecision)
			}
			err = toSalesLine.Validate("quantity", quantity)
			if err != nil {
				return fcodeunits.Result{}, err
			}
		}
		toSalesLine.Description = bomLineDescription(fromBOMComp, salesHeader.Language_code, toSalesLine.Description)
		err = toSalesLine.InsertOrError(false)
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// PurchExplodeBOM (codeunit 73 Purch.-Explode BOM): a purchase line of a bill of materials not
// received or reserved becomes a line per item component; resources cannot be purchased and are
// left out (the user is told)
func PurchExplodeBOM(rec *gtables.PurchaseLine) (fcodeunits.Result, error) {
	err := rec.TestField("type", gtables.PurchaseLine_Type.Item)
	if err == nil {
		err = rec.TestField("quantity_received", types.Decimal{})
	}
	if err == nil {
		err = rec.TestField("reserved_qty_base", types.Decimal{})
	}
	if err != nil {
		return fcodeunits.Result{}, err
	}
	purchHeader := gtables.NewPurchaseHeader(rec)
	err = purchHeader.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "no": rec.Document_no})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	fromBOMComp := gtables.NewBOMComponent(rec)
	fromBOMComp.SetRange("parent_item_no", rec.No)
	noOfBOMComp := fromBOMComp.Count()
	if noOfBOMComp == 0 {
		return fcodeunits.Result{}, apperrors.BOMNotBillOfMaterials(rec.No.String())
	}
	language := fcodeunits.CurrentLanguage()
	if !system.Confirm(i18n.GetInstance().Message("BOM_EXPLODE_CONFIRM", language)) {
		return fcodeunits.Result{Success: true}, nil
	}

	toPurchLine := gtables.NewPurchaseLine(rec)
	toPurchLine.SetRange("document_type", rec.Document_type)
	toPurchLine.SetRange("document_no", rec.Document_no)
	toPurchLine.Document_type = rec.Document_type
	toPurchLine.Document_no = rec.Document_no
	toPurchLine.Line_no = rec.Line_no
	var lineSpacing int
	if toPurchLine.Find(">") {
		lineSpacing = (toPurchLine.Line_no - rec.Line_no) / (1 + noOfBOMComp)
		if lineSpacing == 0 {
			return fcodeunits.Result{}, apperrors.BOMNoSpaceToExplode()
		}
	} else {
		lineSpacing = 10000
	}

	toPurchLine = gtables.NewPurchaseLine(rec)
	err = toPurchLine.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "document_no": rec.Document_no, "line_no": rec.Line_no})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	quantityBase := toPurchLine.Quantity_base
	description, description2 := toPurchLine.Description, toPurchLine.Description_2
	toPurchLine.Init()
	toPurchLine.Description = description
	toPurchLine.Description_2 = description2
	err = toPurchLine.ModifyOrError(false)
	if err != nil {
		return fcodeunits.Result{}, err
	}

	nextLineNo := rec.Line_no
	for found := fromBOMComp.FindSetBuffered(); found; found = fromBOMComp.Next() {
		toPurchLine = gtables.NewPurchaseLine(rec)
		toPurchLine.Init()
		toPurchLine.Document_type = rec.Document_type
		toPurchLine.Document_no = rec.Document_no
		nextLineNo = nextLineNo + lineSpacing
		toPurchLine.Line_no = nextLineNo
		resource := false
		switch fromBOMComp.Type {
		case gtables.BOMComponent_Type.Item:
			toPurchLine.Type = gtables.PurchaseLine_Type.Item
			err = toPurchLine.Validate("no", fromBOMComp.No)
			if err == nil {
				err = toPurchLine.Validate("variant_code", fromBOMComp.Variant_code)
			}
			if err != nil {
				return fcodeunits.Result{}, err
			}
			quantity, err := bomComponentQty(quantityBase, fromBOMComp, toPurchLine.Qty_per_unit_of_measure)
			if err != nil {
				return fcodeunits.Result{}, err
			}
			err = toPurchLine.Validate("quantity", quantity)
			if err != nil {
				return fcodeunits.Result{}, err
			}
		case gtables.BOMComponent_Type.Resource:
			resource = true
		}
		toPurchLine.Description = bomLineDescription(fromBOMComp, purchHeader.Language_code, toPurchLine.Description)
		if resource {
			fcodeunits.Notify(system.StrSubstNo(i18n.GetInstance().Message("BOM_PURCH_RESOURCE_NOT_TRANSFERRED", language), fromBOMComp.No))
			continue
		}
		err = toPurchLine.InsertOrError(false)
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}
