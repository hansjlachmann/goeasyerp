package reports

import (
	"strconv"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/formataddress"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 19 VAT- VIES Declaration Tax Auth: the printed VIES declaration (EC sales list) — the
// company's name, address and VAT registration no., the period (the end of the Posting Date
// filter), the total sales to other EU countries in thousands, and a line per customer: the
// country's EU Country Code, the customer's VAT registration no., the sales and the EU 3-party
// trade amount; a warning line for sales without a customer no.; date and signature at the end.
// The same sales as report 88: entries of the company's own country and of countries without an
// EU Country Code are left out.
const VIESDeclarationTaxAuthID = 19

func init() {
	Register(VIESDeclarationTaxAuthID, "vat-vies-declaration-tax-auth", func() Report { return &viesDeclarationTaxAuth{} })
}

type viesDeclarationTaxAuth struct {
	useAmtsInAddCurr bool
	companyCountry   types.Code
	euCountries      map[types.Code]bool
}

func (r *viesDeclarationTaxAuth) Definition() *Definition {
	return &Definition{
		Options: []Option{{Name: "show_amounts_in_add_currency", Type: "bool", Default: "false"}},
		OnPreReport: func(run *Run) error {
			r.useAmtsInAddCurr = run.OptionBool("show_amounts_in_add_currency")
			r.euCountries = map[types.Code]bool{}
			setReportHeadValues(run)
			glSetup := gtables.GetGeneralLedgerSetup(run)
			if r.useAmtsInAddCurr {
				err := glSetup.TestField("additional_reporting_currency")
				if err != nil {
					return err
				}
				run.SetValue("amounts_in", run.Text("AMOUNTS_IN", glSetup.Additional_reporting_currency))
			} else {
				err := glSetup.TestField("lcy_code")
				if err != nil {
					return err
				}
				run.SetValue("amounts_in", run.Text("AMOUNTS_IN", glSetup.Lcy_code))
			}
			return nil
		},
		DataItems: []*DataItem{{
			Name:                "TaxEntry",
			Table:               gtables.TaxEntryTableName,
			Key:                 []string{"country_code", "sell_to_buy_from_no", "entry_no"},
			TableView:           []ftables.FieldFilter{{Field: "type", Expression: strconv.Itoa(int(gtables.TaxEntry_Type.Sale))}},
			RequestFilterFields: []string{"tax_bus_posting_group", "tax_prod_posting_group", "posting_date"},
			GroupFields:         []string{"country_code", "sell_to_buy_from_no"},
			OnPreDataItem:       Trigger(r.taxEntryOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.taxEntryOnAfterGetRecord),
			OnGroupFooter:       GroupTrigger(r.taxEntryOnGroupFooter),
		}},
	}
}

// euSale: an entry of a country of the EU other than the company's own
func (r *viesDeclarationTaxAuth) euSale(run *Run, countryCode types.Code) bool {
	if countryCode == "" || countryCode == r.companyCountry {
		return false
	}
	eu, ok := r.euCountries[countryCode]
	if !ok {
		country := gtables.NewCountryRegion(run)
		eu = country.Get(countryCode) && country.Eu_country_code != ""
		r.euCountries[countryCode] = eu
	}
	return eu
}

// salesToCust: the entry's base, or its base in the additional reporting currency
func (r *viesDeclarationTaxAuth) salesToCust(rec *gtables.TaxEntry) types.Decimal {
	if r.useAmtsInAddCurr {
		return rec.Additional_currency_base
	}
	return rec.Base
}

// taxEntryOnPreDataItem: the company (its address and VAT registration no.), the period and the
// total of the sales declared (classic OnPreReport's CALCSUMS)
func (r *viesDeclarationTaxAuth) taxEntryOnPreDataItem(rec *gtables.TaxEntry, run *Run) error {
	companyInfo := gtables.GetCompanyInformation(run)
	r.companyCountry = companyInfo.Country_code
	companyAddr, err := formataddress.Company(companyInfo)
	if err != nil {
		return err
	}
	err = companyInfo.TestField("vat_registration_no")
	if err != nil {
		return err
	}
	run.SetValue("company_address", companyAddr)
	run.SetValue("vat_registration_no", companyInfo.Vat_registration_no.String())
	_, period := dateFilterRange(rec.GetFilter("posting_date"))
	if period.IsZero() {
		period = system.WorkDate()
	}
	run.SetValue("period", period)
	totalAmount := types.Decimal{}
	taxEntry := gtables.NewTaxEntry(run)
	taxEntry.CopyFilters(rec)
	for found := taxEntry.FindSetBuffered(); found; found = taxEntry.Next() {
		if r.euSale(run, taxEntry.Country_code) {
			totalAmount = totalAmount.Add(r.salesToCust(taxEntry))
		}
	}
	run.SetValue("total_sales_thousands", system.Round(totalAmount.Neg().Div(types.NewDecimalFromInt(1000)), types.NewDecimalFromInt(1), "<"))
	return nil
}

// taxEntryOnAfterGetRecord: the sale's amount (SalesToCust) and its EU 3-party trade part
func (r *viesDeclarationTaxAuth) taxEntryOnAfterGetRecord(rec *gtables.TaxEntry, run *Run) error {
	if !r.euSale(run, rec.Country_code) {
		return Skip()
	}
	salesToCust := r.salesToCust(rec)
	eu3PartyTradeAmt := types.Decimal{}
	if rec.Eu_3_party_trade {
		eu3PartyTradeAmt = salesToCust
	}
	run.SetValue("sales_to_cust", salesToCust.Neg())
	run.SetValue("eu_3_party_trade_amt", eu3PartyTradeAmt.Neg())
	return nil
}

// taxEntryOnGroupFooter: a customer's line — the customer has a VAT registration no., the country
// an EU Country Code; entries without a customer no. print a warning instead; a customer without
// sales prints nothing
func (r *viesDeclarationTaxAuth) taxEntryOnGroupFooter(rec *gtables.TaxEntry, group string, run *Run) error {
	if group != "sell_to_buy_from_no" {
		return Skip()
	}
	if run.GroupTotal("sales_to_cust").IsZero() && run.GroupTotal("eu_3_party_trade_amt").IsZero() {
		return Skip()
	}
	run.SetValue("warning_text", "")
	if rec.Sell_to_buy_from_no == "" {
		language := fcodeunits.CurrentLanguage()
		run.SetValue("warning_text", run.Text("NOT_FILLED_IN", rec.FieldCaption("sell_to_buy_from_no", language),
			rec.FieldCaption("type", language), run.Text("SALE")))
		return nil
	}
	cust := gtables.NewCustomer(run)
	err := cust.GetOrError(rec.Sell_to_buy_from_no)
	if err == nil {
		err = cust.TestField("vat_registration_no")
	}
	if err == nil {
		err = rec.TestField("country_code")
	}
	if err != nil {
		return err
	}
	country := gtables.NewCountryRegion(run)
	err = country.GetOrError(rec.Country_code)
	if err == nil {
		err = country.TestField("eu_country_code")
	}
	if err != nil {
		return err
	}
	run.SetValue("eu_country_code", country.Eu_country_code.String())
	run.SetValue("cust_vat_registration_no", cust.Vat_registration_no.String())
	return nil
}
