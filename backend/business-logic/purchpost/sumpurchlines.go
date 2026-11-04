// Package purchpost is codeunit 90 Purch.-Post's SumPurchLines: the totals of a purchase
// document the statistics show and the posting's running totals (the posting itself comes with
// the purchase posting step).
package purchpost

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// QtyType is the quantity SumPurchLines totals ('General,Invoicing,Receiving'); quotes have only
// General (the lines' Quantity)
type QtyType int

const (
	General QtyType = iota
	Invoicing
	Receiving
)

// PurchTotals is what SumPurchLines returns: the totals of the lines in the document's currency
// (TotalPurchLine) and in LCY (TotalPurchLineLCY) and the tax
type PurchTotals struct {
	TotalPurchLine    gtables.PurchaseLine
	TotalPurchLineLCY gtables.PurchaseLine
	TaxAmount         types.Decimal
}

// sumPurch holds what SumPurchLines2 adds up (the codeunit's global variables)
type sumPurch struct {
	purchHeader       *gtables.PurchaseHeader
	currency          *gtables.Currency
	totalPurchLine    gtables.PurchaseLine
	totalPurchLineLCY gtables.PurchaseLine
	orgTotalPurchLine gtables.PurchaseLine
}

// SumPurchLines (SumPurchLines): the document's totals — each line's share of its quantity
// (General: all of it), rounded so the rounding differences fall on the total, also converted
// to LCY at the document's factor. Invoice rounding comes with posting.
func SumPurchLines(purchHeader *gtables.PurchaseHeader, qtyType QtyType) (PurchTotals, error) {
	s := &sumPurch{purchHeader: purchHeader}
	err := s.sumPurchLines2(qtyType)
	if err != nil {
		return PurchTotals{}, err
	}
	var totals PurchTotals
	totals.TaxAmount = s.totalPurchLine.Amount_including_tax.Sub(s.totalPurchLine.Amount)
	totals.TotalPurchLine = s.totalPurchLine
	totals.TotalPurchLineLCY = s.totalPurchLineLCY
	return totals, nil
}

