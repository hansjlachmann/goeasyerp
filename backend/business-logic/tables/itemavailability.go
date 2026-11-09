package tables

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/format"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// ItemAvailability on the lines (the lines' Item Availability by → Period, Variant, Location,
// Bin): the item's availability page opens in lookup mode on the line's item, up to the line's
// date and at its variant, location and bin; the period, variant, location or bin the user picks
// replaces the line's date, variant, location or bin after a question.

// The availability types (classic option 'Date,Variant,Location,Bin') and their pages
const (
	ItemAvailabilityByDate     = 0
	ItemAvailabilityByVariant  = 1
	ItemAvailabilityByLocation = 2
	ItemAvailabilityByBin      = 3

	ItemAvailByPeriodsPageID  = 157
	ItemAvailByLocationPageID = 492
	ItemAvailByBinPageID      = 5412
	ItemAvailByVariantPageID  = 5414
)

func init() {
	functions.Register("table", gtables.SalesLineTableName, "ItemAvailability", functions.ForArgs(func(rec *gtables.SalesLine, args functions.Args) (fcodeunits.Result, error) {
		return SalesLineItemAvailability(rec, args.Int(0))
	}, "int"), functions.WithDialog())
	functions.Register("table", gtables.PurchaseLineTableName, "ItemAvailability", functions.ForArgs(func(rec *gtables.PurchaseLine, args functions.Args) (fcodeunits.Result, error) {
		return PurchLineItemAvailability(rec, args.Int(0))
	}, "int"), functions.WithDialog())
	functions.Register("table", gtables.ItemJournalLineTableName, "ItemAvailability", functions.ForArgs(func(rec *gtables.ItemJournalLine, args functions.Args) (fcodeunits.Result, error) {
		return ItemJnlLineItemAvailability(rec, args.Int(0))
	}, "int"), functions.WithDialog())
	functions.Register("table", gtables.BOMJournalLineTableName, "ItemAvailability", functions.ForArgs(func(rec *gtables.BOMJournalLine, args functions.Args) (fcodeunits.Result, error) {
		return BOMJnlLineItemAvailability(rec, args.Int(0))
	}, "int"), functions.WithDialog())
	functions.Register("table", gtables.RequisitionLineTableName, "ItemAvailability", functions.ForArgs(func(rec *gtables.RequisitionLine, args functions.Args) (fcodeunits.Result, error) {
		return ReqLineItemAvailability(rec, args.Int(0))
	}, "int"), functions.WithDialog())
}

// itemAvailabilityLine is what the lines have in common: the field the user's pick goes into
type itemAvailabilityLine interface {
	ftables.Source
	Validate(fieldName string, value interface{}) error
	FieldCaption(fieldName string, args ...string) string
	ModifyOrError(runTrigger bool) error
}

// SalesLineItemAvailability (Sales Line ItemAvailability): up to the Shipment Date
func SalesLineItemAvailability(salesLine *gtables.SalesLine, availabilityType int) (fcodeunits.Result, error) {
	err := salesLine.TestField("type", gtables.SalesLine_Type.Item)
	if err == nil {
		err = salesLine.TestField("no")
	}
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return lineItemAvailability(salesLine, availabilityType, salesLine.No, salesLine.Variant_code, salesLine.Location_code,
		salesLine.Bin_code, "shipment_date", salesLine.Shipment_date)
}

// PurchLineItemAvailability (Purchase Line ItemAvailability): up to the Expected Receipt Date
func PurchLineItemAvailability(purchLine *gtables.PurchaseLine, availabilityType int) (fcodeunits.Result, error) {
	err := purchLine.TestField("type", gtables.PurchaseLine_Type.Item)
	if err == nil {
		err = purchLine.TestField("no")
	}
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return lineItemAvailability(purchLine, availabilityType, purchLine.No, purchLine.Variant_code, purchLine.Location_code,
		purchLine.Bin_code, "expected_receipt_date", purchLine.Expected_receipt_date)
}

// ItemJnlLineItemAvailability (Item Journal Line ItemAvailability): up to the Posting Date
func ItemJnlLineItemAvailability(itemJnlLine *gtables.ItemJournalLine, availabilityType int) (fcodeunits.Result, error) {
	err := itemJnlLine.TestField("item_no")
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return lineItemAvailability(itemJnlLine, availabilityType, itemJnlLine.Item_no, itemJnlLine.Variant_code, itemJnlLine.Location_code,
		itemJnlLine.Bin_code, "posting_date", itemJnlLine.Posting_date)
}

