package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 522 Item/Cust. Discount Overview (classic form 522, a matrix of table 341 Item Discount
// Group against table 340 Customer Discount Group — page 514 turned around): a cell is the
// Discount % of the Cust./Item Discount of its item and customer discount groups. A value typed
// into a cell changes that discount — 0 deletes it — or, when there is none, a positive value
// inserts it (the matrix field's OnAfterValidate).
const itemCustDiscOverviewPageID = 522

func init() {
	pages.OnMatrixCalculate(itemCustDiscOverviewPageID, func(card *gtables.ItemDiscountGroup, itemDiscGroup *gtables.ItemDiscountGroup) ([]pages.MatrixColumn, map[string][]string, error) {
		// the columns: the customer discount groups
		customerDiscGroup := gtables.NewCustomerDiscountGroup(itemDiscGroup)
		var columns []pages.MatrixColumn
		column := map[types.Code]int{}
		for found := customerDiscGroup.FindSet(); found; found = customerDiscGroup.Next() {
			column[customerDiscGroup.Code] = len(columns)
			columns = append(columns, pages.MatrixColumn{Name: customerDiscGroup.Code.String(), DecimalPlaces: "0:5",
				DrilldownFilters: map[string]string{"customer_disc_group": customerDiscGroup.Code.String()}})
		}

		// the cells: the discounts
		values := map[string][]string{}
		for found := itemDiscGroup.FindSet(); found; found = itemDiscGroup.Next() {
			values[itemDiscGroup.Code.String()] = make([]string, len(columns))
		}
		custItemDisc := gtables.NewCustItemDiscount(itemDiscGroup)
		for found := custItemDisc.FindSet(); found; found = custItemDisc.Next() {
			i, ok := column[custItemDisc.Customer_disc_group]
			cells := values[custItemDisc.Item_discount_group.String()]
			if ok && cells != nil && !custItemDisc.Discount_percent.IsZero() {
				cells[i] = custItemDisc.Discount_percent.String()
			}
		}
		return columns, values, nil
	})
	pages.OnMatrixColumnValidate(itemCustDiscOverviewPageID, func(rec *gtables.ItemDiscountGroup, column map[string]string, value types.Decimal) error {
		custItemDisc := gtables.NewCustItemDiscount(rec)
		customerDiscGroup := types.Code(column["customer_disc_group"])
		if custItemDisc.Get(map[string]interface{}{"customer_disc_group": customerDiscGroup, "item_discount_group": rec.Code}) {
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
			custItemDisc.Customer_disc_group = customerDiscGroup
			custItemDisc.Item_discount_group = rec.Code
			err := custItemDisc.Validate("discount_percent", value)
			if err != nil {
				return err
			}
			return custItemDisc.InsertOrError(true)
		}
		return nil
	})
}
