package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// bomComponentLogic is the business logic of table 90 BOM Component: a line of an item's bill
// of materials — an item or a resource and its quantity per assembled unit.
type bomComponentLogic struct{}

func init() { gtables.RegisterBOMComponentLogic(bomComponentLogic{}) }

// OnInsert: the parent item exists
func (bomComponentLogic) OnInsert(rec *gtables.BOMComponent) error {
	item := gtables.NewItem(rec)
	return item.GetOrError(rec.Parent_item_no)
}

// OnValidate_Type: a new type starts without No. and variant
func (bomComponentLogic) OnValidate_Type(rec *gtables.BOMComponent) error {
	rec.No = ""
	rec.Variant_code = ""
	return nil
}

// OnValidate_No: the item's or resource's description and unit of measure; whether the item is
// a bill of materials itself
func (bomComponentLogic) OnValidate_No(rec *gtables.BOMComponent) error {
	err := rec.TestField("type")
	if err != nil {
		return err
	}
	rec.Variant_code = ""
	if rec.No == "" {
		return nil
	}

	switch rec.Type {
	case gtables.BOMComponent_Type.Item:
		item := gtables.NewItem(rec)
		err = item.GetOrError(rec.No)
		if err != nil {
			return err
		}
		item.CalcFields("bill_of_materials")
		rec.Bill_of_materials = item.Bill_of_materials
		rec.Description = item.Description
		rec.Unit_of_measure_code = item.Base_unit_of_measure
	case gtables.BOMComponent_Type.Resource:
		res := gtables.NewResource(rec)
		err = res.GetOrError(rec.No)
		if err != nil {
			return err
		}
		rec.Bill_of_materials = false
		rec.Description = res.Name
		rec.Unit_of_measure_code = res.Unit_of_measure_code
	}
	return nil
}

// OnValidate_Variant_code: only for an item; the variant's description
func (bomComponentLogic) OnValidate_Variant_code(rec *gtables.BOMComponent) error {
	if rec.Variant_code == "" {
		return nil
	}
	err := rec.TestField("type", gtables.BOMComponent_Type.Item)
	if err != nil {
		return err
	}
	err = rec.TestField("no")
	if err != nil {
		return err
	}
	itemVariant := gtables.NewItemVariant(rec)
	err = itemVariant.GetOrError(map[string]interface{}{"item_no": rec.No, "code": rec.Variant_code})
	if err != nil {
		return err
	}
	rec.Description = itemVariant.Description
	return nil
}
