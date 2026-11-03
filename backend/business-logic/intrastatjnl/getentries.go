package intrastatjnl

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// GetEntries is report 594 Get Item Ledger Entries: the purchases and sales between two dates
// with countries that have an Intrastat Code become lines of the batch — one per item entry not
// yet in an Intrastat journal, Receipt or Shipment by its sign, the amount the entry's (an entry
// not invoiced: the invoices of it in the period, the rest at the item's average cost / unit
// price), the item's tariff number, weight and country of origin, the cost regulation %. A batch
// with Amounts in Add. Currency takes the amounts in the additional reporting currency (at the
// ending date's rate; costs not invoiced at the entries' additional-currency costs). It returns
// the number of lines made. Not migrated: job ledger entries.
func GetEntries(intraJnlBatch *gtables.IntrastatJnlBatch, startDate, endDate types.Date, indirectCostPctReq types.Decimal) (int, error) {
	g := &getEntries{batch: intraJnlBatch, startDate: startDate, endDate: endDate}
	if intraJnlBatch.Amounts_in_add_currency {
		g.glSetup = gtables.GetGeneralLedgerSetup(intraJnlBatch)
		err := g.glSetup.TestField("additional_reporting_currency")
		if err != nil {
			return 0, err
		}
		g.addCurrencyFactor, err = tables.ExchangeRates(intraJnlBatch).ExchangeRate(endDate, g.glSetup.Additional_reporting_currency)
		if err != nil {
			return 0, err
		}
	}
	return g.run(indirectCostPctReq)
}

// getEntries holds the run of report 594 (its globals)
type getEntries struct {
	batch              *gtables.IntrastatJnlBatch
	startDate, endDate types.Date
	glSetup            *gtables.GeneralLedgerSetup
	addCurrencyFactor  types.Decimal
}

// exchangeToAddCurr: an LCY amount in the additional reporting currency at the ending date's rate
func (g *getEntries) exchangeToAddCurr(amount types.Decimal) (types.Decimal, error) {
	return tables.ExchangeRates(g.batch).ExchangeAmtLCYToFCY(g.endDate, g.glSetup.Additional_reporting_currency, amount, g.addCurrencyFactor)
}

func (g *getEntries) run(indirectCostPctReq types.Decimal) (int, error) {
	intraJnlBatch, startDate, endDate := g.batch, g.startDate, g.endDate
	intraJnlLine := gtables.NewIntrastatJnlLine(intraJnlBatch)
	intraJnlLine.SetRange("journal_template_name", intraJnlBatch.Journal_template_name)
	intraJnlLine.SetRange("journal_batch_name", intraJnlBatch.Name)
	intraJnlLine.LockTable()
	lineNo := 0
	if intraJnlLine.FindLast() {
		lineNo = intraJnlLine.Line_no
	}
	inserted := 0
	country := gtables.NewCountryRegion(intraJnlBatch)
	country.SetFilter("intrastat_code", "<>''")
	for found := country.FindSetBuffered(); found; found = country.Next() {
		itemLedgEntry := gtables.NewItemLedgerEntry(intraJnlBatch)
		itemLedgEntry.SetRange("country_code", country.Code)
		itemLedgEntry.SetFilter("entry_type", "0|1") // Purchase|Sale
		itemLedgEntry.SetRange("posting_date", startDate, endDate)
		itemLedgEntry.SetRange("drop_shipment", false)
		for ok := itemLedgEntry.FindSetBuffered(); ok; ok = itemLedgEntry.Next() {
			if itemLedgEntry.Quantity.IsZero() {
				continue
			}
			intraJnlLine2 := gtables.NewIntrastatJnlLine(intraJnlBatch)
			intraJnlLine2.SetCurrentKey("source_type", "source_entry_no")
			intraJnlLine2.SetRange("source_type", gtables.IntrastatJnlLine_Source_type.ItemEntry)
			intraJnlLine2.SetRange("source_entry_no", itemLedgEntry.Entry_no)
			if intraJnlLine2.FindFirst() {
				continue
			}
			amount := itemLedgEntry.Amount
			if intraJnlBatch.Amounts_in_add_currency {
				var err error
				amount, err = g.exchangeToAddCurr(amount)
				if err != nil {
					return inserted, err
				}
			}
			if itemLedgEntry.Invoiced_quantity.IsZero() && itemLedgEntry.Unit_amount.IsZero() && amount.IsZero() {
				var err error
				amount, err = g.notInvoicedAmount(itemLedgEntry)
				if err != nil {
					return inserted, err
				}
			}
			lineNo += 10000
			err := insertItemJnlLine(intraJnlBatch, lineNo, itemLedgEntry, amount, indirectCostPctReq)
			if err != nil {
				return inserted, err
			}
			inserted++
		}
	}
	return inserted, nil
}

