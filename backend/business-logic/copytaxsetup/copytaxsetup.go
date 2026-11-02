// Package copytaxsetup is codeunit 399 Copy Tax Setup: the tax groups, jurisdictions, areas
// (with their lines) and details of another company are copied into the company — records
// that exist already are kept (classic IF INSERT THEN;: looked up first, as a failed insert
// would end a PostgreSQL transaction).
package copytaxsetup

import (
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The tables Copy Tax Setup copies (CopyTable[1..4])
const (
	TaxGroups = iota
	TaxJurisdictions
	TaxAreas
	TaxDetail
)

// Tables are the tables copied, by the numbers above (Tax Areas: with their lines)
var Tables = [4]string{gtables.TaxGroupTableName, gtables.TaxJurisdictionTableName, gtables.TaxAreaTableName, gtables.TaxDetailTableName}

// Progress gets the table being copied (TaxGroups …) and how far it is (0–10000)
type Progress func(table, progress int)

// CopyTaxInfo (CopyTaxInfo) copies the chosen tables of the source company into the company of
// src
func CopyTaxInfo(src ftables.Source, sourceCompany string, copyTable [4]bool, progress Progress) {
	if progress == nil {
		progress = func(int, int) {}
	}
	if copyTable[TaxGroups] {
		copyTaxGroup(src, sourceCompany, progress)
	}
	if copyTable[TaxJurisdictions] {
		copyTaxJurisdiction(src, sourceCompany, progress)
	}
	if copyTable[TaxAreas] {
		copyTaxArea(src, sourceCompany, progress)
	}
	if copyTable[TaxDetail] {
		copyTaxDetail(src, sourceCompany, progress)
	}
}

// share: the position of the current record as a progress bar (0–10000)
func share(current, count int) int {
	return (current*10000 + count/2) / count
}

func copyTaxJurisdiction(src ftables.Source, sourceCompany string, progress Progress) {
	taxJurisdiction := gtables.NewTaxJurisdiction(src)
	sourceTaxJurisdiction := gtables.NewTaxJurisdiction(src)
	if !sourceTaxJurisdiction.ChangeCompany(sourceCompany) {
		return
	}
	recordCount := sourceTaxJurisdiction.Count()
	currentRecord := 0
	for found := sourceTaxJurisdiction.FindSet(); found; found = sourceTaxJurisdiction.Next() {
		currentRecord++
		progress(TaxJurisdictions, share(currentRecord, recordCount))
		if taxJurisdiction.GetByPK(sourceTaxJurisdiction.Code) {
			continue
		}
		taxJurisdiction.Init()
		taxJurisdiction.TransferFields(sourceTaxJurisdiction)
		taxJurisdiction.Insert(false)
	}
}

func copyTaxGroup(src ftables.Source, sourceCompany string, progress Progress) {
	taxGroup := gtables.NewTaxGroup(src)
	sourceTaxGroup := gtables.NewTaxGroup(src)
	if !sourceTaxGroup.ChangeCompany(sourceCompany) {
		return
	}
	recordCount := sourceTaxGroup.Count()
	currentRecord := 0
	for found := sourceTaxGroup.FindSet(); found; found = sourceTaxGroup.Next() {
		currentRecord++
		progress(TaxGroups, share(currentRecord, recordCount))
		if taxGroup.GetByPK(sourceTaxGroup.Code) {
			continue
		}
		taxGroup.Init()
		taxGroup.TransferFields(sourceTaxGroup)
		taxGroup.Insert(false)
	}
}

func copyTaxArea(src ftables.Source, sourceCompany string, progress Progress) {
	taxAreaLine := gtables.NewTaxAreaLine(src)
	taxArea := gtables.NewTaxArea(src)
	sourceTaxAreaLine := gtables.NewTaxAreaLine(src)
	sourceTaxArea := gtables.NewTaxArea(src)
	if !sourceTaxArea.ChangeCompany(sourceCompany) {
		return
	}
	sourceTaxAreaLine.ChangeCompany(sourceCompany)
	recordCount := sourceTaxArea.Count()
	currentRecord := 0
	for found := sourceTaxArea.FindSet(); found; found = sourceTaxArea.Next() {
		currentRecord++
		progress(TaxAreas, share(currentRecord, recordCount))
		if !taxArea.GetByPK(sourceTaxArea.Code) {
			taxArea.Init()
			taxArea.TransferFields(sourceTaxArea)
			taxArea.Insert(false)
		}
		sourceTaxAreaLine.SetRange("tax_area", sourceTaxArea.Code)
		for lineFound := sourceTaxAreaLine.FindSet(); lineFound; lineFound = sourceTaxAreaLine.Next() {
			if taxAreaLine.GetByPK(sourceTaxAreaLine.Tax_area, sourceTaxAreaLine.Tax_jurisdiction_code) {
				continue
			}
			taxAreaLine.Init()
			taxAreaLine.TransferFields(sourceTaxAreaLine)
			taxAreaLine.Insert(false)
		}
	}
}

func copyTaxDetail(src ftables.Source, sourceCompany string, progress Progress) {
	sourceTaxDetail := gtables.NewTaxDetail(src)
	taxDetail := gtables.NewTaxDetail(src)
	if !sourceTaxDetail.ChangeCompany(sourceCompany) {
		return
	}
	recordCount := sourceTaxDetail.Count()
	currentRecord := 0
	for found := sourceTaxDetail.FindSet(); found; found = sourceTaxDetail.Next() {
		currentRecord++
		progress(TaxDetail, share(currentRecord, recordCount))
		if taxDetail.GetByPK(sourceTaxDetail.Tax_jurisdiction_code, sourceTaxDetail.Tax_group_code, sourceTaxDetail.Tax_type, sourceTaxDetail.Effective_date) {
			continue
		}
		taxDetail.Init()
		taxDetail.TransferFields(sourceTaxDetail)
		taxDetail.Insert(false)
	}
}
