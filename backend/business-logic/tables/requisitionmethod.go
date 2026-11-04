package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// requisitionMethodLogic is the business logic of table 5408 Requisition_Method.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/requisitionmethod_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type requisitionMethodLogic struct{}

func init() { gtables.RegisterRequisitionMethodLogic(requisitionMethodLogic{}) }

// OnInsert runs before a new record is inserted
func (requisitionMethodLogic) OnInsert(rec *gtables.RequisitionMethod) error {
	return nil
}

// OnModify runs before a record is modified
func (requisitionMethodLogic) OnModify(rec *gtables.RequisitionMethod) error {
	return nil
}

// OnDelete runs before a record is deleted
func (requisitionMethodLogic) OnDelete(rec *gtables.RequisitionMethod) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (requisitionMethodLogic) OnRename(rec *gtables.RequisitionMethod) error {
	return nil
}

// GetItemReorderQty (codeunit 5400 Available Management GetItemReorderQty): the quantity to
// reorder of an item available qtyAvailable — without a requisition method by its reorder
// quantity up to the reorder point (no Maximum Inventory) or up to the Maximum Inventory; with
// one by its Reordering Policy (Fixed Reorder Qty., Maximum Qty.), rounded up to the item's Order
// Multiple
func GetItemReorderQty(item *gtables.Item, reqMethodCode types.Code, qtyAvailable types.Decimal) (types.Decimal, error) {
	var reorderQty types.Decimal
	zero := types.Decimal{}
	one := types.NewDecimalFromInt(1)
	if reqMethodCode == "" {
		if !item.Maximum_inventory.GreaterThan(zero) {
			if item.Reorder_quantity.GreaterThan(zero) {
				reorderQty = system.Round(item.Reorder_point.Sub(qtyAvailable).Div(item.Reorder_quantity), one, ">").Mul(item.Reorder_quantity)
			} else {
				reorderQty = item.Reorder_point.Sub(qtyAvailable)
			}
		} else {
			if item.Reorder_point.GreaterThan(item.Maximum_inventory) ||
				qtyAvailable.Add(item.Reorder_quantity).GreaterThan(item.Maximum_inventory) {
				reorderQty = zero
			} else {
				if item.Reorder_quantity.GreaterThan(zero) {
					reorderQty = system.Round(item.Maximum_inventory.Sub(qtyAvailable).Div(item.Reorder_quantity), one, "<").Mul(item.Reorder_quantity)
				} else {
					reorderQty = item.Maximum_inventory.Sub(qtyAvailable)
				}
			}
		}
	} else {
		reqMethod := gtables.NewRequisitionMethod(item)
		err := reqMethod.GetOrError(reqMethodCode)
		if err != nil {
			return zero, err
		}

		if reqMethod.Calc_below_reorder_point {
			reorderQty = item.Reorder_point.Sub(qtyAvailable)
		} else {
			reorderQty = qtyAvailable.Neg()
		}

		if !reorderQty.GreaterThan(zero) {
			return zero, nil
		}

		switch reqMethod.Reordering_policy {
		case gtables.RequisitionMethod_Reordering_policy.FixedReorderQty:
			reorderQty = item.Reorder_quantity
		case gtables.RequisitionMethod_Reordering_policy.MaximumQty:
			reorderQty = item.Maximum_inventory.Sub(qtyAvailable)
		}

		if !item.Order_multiple.IsZero() {
			reorderQty = system.Round(reorderQty, item.Order_multiple, ">")
		}
	}
	return reorderQty, nil
}
