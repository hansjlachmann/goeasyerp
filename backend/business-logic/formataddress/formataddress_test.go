package formataddress_test

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/formataddress"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func company(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	must(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	must(t, tables.CreateCompanyTables(db, "fa", database.DBTypeSQLite))
	src := ftables.DBSource(db, "fa", database.DBTypeSQLite)
	for _, c := range []struct {
		code, name string
		format     gtables.CountryRegionAddress_format
		contact    gtables.CountryRegionContact_address_format
	}{
		{"NO", "Norway", gtables.CountryRegion_Address_format.PostCodeCity, gtables.CountryRegion_Contact_address_format.Last},
		{"US", "USA", gtables.CountryRegion_Address_format.CityStatePostCode, gtables.CountryRegion_Contact_address_format.First},
		{"GB", "United Kingdom", gtables.CountryRegion_Address_format.BlankLinePostCodeCity, gtables.CountryRegion_Contact_address_format.AfterCompanyName},
	} {
		country := gtables.NewCountryRegion(src)
		country.Code = types.Code(c.code)
		country.Name = types.Text(c.name)
		country.Address_format = c.format
		country.Contact_address_format = c.contact
		must(t, country.InsertOrError(false))
	}
	return src
}

func lines(a formataddress.AddrArray) []string {
	var out []string
	for _, l := range a {
		out = append(out, l)
	}
	return out
}

func TestFormatAddr(t *testing.T) {
	src := company(t)
	cases := []struct {
		name    string
		country string
		want    [8]string
	}{
		// Post Code+City, contact last: blank Address 2 squeezed out
		{"post code + city, contact last", "NO", [8]string{"Nordkapp AS", "Stortingsgata 12", "0159 Oslo", "Norway", "Kari Nordmann"}},
		// City+State+Post Code, contact first
		{"city, state, post code, contact first", "US", [8]string{"Kari Nordmann", "Nordkapp AS", "Stortingsgata 12", "Oslo, OS  0159", "USA"}},
	}
	for _, c := range cases {
		state := ""
		if c.country == "US" {
			state = "OS"
		}
		got, err := formataddress.FormatAddr(src, "Nordkapp AS", "", "Kari Nordmann", "Stortingsgata 12", "", "Oslo", "0159", state, types.Code(c.country))
		must(t, err)
		if got != c.want {
			t.Errorf("%s: %q, want %q", c.name, lines(got), c.want)
		}
	}
	// Blank Line+Post Code+City: the post code line, the country and (after the company name)
	// the contact, the empty lines after
	got, err := formataddress.FormatAddr(src, "Nordkapp AS", "", "Kari", "Stortingsgata 12", "", "London", "W1", "", "GB")
	must(t, err)
	if want := [8]string{"Nordkapp AS", "Kari", "Stortingsgata 12", "", "W1 London", "United Kingdom"}; got != want {
		t.Errorf("blank line format: %q, want %q", lines(got), want)
	}
	// blank country: the General Ledger Setup's local format (Post Code+City, contact first)
	got, err = formataddress.FormatAddr(src, "Nordkapp AS", "", "", "Stortingsgata 12", "", "Oslo", "0159", "", "")
	must(t, err)
	if want := [8]string{"Nordkapp AS", "Stortingsgata 12", "0159 Oslo"}; got != want {
		t.Errorf("local format: %q, want %q", lines(got), want)
	}
}