// BOMJnlLineItemAvailability (BOM Journal Line ItemAvailability): up to the Posting Date
func BOMJnlLineItemAvailability(bomJnlLine *gtables.BOMJournalLine, availabilityType int) (fcodeunits.Result, error) {
	err := bomJnlLine.TestField("item_no")
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return lineItemAvailability(bomJnlLine, availabilityType, bomJnlLine.Item_no, bomJnlLine.Variant_code, bomJnlLine.Location_code,
		bomJnlLine.Bin_code, "posting_date", bomJnlLine.Posting_date)
}

// ReqLineItemAvailability (Requisition Line ItemAvailability): up to the Expected Receipt Date
func ReqLineItemAvailability(reqLine *gtables.RequisitionLine, availabilityType int) (fcodeunits.Result, error) {
	err := reqLine.TestField("type", gtables.RequisitionLine_Type.Item)
	if err == nil {
		err = reqLine.TestField("no")
	}
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return lineItemAvailability(reqLine, availabilityType, reqLine.No, reqLine.Variant_code, reqLine.Location_code,
		reqLine.Bin_code, "expected_receipt_date", reqLine.Expected_receipt_date)
}

// lineItemAvailability is the CASE AvailabilityType OF of the lines' ItemAvailability: the
// page by the type, with the line's other places as filters; a different pick is asked about,
// validated into the line and saved
func lineItemAvailability(line itemAvailabilityLine, availabilityType int, itemNo, variantCode, locationCode, binCode types.Code,
	dateField string, lineDate types.Date) (fcodeunits.Result, error) {
	item := gtables.NewItem(line)
	err := item.GetOrError(itemNo)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	item.SetRange("no", itemNo)
	if !lineDate.IsZero() {
		err = item.SetFlowFilter("date_filter", ".."+lineDate.String())
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}

	var pageID int
	var field string
	var oldValue string
	var setFilters map[string]types.Code
	switch availabilityType {
	case ItemAvailabilityByDate:
		pageID, field, oldValue = ItemAvailByPeriodsPageID, dateField, lineDate.String()
		setFilters = map[string]types.Code{"variant_filter": variantCode, "location_filter": locationCode, "bin_filter": binCode}
	case ItemAvailabilityByVariant:
		pageID, field, oldValue = ItemAvailByVariantPageID, "variant_code", string(variantCode)
		setFilters = map[string]types.Code{"location_filter": locationCode, "bin_filter": binCode}
	case ItemAvailabilityByLocation:
		pageID, field, oldValue = ItemAvailByLocationPageID, "location_code", string(locationCode)
		setFilters = map[string]types.Code{"variant_filter": variantCode, "bin_filter": binCode}
	case ItemAvailabilityByBin:
		pageID, field, oldValue = ItemAvailByBinPageID, "bin_code", string(binCode)
		setFilters = map[string]types.Code{"variant_filter": variantCode, "location_filter": locationCode}
	default:
		return fcodeunits.Result{Success: true}, nil
	}
	for flowFilter, value := range setFilters {
		err = item.SetFlowFilter(flowFilter, exactCodeFilter(value))
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}

	itemAvail := system.NewPage(pageID)
	itemAvail.LookupMode(true)
	itemAvail.SetRecord(item)
	itemAvail.SetTableView(item)
	if itemAvail.RunModal() != system.ActionLookupOK {
		return fcodeunits.Result{Success: true}, nil
	}
	lastValue := itemAvail.GetLastRow()
	if lastValue == oldValue {
		return fcodeunits.Result{Success: true}, nil
	}

	var newValue interface{} = types.Code(lastValue)
	language := fcodeunits.CurrentLanguage()
	shownOld, shownNew := oldValue, lastValue
	if availabilityType == ItemAvailabilityByDate {
		lastDate, err := types.NewDateFromString(lastValue)
		if err != nil || lastDate.IsZero() {
			return fcodeunits.Result{Success: true}, nil
		}
		newValue = lastDate
		shownOld, shownNew = format.Date(lineDate, language), format.Date(lastDate, language)
	}
	question := system.StrSubstNo(i18n.GetInstance().Message("ITEM_AVAIL_CHANGE_FIELD", language), line.FieldCaption(field), shownOld, shownNew)
	if !system.Confirm(question) {
		return fcodeunits.Result{Success: true}, nil
	}
	err = line.Validate(field, newValue)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	err = line.ModifyOrError(true)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}
