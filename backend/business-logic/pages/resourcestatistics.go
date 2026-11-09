package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// resStatisticsColumns are the period columns of the resource statistics pages (223, 230):
// this accounting period, this fiscal year, last fiscal year and the total (no date filter)
var resStatisticsColumns = []string{"this_period", "this_year", "last_year", "total"}

// resStatisticsValues are one period column's figures (classic arrays ResCapacity[i] …)
type resStatisticsValues struct {
	capacity        types.Decimal
	usageCost       types.Decimal
	unitPrice       types.Decimal    // Sales (Price): what was invoiced
	usageUnits      [2]types.Decimal // [0] not chargeable, [1] chargeable
	usagePrice      [2]types.Decimal
	totalUsageUnits types.Decimal
	unusedCapacity  types.Decimal
	chargeablePct   types.Decimal
	invoicedPct     types.Decimal
	profit          types.Decimal
	profitPct       types.Decimal
}

// resStatisticsSource is a resource or resource group with the FlowFields the statistics use
type resStatisticsSource interface {
	SetFlowFilter(field, expression string) error
	CalcFields(fieldNames ...string)
}

// Page 223 Resource Statistics and 230 Res. Gr. Statistics: for this accounting period, this
// fiscal year, last fiscal year and in total, the capacity and what was not used of it, the
// usage not chargeable and chargeable (units and price), what was invoiced, the usage cost and
// the profit (OnAfterGetRecord; the periods of the work date, DateFilter-Calc)
func init() {
	pages.OnAfterGetRecord(223, func(rec *gtables.Resource, page *pages.Control) {
		resDateFilter, resDateName := resStatisticsPeriods(rec)
		for i, column := range resStatisticsColumns {
			res := gtables.NewResource(rec)
			res.No = rec.No
			v := calcResStatistics(res, resDateFilter[i], func() resStatisticsValues {
				return resStatisticsValues{capacity: res.Capacity, usageCost: res.Usage_cost, unitPrice: res.Sales_price}
			}, func() (types.Decimal, types.Decimal) {
				return res.Usage_qty, res.Usage_price
			})
			setResStatisticsValues(page, column, resDateName[i], v)
		}
	})
	pages.OnAfterGetRecord(230, func(rec *gtables.ResourceGroup, page *pages.Control) {
		resGrDateFilter, resGrDateName := resStatisticsPeriods(rec)
		for i, column := range resStatisticsColumns {
			resGr := gtables.NewResourceGroup(rec)
			resGr.No = rec.No
			v := calcResStatistics(resGr, resGrDateFilter[i], func() resStatisticsValues {
				return resStatisticsValues{capacity: resGr.Capacity, usageCost: resGr.Usage_cost, unitPrice: resGr.Sales_price}
			}, func() (types.Decimal, types.Decimal) {
				return resGr.Usage_qty, resGr.Usage_price
			})
			setResStatisticsValues(page, column, resGrDateName[i], v)
		}
	})
}

// resStatisticsPeriods: the date filters and names of the columns (the total has neither)
func resStatisticsPeriods(src ftables.Source) ([4]string, [4]string) {
	currentDate := system.WorkDate()
	fiscalYearText := i18n.GetInstance().Message("FISCAL_YEAR_NAME", fcodeunits.CurrentLanguage())
	var dateFilter, dateName [4]string
	dateFilter[0], dateName[0] = tables.CreateAccountingPeriodFilter(src, currentDate, 0)
	dateFilter[1], dateName[1] = tables.CreateFiscalYearFilter(src, currentDate, 0, fiscalYearText)
	dateFilter[2], dateName[2] = tables.CreateFiscalYearFilter(src, currentDate, -1, fiscalYearText)
	return dateFilter, dateName
}

// calcResStatistics is one turn of the classic FOR i loop: Capacity, Usage (Cost) and Sales
// (Price) in the period, then Usage (Qty.)/(Price) not chargeable and chargeable. Without
// accounting periods a period's filter is blank, as classic: its amounts are the total.
func calcResStatistics(rec resStatisticsSource, dateFilter string, totals func() resStatisticsValues,
	usage func() (types.Decimal, types.Decimal)) resStatisticsValues {
	_ = rec.SetFlowFilter("date_filter", dateFilter)
	_ = rec.SetFlowFilter("chargeable_filter", "")
	rec.CalcFields("capacity", "usage_cost", "sales_price")
	v := totals()
	v.totalUsageUnits = types.ZeroDecimal()
	for j := 0; j < 2; j++ {
		chargeable := "No"
		if j == 1 {
			chargeable = "Yes"
		}
		_ = rec.SetFlowFilter("chargeable_filter", chargeable)
		rec.CalcFields("usage_qty", "usage_price")
		v.usageUnits[j], v.usagePrice[j] = usage()
		v.totalUsageUnits = v.totalUsageUnits.Add(v.usageUnits[j])
	}
	v.unusedCapacity = v.capacity.Sub(v.totalUsageUnits)
	v.chargeablePct = calcPercentage(v.usageUnits[1], v.totalUsageUnits)
	v.invoicedPct = calcPercentage(v.unitPrice, v.usagePrice[1])
	v.profit = v.unitPrice.Sub(v.usageCost)
	v.profitPct = calcPercentage(v.profit, v.unitPrice)
	return v
}

// setResStatisticsValues puts a column's figures into the page's calculated fields
func setResStatisticsValues(page *pages.Control, column, name string, v resStatisticsValues) {
	page.SetValue(column+"_name", name)
	page.SetValue(column+"_capacity", v.capacity)
	page.SetValue(column+"_unused_capacity", v.unusedCapacity)
	page.SetValue(column+"_not_chargeable_units", v.usageUnits[0])
	page.SetValue(column+"_not_chargeable_price", v.usagePrice[0])
	page.SetValue(column+"_chargeable_units", v.usageUnits[1])
	page.SetValue(column+"_chargeable_price", v.usagePrice[1])
	page.SetValue(column+"_chargeable_pct", v.chargeablePct)
	page.SetValue(column+"_invoiced", v.unitPrice)
	page.SetValue(column+"_invoiced_pct", v.invoicedPct)
	page.SetValue(column+"_usage_cost", v.usageCost)
	page.SetValue(column+"_profit", v.profit)
	page.SetValue(column+"_profit_pct", v.profitPct)
}
