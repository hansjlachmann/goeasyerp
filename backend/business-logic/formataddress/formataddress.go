// Package formataddress is codeunit 365 Format Address: the address of a company, customer,
// document … as the 8 lines printed on documents, in the country's address format (the General
// Ledger Setup's local format for a blank country) — name, contact, address, the post code and
// city line, county and country, blank lines squeezed out.
package formataddress

import (
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// AddrArray is an address as printed: up to 8 lines, the used ones first
type AddrArray [8]string

// zipCodeCityLength is the length of the post code and city line (classic Text[90])
const zipCodeCityLength = 90

// FormatAddr (FormatAddr): the lines of an address in the country's formats — Address Format
// (where the post code goes) and Contact Address Format (where the contact goes)
func FormatAddr(src ftables.Source, name, name2, contact, addr, addr2, city string, postCode types.Code, county string, countryCode types.Code) (AddrArray, error) {
	var addrArray AddrArray
	country := gtables.NewCountryRegion(src)
	if countryCode == "" {
		glSetup := gtables.GetGeneralLedgerSetup(src)
		country.Init()
		country.Address_format = gtables.CountryRegionAddress_format(glSetup.Local_address_format)
		country.Contact_address_format = gtables.CountryRegionContact_address_format(glSetup.Local_cont_addr_format)
	} else {
		err := country.GetOrError(countryCode)
		if err != nil {
			return addrArray, err
		}
	}
	var nameLineNo, name2LineNo, contLineNo, addrLineNo, addr2LineNo, zipCodeCityLineNo, countyLineNo, countryLineNo int
	switch country.Contact_address_format {
	case gtables.CountryRegion_Contact_address_format.First:
		nameLineNo, name2LineNo, contLineNo, addrLineNo, addr2LineNo, zipCodeCityLineNo, countyLineNo, countryLineNo = 2, 3, 1, 4, 5, 6, 7, 8
	case gtables.CountryRegion_Contact_address_format.AfterCompanyName:
		nameLineNo, name2LineNo, contLineNo, addrLineNo, addr2LineNo, zipCodeCityLineNo, countyLineNo, countryLineNo = 1, 2, 3, 4, 5, 6, 7, 8
	case gtables.CountryRegion_Contact_address_format.Last:
		nameLineNo, name2LineNo, contLineNo, addrLineNo, addr2LineNo, zipCodeCityLineNo, countyLineNo, countryLineNo = 1, 2, 8, 3, 4, 5, 6, 7
	}
	// the classic array is 1-based
	set := func(lineNo int, text string) { addrArray[lineNo-1] = text }
	set(nameLineNo, name)
	set(name2LineNo, name2)
	set(addrLineNo, addr)
	set(addr2LineNo, addr2)
	switch country.Address_format {
	case gtables.CountryRegion_Address_format.PostCodeCity, gtables.CountryRegion_Address_format.CityStatePostCode, gtables.CountryRegion_Address_format.CityPostCode:
		set(contLineNo, contact)
		zipCodeCityText, countyText := generateZIPCodeCity(city, postCode, county, country)
		set(zipCodeCityLineNo, zipCodeCityText)
		set(countyLineNo, countyText)
		set(countryLineNo, country.Name.String())
		system.CompressArray(addrArray[:])
	case gtables.CountryRegion_Address_format.BlankLinePostCodeCity:
		if contLineNo < zipCodeCityLineNo {
			set(contLineNo, contact)
		}
		system.CompressArray(addrArray[:])
		insertText := 1
		for index := 0; index < len(addrArray); index++ {
			if addrArray[index] == "" {
				switch insertText {
				case 2:
					addrArray[index], _ = generateZIPCodeCity(city, postCode, county, country)
				case 3:
					addrArray[index] = country.Name.String()
				case 4:
					if contLineNo > zipCodeCityLineNo {
						addrArray[index] = contact
					}
				}
				insertText++
			}
		}
	}
	return addrArray, nil
}

// generateZIPCodeCity (GenerateZIPCodeCity): the post code and city line — and the county,
// when it has its own line — in the country's address format
func generateZIPCodeCity(city string, postCode types.Code, county string, country *gtables.CountryRegion) (string, string) {
	zipCode := postCode.String()
	zipCodeCityText := ""
	countyText := ""
	switch country.Address_format {
	case gtables.CountryRegion_Address_format.PostCodeCity:
		if zipCode != "" {
			zipCodeCityText = system.DelStr(zipCode+" "+city, zipCodeCityLength+1)
		} else {
			zipCodeCityText = city
		}
		countyText = county
	case gtables.CountryRegion_Address_format.CityStatePostCode:
		if county != "" && zipCode != "" {
			zipCodeCityText = system.DelStr(city, zipCodeCityLength-system.StrLen(zipCode)-system.StrLen(county)-3) + ", " + county + "  " + zipCode
		} else {
			if zipCode == "" {
				zipCodeCityText = city
				countyText = county
			} else {
				if county == "" && zipCode != "" {
					zipCodeCityText = system.DelStr(city, zipCodeCityLength-system.StrLen(zipCode)-1) + ", " + zipCode
				}
			}
		}
	case gtables.CountryRegion_Address_format.CityPostCode:
		if zipCode != "" {
			zipCodeCityText = system.DelStr(city, zipCodeCityLength-system.StrLen(zipCode)-1) + ", " + zipCode
		} else {
			zipCodeCityText = city
		}
	case gtables.CountryRegion_Address_format.BlankLinePostCodeCity:
		if zipCode != "" {
			zipCodeCityText = system.DelStr(zipCode+" "+city, zipCodeCityLength+1)
		} else {
			zipCodeCityText = city
		}
	}
	return zipCodeCityText, countyText
}

// Company (Company): the company's address (Company Information; its country's format)
func Company(companyInfo *gtables.CompanyInformation) (AddrArray, error) {
	return FormatAddr(companyInfo, companyInfo.Name.String(), companyInfo.Name_2.String(), "", companyInfo.Address.String(), companyInfo.Address_2.String(),
		companyInfo.City.String(), companyInfo.Post_code, companyInfo.State.String(), companyInfo.Country_code)
}

// Customer (Customer): a customer's address with its contact
func Customer(cust *gtables.Customer) (AddrArray, error) {
	return FormatAddr(cust, cust.Name.String(), cust.Name_2.String(), cust.Contact.String(), cust.Address.String(), cust.Address_2.String(),
		cust.City.String(), cust.Post_code, cust.State.String(), cust.Country_code)
}

// SalesHeaderSellTo (SalesHeaderSellTo): a sales document's sell-to address
func SalesHeaderSellTo(salesHeader *gtables.SalesHeader) (AddrArray, error) {
	return FormatAddr(salesHeader, salesHeader.Sell_to_customer_name.String(), salesHeader.Sell_to_customer_name_2.String(), salesHeader.Sell_to_contact.String(),
		salesHeader.Sell_to_address.String(), salesHeader.Sell_to_address_2.String(), salesHeader.Sell_to_city.String(), salesHeader.Sell_to_post_code,
		salesHeader.Sell_to_state.String(), salesHeader.Sell_to_country_code)
}

// SalesHeaderBillTo (SalesHeaderBillTo): a sales document's bill-to address
func SalesHeaderBillTo(salesHeader *gtables.SalesHeader) (AddrArray, error) {
	return FormatAddr(salesHeader, salesHeader.Bill_to_name.String(), salesHeader.Bill_to_name_2.String(), salesHeader.Bill_to_contact.String(),
		salesHeader.Bill_to_address.String(), salesHeader.Bill_to_address_2.String(), salesHeader.Bill_to_city.String(), salesHeader.Bill_to_post_code,
		salesHeader.Bill_to_state.String(), salesHeader.Bill_to_country_code)
}

// SalesHeaderShipTo (SalesHeaderShipTo): a sales document's ship-to address
func SalesHeaderShipTo(salesHeader *gtables.SalesHeader) (AddrArray, error) {
	return FormatAddr(salesHeader, salesHeader.Ship_to_name.String(), salesHeader.Ship_to_name_2.String(), salesHeader.Ship_to_contact.String(),
		salesHeader.Ship_to_address.String(), salesHeader.Ship_to_address_2.String(), salesHeader.Ship_to_city.String(), salesHeader.Ship_to_post_code,
		salesHeader.Ship_to_state.String(), salesHeader.Ship_to_country_code)
}

// SalesShptBillTo / SalesShptShipTo, SalesInvBillTo / SalesInvShipTo, SalesCrMemoBillTo /
// SalesCrMemoShipTo: the addresses of the posted sales documents
func SalesShptBillTo(rec *gtables.SalesShipmentHeader) (AddrArray, error) {
	return FormatAddr(rec, rec.Bill_to_name.String(), rec.Bill_to_name_2.String(), rec.Bill_to_contact.String(),
		rec.Bill_to_address.String(), rec.Bill_to_address_2.String(), rec.Bill_to_city.String(), rec.Bill_to_post_code,
		rec.Bill_to_state.String(), rec.Bill_to_country_code)
}

func SalesShptShipTo(rec *gtables.SalesShipmentHeader) (AddrArray, error) {
	return FormatAddr(rec, rec.Ship_to_name.String(), rec.Ship_to_name_2.String(), rec.Ship_to_contact.String(),
		rec.Ship_to_address.String(), rec.Ship_to_address_2.String(), rec.Ship_to_city.String(), rec.Ship_to_post_code,
		rec.Ship_to_state.String(), rec.Ship_to_country_code)
}

func SalesInvBillTo(rec *gtables.SalesInvoiceHeader) (AddrArray, error) {
	return FormatAddr(rec, rec.Bill_to_name.String(), rec.Bill_to_name_2.String(), rec.Bill_to_contact.String(),
		rec.Bill_to_address.String(), rec.Bill_to_address_2.String(), rec.Bill_to_city.String(), rec.Bill_to_post_code,
		rec.Bill_to_state.String(), rec.Bill_to_country_code)
}

func SalesInvShipTo(rec *gtables.SalesInvoiceHeader) (AddrArray, error) {
	return FormatAddr(rec, rec.Ship_to_name.String(), rec.Ship_to_name_2.String(), rec.Ship_to_contact.String(),
		rec.Ship_to_address.String(), rec.Ship_to_address_2.String(), rec.Ship_to_city.String(), rec.Ship_to_post_code,
		rec.Ship_to_state.String(), rec.Ship_to_country_code)
}

func SalesCrMemoBillTo(rec *gtables.SalesCrMemoHeader) (AddrArray, error) {
	return FormatAddr(rec, rec.Bill_to_name.String(), rec.Bill_to_name_2.String(), rec.Bill_to_contact.String(),
		rec.Bill_to_address.String(), rec.Bill_to_address_2.String(), rec.Bill_to_city.String(), rec.Bill_to_post_code,
		rec.Bill_to_state.String(), rec.Bill_to_country_code)
}

func SalesCrMemoShipTo(rec *gtables.SalesCrMemoHeader) (AddrArray, error) {
	return FormatAddr(rec, rec.Ship_to_name.String(), rec.Ship_to_name_2.String(), rec.Ship_to_contact.String(),
		rec.Ship_to_address.String(), rec.Ship_to_address_2.String(), rec.Ship_to_city.String(), rec.Ship_to_post_code,
		rec.Ship_to_state.String(), rec.Ship_to_country_code)
}

// The purchase documents: the vendor (buy-from) and the ship-to address (the company or its location)

func PurchHeaderBuyFrom(rec *gtables.PurchaseHeader) (AddrArray, error) {
	return FormatAddr(rec, rec.Buy_from_vendor_name.String(), rec.Buy_from_vendor_name_2.String(), rec.Buy_from_contact.String(),
		rec.Buy_from_address.String(), rec.Buy_from_address_2.String(), rec.Buy_from_city.String(), rec.Buy_from_post_code,
		rec.Buy_from_state.String(), rec.Buy_from_country_code)
}

func PurchHeaderShipTo(rec *gtables.PurchaseHeader) (AddrArray, error) {
	return FormatAddr(rec, rec.Ship_to_name.String(), rec.Ship_to_name_2.String(), rec.Ship_to_contact.String(),
		rec.Ship_to_address.String(), rec.Ship_to_address_2.String(), rec.Ship_to_city.String(), rec.Ship_to_post_code,
		rec.Ship_to_state.String(), rec.Ship_to_country_code)
}

func PurchRcptBuyFrom(rec *gtables.PurchRcptHeader) (AddrArray, error) {
	return FormatAddr(rec, rec.Buy_from_vendor_name.String(), rec.Buy_from_vendor_name_2.String(), rec.Buy_from_contact.String(),
		rec.Buy_from_address.String(), rec.Buy_from_address_2.String(), rec.Buy_from_city.String(), rec.Buy_from_post_code,
		rec.Buy_from_state.String(), rec.Buy_from_country_code)
}

func PurchRcptShipTo(rec *gtables.PurchRcptHeader) (AddrArray, error) {
	return FormatAddr(rec, rec.Ship_to_name.String(), rec.Ship_to_name_2.String(), rec.Ship_to_contact.String(),
		rec.Ship_to_address.String(), rec.Ship_to_address_2.String(), rec.Ship_to_city.String(), rec.Ship_to_post_code,
		rec.Ship_to_state.String(), rec.Ship_to_country_code)
}

func PurchInvBuyFrom(rec *gtables.PurchInvHeader) (AddrArray, error) {
	return FormatAddr(rec, rec.Buy_from_vendor_name.String(), rec.Buy_from_vendor_name_2.String(), rec.Buy_from_contact.String(),
		rec.Buy_from_address.String(), rec.Buy_from_address_2.String(), rec.Buy_from_city.String(), rec.Buy_from_post_code,
		rec.Buy_from_state.String(), rec.Buy_from_country_code)
}

func PurchInvShipTo(rec *gtables.PurchInvHeader) (AddrArray, error) {
	return FormatAddr(rec, rec.Ship_to_name.String(), rec.Ship_to_name_2.String(), rec.Ship_to_contact.String(),
		rec.Ship_to_address.String(), rec.Ship_to_address_2.String(), rec.Ship_to_city.String(), rec.Ship_to_post_code,
		rec.Ship_to_state.String(), rec.Ship_to_country_code)
}

func PurchCrMemoBuyFrom(rec *gtables.PurchCrMemoHdr) (AddrArray, error) {
	return FormatAddr(rec, rec.Buy_from_vendor_name.String(), rec.Buy_from_vendor_name_2.String(), rec.Buy_from_contact.String(),
		rec.Buy_from_address.String(), rec.Buy_from_address_2.String(), rec.Buy_from_city.String(), rec.Buy_from_post_code,
		rec.Buy_from_state.String(), rec.Buy_from_country_code)
}

func PurchCrMemoShipTo(rec *gtables.PurchCrMemoHdr) (AddrArray, error) {
	return FormatAddr(rec, rec.Ship_to_name.String(), rec.Ship_to_name_2.String(), rec.Ship_to_contact.String(),
		rec.Ship_to_address.String(), rec.Ship_to_address_2.String(), rec.Ship_to_city.String(), rec.Ship_to_post_code,
		rec.Ship_to_state.String(), rec.Ship_to_country_code)
}
