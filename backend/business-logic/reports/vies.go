package reports

import (
	"bytes"
	"strconv"

	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 88 VAT- VIES Declaration Disk: the sales to customers in other EU countries in the
// period of the Posting Date filter as the VIES declaration file — per country and customer the
// tax entries' base (and the EU 3-party trade part); the customer needs a country and a VAT
// registration no. Each group's entries get its Internal Ref. No. Entries of the company's own
// country and of countries without an EU Country Code are left out (the classic report expects
// the user to filter them away).
const VIESDeclarationDiskID = 88

func init() {
	Register(VIESDeclarationDiskID, "vat-vies-declaration-disk", func() Report { return &viesDeclarationDisk{} })
}

type viesGroup struct {
	countryCode, customerNo types.Code
	salesToCust, eu3Party   types.Decimal
}

type viesDeclarationDisk struct {
	groups           []*viesGroup
	period           types.Date
	useAmtsInAddCurr bool
}

func (r *viesDeclarationDisk) Definition() *Definition {
	return &Definition{
		Transaction: true,
		Options:     []Option{{Name: "show_amounts_in_add_currency", Type: "bool", Default: "false"}},
		OnPreReport: func(run *Run) error {
			r.useAmtsInAddCurr = run.OptionBool("show_amounts_in_add_currency")
			return nil
		},
		DataItems: []*DataItem{{
			Name:                "TaxEntry",
			Table:               gtables.TaxEntryTableName,
			Key:                 []string{"country_code", "sell_to_buy_from_no", "entry_no"},
			TableView:           []ftables.FieldFilter{{Field: "type", Expression: strconv.Itoa(int(gtables.TaxEntry_Type.Sale))}},
			RequestFilterFields: []string{"tax_bus_posting_group", "tax_prod_posting_group", "posting_date"},
			OnPreDataItem: Trigger(func(rec *gtables.TaxEntry, run *Run) error {
				_, r.period = dateFilterRange(rec.GetFilter("posting_date"))
				if r.period.IsZero() {
					r.period = system.WorkDate()
				}
				return nil
			}),
			OnAfterGetRecord: Trigger(r.taxEntryOnAfterGetRecord),
			OnPostDataItem:   Trigger(r.taxEntryOnPostDataItem),
		}},
	}
}

// taxEntryOnAfterGetRecord: the entry's base to its country's and customer's group
func (r *viesDeclarationDisk) taxEntryOnAfterGetRecord(rec *gtables.TaxEntry, run *Run) error {
	n := len(r.groups)
	if n == 0 || r.groups[n-1].countryCode != rec.Country_code || r.groupCustomer(n) != rec.Sell_to_buy_from_no {
		r.groups = append(r.groups, &viesGroup{countryCode: rec.Country_code, customerNo: rec.Sell_to_buy_from_no})
	}
	group := r.groups[len(r.groups)-1]
	salesToCust := rec.Base
	if r.useAmtsInAddCurr {
		salesToCust = rec.Additional_currency_base
	}
	group.salesToCust = group.salesToCust.Add(salesToCust)
	if rec.Eu_3_party_trade {
		group.eu3Party = group.eu3Party.Add(salesToCust)
	}
	return nil
}

func (r *viesDeclarationDisk) groupCustomer(n int) types.Code { return r.groups[n-1].customerNo }

// taxEntryOnPostDataItem: the file — a record per group with an amount, the total
func (r *viesDeclarationDisk) taxEntryOnPostDataItem(rec *gtables.TaxEntry, run *Run) error {
	companyInfo := gtables.GetCompanyInformation(run)
	vatRegNo := system.ConvertStr(companyInfo.Vat_registration_no.String(), "WwWw", "    ")
	var file bytes.Buffer
	writeLine := func(text string) { file.WriteString(fixedWidth(text, 80) + "\r\n") }
	writeLine("00" + fixedWidth(vatRegNo, 8) + "LIST")
	writeLine("0100001")
	internalReferenceNo := r.period.Format("0601") + "000000"
	noOfGrTotal := 0
	total := types.Decimal{}
	for _, g := range r.groups {
		// sales in the company's own country or a country outside the EU are no EU sales
		euCountry := gtables.NewCountryRegion(run)
		if g.countryCode == "" || g.countryCode == companyInfo.Country_code || !euCountry.Get(g.countryCode) || euCountry.Eu_country_code == "" {
			continue
		}
		one := types.NewDecimalFromInt(1)
		if system.Round(g.salesToCust.Abs(), one, "<").IsZero() && system.Round(g.eu3Party.Abs(), one, "<").IsZero() {
			continue
		}
		if g.customerNo == "" {
			return apperrors.FieldRequired(gtables.TaxEntryTableName, "sell_to_buy_from_no")
		}
		cust := gtables.NewCustomer(run)
		err := cust.GetOrError(g.customerNo)
		if err == nil {
			err = cust.TestField("country_code")
		}
		if err == nil {
			err = cust.TestField("vat_registration_no")
		}
		if err != nil {
			return err
		}
		country := gtables.NewCountryRegion(run)
		err = country.GetOrError(g.countryCode)
		if err == nil {
			err = country.TestField("eu_country_code")
		}
		if err != nil {
			return err
		}
		noOfGrTotal++
		internalReferenceNo = system.IncStr(internalReferenceNo)
		taxEntry := gtables.NewTaxEntry(run)
		taxEntry.CopyFilters(rec)
		taxEntry.SetRange("country_code", g.countryCode)
		taxEntry.SetRange("sell_to_buy_from_no", g.customerNo)
		var entries []*gtables.TaxEntry
		for found := taxEntry.FindSetBuffered(); found; found = taxEntry.Next() {
			entry := gtables.NewTaxEntry(run)
			entry.Copy(taxEntry)
			entries = append(entries, entry)
		}
		for _, entry := range entries {
			entry.Internal_ref_no = types.Text(internalReferenceNo)
			err = entry.ModifyOrError(false)
			if err != nil {
				return err
			}
		}
		writeLine("02" + fixedWidth(internalReferenceNo, 10) + r.period.Format("060102") + fixedWidth(vatRegNo, 8) +
			fixedWidth(country.Eu_country_code.String(), 2) + fixedWidth(cust.Vat_registration_no.String(), 12) +
			zeroNumber(g.salesToCust, 15) + numeralSign(g.salesToCust.Neg()) + "0" + zeroNumber(g.eu3Party, 15) + numeralSign(g.eu3Party.Neg()))
		total = total.Add(g.salesToCust).Add(g.eu3Party)
	}
	writeLine("10" + zeroText(strconv.Itoa(noOfGrTotal), 9) + zeroNumber(total, 15) + numeralSign(total.Neg()))
	run.WriteFile(run.Text("FILE_NAME", r.period.Format("0601")), file.Bytes())
	run.Message(run.Text("DONE", noOfGrTotal))
	return nil
}

// numeralSign (DecimalNumeralSign): + or - of an amount
func numeralSign(d types.Decimal) string {
	if d.IsNegative() {
		return "-"
	}
	return "+"
}