// notInvoicedAmount: an entry not invoiced in full — the invoices of it in the period, the rest
// at the item's average cost (purchases) or unit price (sales)
func (g *getEntries) notInvoicedAmount(rec *gtables.ItemLedgerEntry) (types.Decimal, error) {
	addCurr := g.batch.Amounts_in_add_currency
	itemLedgEntry2 := gtables.NewItemLedgerEntry(rec)
	itemLedgEntry2.SetCurrentKey("closed_by_entry_no")
	itemLedgEntry2.SetRange("closed_by_entry_no", rec.Entry_no)
	itemLedgEntry2.SetRange("posting_date", g.startDate, g.endDate)
	itemLedgEntry2.SetRange("entry_type", rec.Entry_type)
	totalInvoicedQty := types.Decimal{}
	totalAmount := types.Decimal{}
	for found := itemLedgEntry2.FindSetBuffered(); found; found = itemLedgEntry2.Next() {
		if rec.Quantity.IsPositive() == itemLedgEntry2.Invoiced_quantity.IsPositive() {
			totalInvoicedQty = totalInvoicedQty.Add(itemLedgEntry2.Invoiced_quantity)
			if addCurr {
				// the invoice's amount at its additional-currency share of the cost (the classic
				// divides by the unit cost unchecked)
				if !itemLedgEntry2.Unit_cost.IsZero() {
					totalAmount = totalAmount.Add(itemLedgEntry2.Amount.Mul(itemLedgEntry2.Additional_currency_unit_cost).Div(itemLedgEntry2.Unit_cost))
				}
			} else {
				totalAmount = totalAmount.Add(itemLedgEntry2.Amount)
			}
		}
	}
	if !rec.Quantity.Equal(totalInvoicedQty) {
		item := gtables.NewItem(rec)
		if item.Get(rec.Item_no) {
			if rec.Entry_type == gtables.ItemLedgerEntry_Entry_type.Purchase {
				if addCurr {
					totalAmount = totalAmount.Add(rec.Quantity.Sub(totalInvoicedQty).Mul(item.Add_curr_average_cost))
				} else {
					totalAmount = totalAmount.Add(rec.Quantity.Sub(totalInvoicedQty).Mul(item.Average_cost))
				}
			} else {
				unitPrice := item.Unit_price
				if addCurr {
					var err error
					unitPrice, err = g.exchangeToAddCurr(unitPrice)
					if err != nil {
						return types.Decimal{}, err
					}
				}
				totalAmount = totalAmount.Add(rec.Quantity.Sub(totalInvoicedQty).Mul(unitPrice))
			}
		}
	}
	return totalAmount, nil
}

// insertItemJnlLine (InsertItemJnlLine): an item entry as a journal line
func insertItemJnlLine(intraJnlBatch *gtables.IntrastatJnlBatch, lineNo int, itemLedgEntry *gtables.ItemLedgerEntry, amount, indirectCostPctReq types.Decimal) error {
	one := types.NewDecimalFromInt(1)
	intraJnlLine := gtables.NewIntrastatJnlLine(intraJnlBatch)
	intraJnlLine.Init()
	intraJnlLine.Journal_template_name = intraJnlBatch.Journal_template_name
	intraJnlLine.Journal_batch_name = intraJnlBatch.Name
	intraJnlLine.Line_no = lineNo
	intraJnlLine.Date = itemLedgEntry.Posting_date
	intraJnlLine.Country_code = itemLedgEntry.Country_code
	intraJnlLine.Transaction_type = itemLedgEntry.Transaction_type
	intraJnlLine.Transport_method = itemLedgEntry.Transport_method
	intraJnlLine.Source_entry_no = itemLedgEntry.Entry_no
	intraJnlLine.Document_no = itemLedgEntry.Document_no
	intraJnlLine.Item_no = itemLedgEntry.Item_no
	intraJnlLine.Entry_exit_point = itemLedgEntry.Entry_exit_point
	intraJnlLine.Area = itemLedgEntry.Area
	intraJnlLine.Transaction_specification = itemLedgEntry.Transaction_specification
	if itemLedgEntry.Quantity.IsNegative() {
		intraJnlLine.Type = gtables.IntrastatJnlLine_Type.Shipment
	} else {
		intraJnlLine.Type = gtables.IntrastatJnlLine_Type.Receipt
	}
	intraJnlLine.Amount = system.Round(amount.Abs(), one)
	err := intraJnlLine.Validate("item_no", intraJnlLine.Item_no)
	if err != nil {
		return err
	}
	intraJnlLine.Source_type = gtables.IntrastatJnlLine_Source_type.ItemEntry
	err = intraJnlLine.Validate("quantity", system.Round(itemLedgEntry.Quantity.Abs(), one))
	if err == nil {
		err = intraJnlLine.Validate("cost_regulation_percent", indirectCostPctReq)
	}
	if err != nil {
		return err
	}
	return intraJnlLine.InsertOrError(true)
}