func (s *sumPurch) sumPurchLines2(qtyType QtyType) error {
	err := s.initCurrency()
	if err != nil {
		return err
	}
	oldPurchLine := gtables.NewPurchaseLine(s.purchHeader)
	oldPurchLine.SetRange("document_type", s.purchHeader.Document_type)
	oldPurchLine.SetRange("document_no", s.purchHeader.No)
	for found := oldPurchLine.FindSet(); found; found = oldPurchLine.Next() {
		purchLine := *oldPurchLine
		var purchLineQty types.Decimal
		switch qtyType {
		case General:
			purchLineQty = purchLine.Quantity
		}
		if !purchLineQty.IsZero() {
			s.divideAmount(&purchLine, purchLineQty)
			if !purchLine.Amount.IsZero() {
				if s.totalPurchLine.Amount.IsZero() {
					s.totalPurchLine.Tax_percent = purchLine.Tax_percent
				} else {
					if !s.totalPurchLine.Tax_percent.Equal(purchLine.Tax_percent) {
						s.totalPurchLine.Tax_percent = types.Decimal{}
					}
				}
			}
			err := s.roundAmount(&purchLine, purchLineQty)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// initCurrency: the document's currency and its rounding (local currency: the General Ledger
// Setup's)
func (s *sumPurch) initCurrency() error {
	s.currency = gtables.NewCurrency(s.purchHeader)
	if s.purchHeader.Currency_code == "" {
		tables.InitRoundingPrecision(s.currency)
		return nil
	}
	err := s.currency.GetOrError(s.purchHeader.Currency_code)
	if err != nil {
		return err
	}
	return s.currency.TestField("amount_rounding_precision")
}

// divideAmount (DivideAmount): the line's amounts for a share of its quantity
func (s *sumPurch) divideAmount(purchLine *gtables.PurchaseLine, purchLineQty types.Decimal) {
	share := func(value types.Decimal) types.Decimal {
		return value.Mul(purchLineQty).Div(purchLine.Quantity)
	}
	purchLine.Amount = share(purchLine.Amount)
	purchLine.Tax_base_amount = share(purchLine.Tax_base_amount)
	purchLine.Amount_including_tax = share(purchLine.Amount_including_tax)
	purchLine.Line_discount_amount = share(purchLine.Line_discount_amount)
	purchLine.Inv_discount_amount = share(purchLine.Inv_discount_amount)
}

// roundAmount (RoundAmount): the line rounded so the running totals stay exact, added to the
// totals with its quantity, weights, volume and parcels; in LCY with its cost
func (s *sumPurch) roundAmount(purchLine *gtables.PurchaseLine, purchLineQty types.Decimal) error {
	precision := s.currency.Amount_rounding_precision
	incrAmount(&s.orgTotalPurchLine, purchLine)
	noTax := purchLine.Amount.Equal(purchLine.Amount_including_tax)
	purchLine.Amount_including_tax = system.Round(s.orgTotalPurchLine.Amount_including_tax, precision).Sub(s.totalPurchLine.Amount_including_tax)
	if noTax {
		purchLine.Amount = purchLine.Amount_including_tax
		purchLine.Tax_base_amount = purchLine.Amount_including_tax
	} else {
		purchLine.Amount = system.Round(s.orgTotalPurchLine.Amount, precision).Sub(s.totalPurchLine.Amount)
		purchLine.Tax_base_amount = system.Round(s.orgTotalPurchLine.Tax_base_amount, precision).Sub(s.totalPurchLine.Tax_base_amount)
	}
	purchLine.Line_discount_amount = system.Round(s.orgTotalPurchLine.Line_discount_amount, precision).Sub(s.totalPurchLine.Line_discount_amount)
	purchLine.Inv_discount_amount = system.Round(s.orgTotalPurchLine.Inv_discount_amount, precision).Sub(s.totalPurchLine.Inv_discount_amount)
	incrAmount(&s.totalPurchLine, purchLine)
	s.totalPurchLine.Net_weight = s.totalPurchLine.Net_weight.Add(system.Round(purchLineQty.Mul(purchLine.Net_weight), types.MustDecimal("0.00001")))
	s.totalPurchLine.Gross_weight = s.totalPurchLine.Gross_weight.Add(system.Round(purchLineQty.Mul(purchLine.Gross_weight), types.MustDecimal("0.00001")))
	s.totalPurchLine.Unit_volume = s.totalPurchLine.Unit_volume.Add(system.Round(purchLineQty.Mul(purchLine.Unit_volume), types.MustDecimal("0.00001")))
	s.totalPurchLine.Quantity = s.totalPurchLine.Quantity.Add(purchLineQty)
	if purchLine.Units_per_parcel.IsPositive() {
		s.totalPurchLine.Units_per_parcel = s.totalPurchLine.Units_per_parcel.Add(system.Round(purchLineQty.Div(purchLine.Units_per_parcel), types.NewDecimalFromInt(1), ">"))
	}
	if s.purchHeader.Currency_code != "" {
		var useDate types.Date
		if s.purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Quote {
			useDate = system.WorkDate()
		} else {
			useDate = s.purchHeader.Posting_date
		}
		currExchRate := tables.ExchangeRates(s.purchHeader)
		toLCY := func(totalFCY, totalLCY types.Decimal) (types.Decimal, error) {
			amount, err := currExchRate.ExchangeAmtFCYToLCY(useDate, s.purchHeader.Currency_code, totalFCY, s.purchHeader.Currency_factor)
			if err != nil {
				return types.Decimal{}, err
			}
			return system.Round(amount).Sub(totalLCY), nil
		}
		noTax = purchLine.Amount.Equal(purchLine.Amount_including_tax)
		var err error
		purchLine.Amount_including_tax, err = toLCY(s.totalPurchLine.Amount_including_tax, s.totalPurchLineLCY.Amount_including_tax)
		if err != nil {
			return err
		}
		if noTax {
			purchLine.Amount = purchLine.Amount_including_tax
		} else {
			purchLine.Amount, err = toLCY(s.totalPurchLine.Amount, s.totalPurchLineLCY.Amount)
			if err != nil {
				return err
			}
		}
		purchLine.Line_discount_amount, err = toLCY(s.totalPurchLine.Line_discount_amount, s.totalPurchLineLCY.Line_discount_amount)
		if err != nil {
			return err
		}
		purchLine.Inv_discount_amount, err = toLCY(s.totalPurchLine.Inv_discount_amount, s.totalPurchLineLCY.Inv_discount_amount)
		if err != nil {
			return err
		}
	}
	incrAmount(&s.totalPurchLineLCY, purchLine)
	return nil
}

// incrAmount (IncrAmount): a line's amounts added to a total
func incrAmount(totalPurchLine *gtables.PurchaseLine, purchLine *gtables.PurchaseLine) {
	totalPurchLine.Amount = totalPurchLine.Amount.Add(purchLine.Amount)
	totalPurchLine.Tax_base_amount = totalPurchLine.Tax_base_amount.Add(purchLine.Tax_base_amount)
	totalPurchLine.Amount_including_tax = totalPurchLine.Amount_including_tax.Add(purchLine.Amount_including_tax)
	totalPurchLine.Line_discount_amount = totalPurchLine.Line_discount_amount.Add(purchLine.Line_discount_amount)
	totalPurchLine.Inv_discount_amount = totalPurchLine.Inv_discount_amount.Add(purchLine.Inv_discount_amount)
}

// Totals are the running totals of SumPurchLines for codeunit 90's posting: each line's share
// of the amounts is rounded so the totals stay exact, in the document's currency and in LCY
type Totals struct {
	s sumPurch
}

// NewTotals starts the totals of a document
func NewTotals(purchHeader *gtables.PurchaseHeader) (*Totals, error) {
	t := &Totals{s: sumPurch{purchHeader: purchHeader}}
	err := t.s.initCurrency()
	if err != nil {
		return nil, err
	}
	return t, nil
}

// Currency is the document's currency (local currency: the General Ledger Setup's rounding)
func (t *Totals) Currency() *gtables.Currency { return t.s.currency }

// FCY and LCY are the totals so far in the document's currency and in LCY
func (t *Totals) FCY() *gtables.PurchaseLine { return &t.s.totalPurchLine }
func (t *Totals) LCY() *gtables.PurchaseLine { return &t.s.totalPurchLineLCY }

// InvoicedShare adds a line's share for a quantity to the totals and returns it in the
// document's currency and in LCY (DivideAmount, RoundAmount)
func (t *Totals) InvoicedShare(purchLine *gtables.PurchaseLine, qty types.Decimal) (*gtables.PurchaseLine, *gtables.PurchaseLine) {
	share := gtables.NewPurchaseLine(purchLine)
	share.Copy(purchLine)
	t.s.divideAmount(share, qty)
	before := t.s.totalPurchLine
	_ = t.s.roundAmount(share, qty)
	lineFCY := gtables.NewPurchaseLine(purchLine)
	lineFCY.Copy(purchLine)
	lineFCY.Amount = t.s.totalPurchLine.Amount.Sub(before.Amount)
	lineFCY.Amount_including_tax = t.s.totalPurchLine.Amount_including_tax.Sub(before.Amount_including_tax)
	lineFCY.Tax_base_amount = t.s.totalPurchLine.Tax_base_amount.Sub(before.Tax_base_amount)
	lineFCY.Line_discount_amount = t.s.totalPurchLine.Line_discount_amount.Sub(before.Line_discount_amount)
	lineFCY.Inv_discount_amount = t.s.totalPurchLine.Inv_discount_amount.Sub(before.Inv_discount_amount)
	lineFCY.Quantity = qty
	share.Quantity = qty
	return lineFCY, share
}
