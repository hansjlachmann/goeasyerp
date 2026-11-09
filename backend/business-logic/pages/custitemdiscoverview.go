package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 514 Cust./Item Discount Overview (classic form 514, a matrix of table 340 Customer
// Discount Group against table 341 Item Discount Group): a cell is the Discount % of the
// Cust./Item Discount of its customer and item discount groups. A value typed into a cell
// changes that discount — 0 deletes it — or, when there is none, a positive value inserts it
// (the matrix field's OnAfterValidate).
const custItemDiscOverviewPageID = 514

func init() {
	pages.OnMatrixCalculate(custItemDiscOverviewPageID, func(card *gtables.CustomerDiscountGroup, customerDiscGroup *gtables.CustomerDiscountGroup) ([]pages.MatrixColumn, map[string][]string, error) {
		// the columns: the item discount groups
		itemDiscGroup := gtables.NewItemDiscountGroup(customerDiscGroup)
		var columns []pages.MatrixColumn
		column := map[types.Code]int{}
		for found := itemDiscGroup.FindSet(); found; found = itemDiscGroup.Next() {
			column[itemDiscGroup.Code] = len(columns)
			columns = append(columns, pages.MatrixColumn{Name: itemDiscGroup.Code.String(), DecimalPlaces: "0:5",
				DrilldownFilters: map[string]string{"item_discount_group": itemDiscGroup.Code.String()}})
		}

		// the cells: the discounts
		values := map[string][]string{}
		for found := customerDiscGroup.FindSet(); found; found = customerDiscGroup.Next() {
			values[customerDiscGroup.Code.String()] = make([]string, len(columns))
		}
		custItemDisc := gtables.NewCustItemDiscount(customerDiscGroup)
		for found := custItemDisc.FindSet(); found; found = custItemDisc.Next() {
			i, ok := column[custItemDisc.Item_discount_group]
			cells := values[custItemDisc.Customer_disc_group.String()]
			if ok && cells != nil && !custItemDisc.Discount_percent.IsZero() {
				cells[i] = custItemDisc.Discount_percent.String()
			}
		}
		return columns, values, nil
	})
	pages.OnMatrixColumnValidate(custItemDiscOverviewPageID, func(rec *gtables.CustomerDiscountGroup, column map[string]string, value types.Decimal) error {
		custItemDisc := gtables.NewCustItemDiscount(rec)
		itemDiscountGroup := types.Code(column["item_discount_group"])
		if custItemDisc.Get(map[string]interface{}{"customer_disc_group": rec.Code, "item_discount_group": itemDiscountGroup}) {
			if value.IsZero() {
				return custItemDisc.DeleteOrError(true)
			}
			err := custItemDisc.Validate("discount_percent", value)
			if err != nil {
				return err
			}
			return custItemDisc.ModifyOrError(true)
		} else if value.IsPositive() {
			custItemDisc.Init()
			custItemDisc.Customer_disc_group = rec.Code
			custItemDisc.Item_discount_group = itemDiscountGroup
			err := custItemDisc.Validate("discount_percent", value)
			if err != nil {
				return err
			}
			return custItemDisc.InsertOrError(true)
		}
		return nil
	})
}
