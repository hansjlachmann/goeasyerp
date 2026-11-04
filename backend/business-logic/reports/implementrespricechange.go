package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 1193 Implement Res. Price Change: the suggested price changes become the resource
// prices (inserted or modified); afterwards the user may delete the suggestions.
const ImplementResPriceChangeID = 1193

func init() {
	Register(ImplementResPriceChangeID, "implement-res-price-change", func() Report { return &implementResPriceChange{} })
}

type implementResPriceChange struct {
	window *system.Window
}

func (r *implementResPriceChange) Definition() *Definition {
	return &Definition{
		DataItems: []*DataItem{{
			Name: "Resource_Price_Change", Table: "Resource_Price_Change", Key: []string{"project_no", "type", "code", "work_type_code", "currency_code"},
			RequestFilterFields: []string{"project_no", "type", "code", "currency_code"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem:      Trigger(r.onPostDataItem),
		}},
	}
}

func (r *implementResPriceChange) onPreDataItem(resPriceChange *gtables.ResourcePriceChange, run *Run) error {
	r.window = system.OpenWindow(run.Text("WINDOW"))
	return nil
}

func (r *implementResPriceChange) onAfterGetRecord(resPriceChange *gtables.ResourcePriceChange, run *Run) error {
	r.window.Update(1, resPriceChange.Type)
	r.window.Update(2, resPriceChange.Code)
	r.window.Update(3, resPriceChange.Work_type_code)
	r.window.Update(4, resPriceChange.Currency_code)
	resourcePrice := gtables.NewResourcePrice(run)
	resourcePrice.Project_no = resPriceChange.Project_no
	resourcePrice.Type = gtables.ResourcePriceType(resPriceChange.Type)
	resourcePrice.Code = resPriceChange.Code
	resourcePrice.Work_type_code = resPriceChange.Work_type_code
	resourcePrice.Currency_code = resPriceChange.Currency_code
	resourcePrice.Unit_price = resPriceChange.New_unit_price
	if !resourcePrice.Insert(false) {
		return resourcePrice.ModifyOrError(false)
	}
	return nil
}

// onPostDataItem asks whether to delete the suggestions that were carried out — the data
// item's records in its filters (classic: in the footer section, after a COMMIT)
func (r *implementResPriceChange) onPostDataItem(resPriceChange *gtables.ResourcePriceChange, run *Run) error {
	if system.GuiAllowed() && system.Confirm(run.Text("DELETE_SUGGESTIONS")) {
		resPriceChange.DeleteAll()
	}
	return nil
}
