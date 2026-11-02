package tables

import (
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// intrastatJnlLineLogic is the business logic of table 263 Intrastat Jnl. Line.
type intrastatJnlLineLogic struct{}

func init() { gtables.RegisterIntrastatJnlLineLogic(intrastatJnlLineLogic{}) }

// OnInsert: the line's batch exists
func (intrastatJnlLineLogic) OnInsert(rec *gtables.IntrastatJnlLine) error {
	intraJnlBatch := gtables.NewIntrastatJnlBatch(rec)
	return intraJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "name": rec.Journal_batch_name})
}

// OnValidate_Tariff_no: a line without an item takes the tariff number's description
func (intrastatJnlLineLogic) OnValidate_Tariff_no(rec *gtables.IntrastatJnlLine) error {
	err := rec.TestField("item_no", "")
	if err != nil {
		return err
	}
	return getIntraItemDescription(rec)
}

// OnValidate_Item_no: the item's name, weight, tariff number and country of origin (an item
// entered by hand; the item needs a tariff number)
func (intrastatJnlLineLogic) OnValidate_Item_no(rec *gtables.IntrastatJnlLine) error {
	err := rec.TestField("source_type", gtables.IntrastatJnlLine_Source_type.Blank)
	if err != nil {
		return err
	}
	item := gtables.NewItem(rec)
	if rec.Item_no != "" {
		err = item.GetOrError(rec.Item_no)
		if err == nil {
			err = item.TestField("tariff_no")
		}
		if err != nil {
			return err
		}
	}
	rec.Name = types.Text(system.CopyStr(item.Description.String(), 1, 30))
	err = rec.Validate("net_weight", item.Net_weight)
	if err != nil {
		return err
	}
	rec.Tariff_no = item.Tariff_no
	rec.Country_of_origin_code = item.Country_of_origin_code
	return getIntraItemDescription(rec)
}

// OnValidate_Net_weight: the total weight of the quantity
func (intrastatJnlLineLogic) OnValidate_Net_weight(rec *gtables.IntrastatJnlLine) error {
	if !rec.Quantity.IsZero() {
		rec.Total_weight = system.Round(rec.Net_weight.Mul(rec.Quantity), types.MustDecimal("0.00001"))
	} else {
		rec.Total_weight = system.Round(rec.Net_weight, types.MustDecimal("0.00001"))
	}
	return nil
}

// OnValidate_Amount: the statistical value (with the cost regulation)
func (intrastatJnlLineLogic) OnValidate_Amount(rec *gtables.IntrastatJnlLine) error {
	if !rec.Cost_regulation_percent.IsZero() {
		return rec.Validate("cost_regulation_percent", rec.Cost_regulation_percent)
	}
	rec.Statistical_value = rec.Amount.Add(rec.Indirect_cost)
	return nil
}

// OnValidate_Quantity: the total weight of the quantity
func (intrastatJnlLineLogic) OnValidate_Quantity(rec *gtables.IntrastatJnlLine) error {
	rec.Total_weight = system.Round(rec.Net_weight.Mul(rec.Quantity), types.MustDecimal("0.00001"))
	return nil
}

// OnValidate_Cost_regulation_percent: the indirect cost as a percent of the amount
func (intrastatJnlLineLogic) OnValidate_Cost_regulation_percent(rec *gtables.IntrastatJnlLine) error {
	one := types.NewDecimalFromInt(1)
	rec.Indirect_cost = system.Round(rec.Amount.Mul(rec.Cost_regulation_percent).Div(types.NewDecimalFromInt(100)), one)
	rec.Statistical_value = system.Round(rec.Amount.Add(rec.Indirect_cost), one)
	return nil
}

// OnValidate_Indirect_cost: an indirect cost of its own, no percent
func (intrastatJnlLineLogic) OnValidate_Indirect_cost(rec *gtables.IntrastatJnlLine) error {
	rec.Cost_regulation_percent = types.Decimal{}
	rec.Statistical_value = rec.Amount.Add(rec.Indirect_cost)
	return nil
}

// getIntraItemDescription (GetItemDescription): the tariff number's description and its
// Supplementary Units
func getIntraItemDescription(rec *gtables.IntrastatJnlLine) error {
	if rec.Tariff_no != "" {
		tariffNumber := gtables.NewTariffNumber(rec)
		err := tariffNumber.GetOrError(rec.Tariff_no)
		if err != nil {
			return err
		}
		rec.Item_description = tariffNumber.Description
		rec.Supplementary_units = tariffNumber.Supplementary_units
	} else {
		rec.Item_description = ""
	}
	return nil
}
