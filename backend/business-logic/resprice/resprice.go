// Package resprice is codeunits 220 Resource-Find Cost and 221 Resource-Find Price: the cost
// and the sales price of a resource for a work type (and currency).
package resprice

import (
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// FindResCost (codeunit 220, Run with a Resource Cost record whose Code is the resource): the
// cost of the resource for the record's work type — the resource's own cost, else its
// group's, else the cost for all resources, else the resource card's cost. A "% Extra" or
// "LCY Extra" cost adds to the cost found without the work type. rec gets the cost found.
func FindResCost(rec *gtables.ResourceCost) error {
	resCost := gtables.NewResourceCost(rec)
	copyResCost(resCost, rec)
	resCost2 := gtables.NewResourceCost(rec)
	copyResCost(resCost2, resCost)

	err := findResUnitCost(resCost)
	if err != nil {
		return err
	}

	hundred := types.NewDecimalFromInt(100)
	one := types.NewDecimalFromInt(1)
	switch resCost.Cost_type {
	case gtables.ResourceCost_Cost_type.Fixed:
	case gtables.ResourceCost_Cost_type.Extra:
		resCost2.Work_type_code = ""
		err = findResUnitCost(resCost2)
		if err != nil {
			return err
		}
		resCost.Direct_unit_cost = resCost2.Direct_unit_cost.Mul(one.Add(resCost.Direct_unit_cost.Div(hundred)))
		resCost.Unit_cost = resCost2.Unit_cost.Mul(one.Add(resCost.Unit_cost.Div(hundred)))
	case gtables.ResourceCost_Cost_type.LCYExtra:
		resCost2.Work_type_code = ""
		err = findResUnitCost(resCost2)
		if err != nil {
			return err
		}
		resCost.Direct_unit_cost = resCost2.Direct_unit_cost.Add(resCost.Direct_unit_cost)
		resCost.Unit_cost = resCost2.Unit_cost.Add(resCost.Unit_cost)
	}
	copyResCost(rec, resCost)
	return nil
}

// findResUnitCost (FindResUnitCost): the cost record nearest to the resource and work type
func findResUnitCost(nearestResCost *gtables.ResourceCost) error {
	code := nearestResCost.Code
	workTypeCode := nearestResCost.Work_type_code
	if nearestResCost.Get(map[string]interface{}{"type": gtables.ResourceCost_Type.Resource, "code": code, "work_type_code": workTypeCode}) {
		return nil
	}
	res := gtables.NewResource(nearestResCost)
	err := res.GetOrError(code)
	if err != nil {
		return err
	}
	if nearestResCost.Get(map[string]interface{}{"type": gtables.ResourceCost_Type.GroupResource, "code": res.Resource_group_no, "work_type_code": workTypeCode}) {
		return nil
	}
	if nearestResCost.Get(map[string]interface{}{"type": gtables.ResourceCost_Type.All, "code": "", "work_type_code": workTypeCode}) {
		return nil
	}
	nearestResCost.Init()
	nearestResCost.Type = gtables.ResourceCost_Type.Resource
	nearestResCost.Code = res.No
	nearestResCost.Work_type_code = workTypeCode
	nearestResCost.Cost_type = gtables.ResourceCost_Cost_type.Fixed
	nearestResCost.Direct_unit_cost = res.Direct_unit_cost
	nearestResCost.Unit_cost = res.Unit_cost
	return nil
}

func copyResCost(to, from *gtables.ResourceCost) {
	to.Type = from.Type
	to.Code = from.Code
	to.Work_type_code = from.Work_type_code
	to.Cost_type = from.Cost_type
	to.Direct_unit_cost = from.Direct_unit_cost
	to.Unit_cost = from.Unit_cost
}

// FindResPrice (codeunit 221, Run with a Resource Price record whose Code is the resource):
// the price of the resource for the record's project, work type and currency — the project's
// prices first, then those of every project (blank Project No.): the resource's own, its
// group's, then the price for all resources, each in the currency or else in local currency;
// without one the resource card's Unit Price in local currency. rec gets the price found.
func FindResPrice(rec *gtables.ResourcePrice) error {
	resPrice2 := gtables.NewResourcePrice(rec)
	found, err := findResPrice(rec, resPrice2)
	if err != nil {
		return err
	}
	if found {
		rec.Project_no = resPrice2.Project_no
		rec.Type = resPrice2.Type
		rec.Code = resPrice2.Code
		rec.Work_type_code = resPrice2.Work_type_code
		rec.Currency_code = resPrice2.Currency_code
		rec.Unit_price = resPrice2.Unit_price
	} else {
		res := gtables.NewResource(rec)
		err = res.GetOrError(rec.Code)
		if err != nil {
			return err
		}
		rec.Project_no = ""
		rec.Type = gtables.ResourcePrice_Type.Resource
		rec.Code = res.No
		rec.Currency_code = ""
		rec.Unit_price = res.Unit_price
	}
	return nil
}

// findResPrice (FindResPrice): the price nearest to the project, resource, work type and
// currency
func findResPrice(resPrice, resPrice2 *gtables.ResourcePrice) (bool, error) {
	get := func(projectNo types.Code, priceType gtables.ResourcePriceType, code types.Code, currencyCode types.Code) bool {
		return resPrice2.Get(map[string]interface{}{"project_no": projectNo, "type": priceType, "code": code, "work_type_code": resPrice.Work_type_code, "currency_code": currencyCode})
	}
	res := gtables.NewResource(resPrice)
	for _, projectNo := range []types.Code{resPrice.Project_no, ""} {
		if get(projectNo, gtables.ResourcePrice_Type.Resource, resPrice.Code, resPrice.Currency_code) {
			return true, nil
		}
		if get(projectNo, gtables.ResourcePrice_Type.Resource, resPrice.Code, "") {
			return true, nil
		}
		err := res.GetOrError(resPrice.Code)
		if err != nil {
			return false, err
		}
		if get(projectNo, gtables.ResourcePrice_Type.GroupResource, res.Resource_group_no, resPrice.Currency_code) {
			return true, nil
		}
		if get(projectNo, gtables.ResourcePrice_Type.GroupResource, res.Resource_group_no, "") {
			return true, nil
		}
		if get(projectNo, gtables.ResourcePrice_Type.All, "", resPrice.Currency_code) {
			return true, nil
		}
		if get(projectNo, gtables.ResourcePrice_Type.All, "", "") {
			return true, nil
		}
	}
	return false, nil
}
