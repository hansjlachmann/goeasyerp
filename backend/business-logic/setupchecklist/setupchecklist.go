// Package setupchecklist is codeunit 406 Setup Checklist Management: the Setup Checklist (page
// 531) lists the setup tables of a new company in the order to set them up, grouped in
// application areas — the tables of this installation (a table that comes with a later module
// appears when it is there). Each line opens the table's page (ShowData: the page's Table action
// and the No. of Records drilldown) and can copy the table's data from another company (Copy
// Data, the tables copyTables lists).
package setupchecklist

import (
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// checklistEntry is a line of the checklist (MakeChecklistLine: a table and the page it is set
// up on) or an application area starting (MakeAreaLine: its main table and name)
type checklistEntry struct {
	area  int
	name  string
	table int
	page  int
}

// checklist (MakeChecklist): the setup tables in their order
var checklist = []checklistEntry{
	{area: 15, name: "General Ledger"},
	{table: 79, page: 1},    // Company Information | Company Information
	{table: 50, page: 100},  // Accounting Period | Accounting Periods
	{table: 98, page: 118},  // General Ledger Setup | General Ledger Setup
	{table: 308, page: 456}, // No. Series | No. Series
	{table: 15, page: 16},   // G/L Account | Chart of Accounts
	{table: 250, page: 312}, // Gen. Business Posting Group | Gen. Business Posting Groups
	{table: 251, page: 313}, // Gen. Product Posting Group | Gen. Product Posting Groups
	{table: 252, page: 314}, // General Posting Setup | General Posting Setup
	{table: 323, page: 470}, // Tax Business Posting Group | Tax Business Posting Groups
	{table: 324, page: 471}, // Tax Product Posting Group | Tax Product Posting Groups
	{table: 325, page: 472}, // Tax Posting Setup | Tax Posting Setup
	{table: 84, page: 103},  // Acc. Schedule Name | Account Schedule Names
	{table: 333, page: 488}, // Acc. Sched. Column Layout Name | Acc.Sched. Column Layout Names
	{table: 95, page: 121},  // G/L Budget Name | G/L Budget Names
	{table: 255, page: 318}, // Tax Statement Template | Tax Statement Templates
	{table: 260, page: 310}, // Tariff Number | Tariff Numbers
	{table: 258, page: 308}, // Transaction Type | Transaction Types
	{table: 285, page: 406}, // Transaction Specification | Transaction Specifications
	{table: 259, page: 309}, // Transport Method | Transport Methods
	{table: 282, page: 394}, // Entry/Exit Point | Entry/Exit Points
	{table: 284, page: 405}, // Area | Areas
	{table: 320, page: 466}, // Tax Jurisdiction | Tax Jurisdictions
	{table: 321, page: 467}, // Tax Group | Tax Groups
	{table: 322, page: 468}, // Tax Detail | Tax Details
	{table: 318, page: 464}, // Tax Area | Tax Area
	{table: 319, page: 465}, // Tax Area Line | Tax Area Line
	{table: 230, page: 257}, // Source Code | Source Codes
	{table: 231, page: 259}, // Reason Code | Reason Codes
	{table: 7, page: 8},     // Standard Text | Standard Text Codes
	{table: 220, page: 240}, // Business Unit | Business Units
	{table: 11, page: 12},   // Dimension 1 | Dimension 1 values
	{table: 12, page: 13},   // Dimension 2 | Dimension 2 values
	{table: 225, page: 367}, // ZIP Code | ZIP Codes
	{table: 9, page: 10},    // Country | Countries
	{table: 8, page: 9},     // Language | Languages
	{table: 4, page: 5},     // Currency | Currencies
	{table: 270, page: 370}, // Bank Account | Bank Account Card
	{table: 277, page: 373}, // Bank Account Posting Group | Bank Account Posting Groups
	{area: 18, name: "Sales & Receivables"},
	{table: 311, page: 459}, // Sales & Receivables Setup | Sales & Receivables Setup
	{table: 18, page: 21},   // Customer | Customer Card
	{table: 92, page: 110},  // Customer Posting Group | Customer Posting Groups
	{table: 3, page: 4},     // Payment Terms | Payment Terms
	{table: 289, page: 427}, // Payment Method | Payment Methods
	{table: 292, page: 431}, // Reminder Terms | Reminder Terms
	{table: 5, page: 6},     // Finance Charge Terms | Finance Charge Terms
	{table: 10, page: 11},   // Shipment Method | Shipment Methods
	{table: 291, page: 428}, // Shipping Agent | Shipping Agents
	{table: 340, page: 512}, // Customer Discount Group | Customer Disc. Groups
	{table: 13, page: 14},   // Salesperson/Purchaser | Salespeople/Purchasers
	{area: 5000, name: "Contact Management"},
	{table: 5007, page: 5012}, // Contact Management Setup | Contact Management Setup
	{table: 5000, page: 5001}, // Prospect | Prospect Card
	{table: 5002, page: 5004}, // Contact | Contact Card
	{table: 5001, page: 5003}, // Prospect Status | Prospect Status
	{table: 286, page: 429},   // Territory | Territories
	{table: 5003, page: 5007}, // Contact Job Responsibility | Contact Job Responsibilities
	{table: 5011, page: 5017}, // Market Group Code | Market Group Codes
	{table: 5013, page: 5019}, // Trade Code | Trade Codes
	{area: 23, name: "Purchases & Payables"},
	{table: 312, page: 460}, // Purchases & Payables Setup | Purchases & Payables Setup
	{table: 23, page: 26},   // Vendor | Vendor Card
	{table: 93, page: 111},  // Vendor Posting Group | Vendor Posting Groups
	{area: 27, name: "Inventory"},
	{table: 313, page: 461}, // Inventory Setup | Inventory Setup
	{table: 336, page: 496}, // Reservation Setup | Reservation Setup
	{table: 27, page: 30},   // Item | Item Card
	{table: 94, page: 112},  // Inventory Posting Group | Inventory Posting Groups
	{table: 204, page: 209}, // Unit of Measure | Units of Measure
	{table: 6, page: 7},     // Price Group | Price Groups
	{table: 341, page: 513}, // Item Discount Group | Item Disc. Groups
	{table: 14, page: 15},   // Location | Locations
	{table: 42, page: 65},   // Rounding Method | Rounding Methods
	{area: 156, name: "Resources"},
	{table: 314, page: 462}, // Resources Setup | Resources Setup
	{table: 156, page: 76},  // Resource | Resource Card
	{table: 152, page: 72},  // Resource Group | Resource Groups
	{table: 200, page: 208}, // Work Type | Work Types
	{area: 167, name: "Projects"},
	{table: 315, page: 463}, // Projects Setup | Projects Setup
	{table: 208, page: 211}, // Project Posting Group | Project Posting Groups
	{table: 161, page: 82},  // Phase | Phases
	{table: 162, page: 83},  // Task | Tasks
	{table: 163, page: 84},  // Step | Steps
	{area: 5600, name: "Fixed Assets"},
	{table: 5603, page: 5607}, // FA Setup | Fixed Asset Setup
	{table: 5600, page: 5600}, // Fixed Asset | Fixed Asset Card
	{table: 5628, page: 5644}, // Insurance | Insurance Card
	{table: 5606, page: 5612}, // FA Posting Group | FA Posting Group Card
	{table: 5619, page: 5630}, // FA Journal Template | FA Journal Templates
	{table: 5622, page: 5637}, // FA Reclass. Journal Template | FA Reclass. Journal Templates
	{table: 5633, page: 5652}, // Insurance Journal Template | Insurance Journal Templates
	{table: 5611, page: 5610}, // Depreciation Book | Depreciation Book Card
	{table: 5607, page: 5615}, // FA Class | FA Classes
	{table: 5608, page: 5616}, // FA Subclass | FA Subclasses
	{table: 5609, page: 5617}, // FA Location | FA Locations
	{table: 5630, page: 5648}, // Insurance Type | Insurance Types
	{table: 5626, page: 5642}, // Maintenance | Maintenance
	{area: 5200, name: "Human Resources"},
	{table: 5218, page: 5233}, // Human Resources Setup | Human Resources Setup
	{table: 5200, page: 5200}, // Employee | Employee Card
	{table: 5206, page: 5210}, // Cause of Absence | Causes of Absence
	{table: 5210, page: 5214}, // Cause of Inactivity | Causes of Inactivity
	{table: 5217, page: 5215}, // Grounds for Termination | Grounds for Termination
	{table: 5211, page: 5217}, // Employment Contract | Employment Contracts
	{table: 5202, page: 5205}, // Qualification | Qualifications
	{table: 5204, page: 5208}, // Relative | Relatives
	{table: 5213, page: 5219}, // Misc. Article | Misc. Article Information
	{table: 5215, page: 5220}, // Confidential | Confidential
	{table: 5212, page: 5216}, // Employee Statistics Group | Employee Statistics Groups
	{table: 5209, page: 5213}, // Union | Unions
	{area: 10060, name: "Payroll"},
	{table: 10055, page: 10055}, // Payroll Posting Group | Payroll Posting Groups
	{table: 10054, page: 10054}, // Pay Cycle | Pay Cycles
	{table: 10053, page: 10053}, // Filing Status | Filing Statuses
	{table: 10072, page: 10072}, // Payroll Tax Authority | Payroll Tax Authorities
	{table: 10071, page: 10071}, // Tax Form Class | Payroll Tax Form Classes
	{table: 10063, page: 10063}, // Payroll Rate | Payroll Rate Codes
	{table: 10060, page: 10060}, // Payroll Control | Payroll Control Card
	{table: 10067, page: 10067}, // Payroll Group | Payroll Groups
	{table: 10085, page: 10085}, // Pay Control Schedule | Pay Control Schedules
	{table: 10086, page: 10086}, // Pay Control Schedule Column | Payroll Control Schedule

}

// copyTables (TransferContents): the tables Copy Data copies, each with the tables copied with
// it (a statement template's names and lines, a number series' lines and relationships …)
var copyTables = map[int][]int{
	231: {231}, 15: {15}, 250: {250}, 251: {251}, 252: {252}, 323: {323}, 324: {324}, 325: {325},
	255: {255, 257, 256}, 318: {318}, 319: {319}, 320: {320}, 321: {321}, 322: {322},
	84: {84, 85}, 333: {333, 334}, 9: {9}, 286: {286}, 284: {284}, 225: {225}, 8: {8}, 4: {4},
	282: {282}, 42: {42}, 291: {291}, 277: {277}, 308: {308, 309, 310}, 92: {92}, 10: {10}, 3: {3},
	289: {289}, 292: {292}, 5: {5}, 340: {340}, 93: {93}, 94: {94}, 341: {341}, 204: {204, 5402},
	260: {260}, 258: {258}, 285: {285}, 259: {259}, 200: {200}, 208: {208},
	5001: {5001}, 5003: {5003}, 5011: {5011}, 5013: {5013},
	5202: {5202}, 5204: {5204}, 5206: {5206}, 5210: {5210}, 5211: {5211}, 5212: {5212}, 5213: {5213}, 5215: {5215}, 5217: {5217},
	5606: {5606}, 5607: {5607}, 5608: {5608}, 5611: {5611}, 5619: {5619}, 5622: {5622}, 5630: {5630}, 5633: {5633},
	10055: {10055}, 10054: {10054}, 10053: {10053}, 10072: {10072}, 10071: {10071}, 10063: {10063}, 10085: {10085}, 10086: {10086},
}

// objectName is the name of a table or page of this installation (the Object table); ok false
// when there is none
func objectName(src ftables.Source, objectType gtables.ObjectType, id int) (string, bool) {
	object := gtables.NewObject(ftables.DBSource(src.GetDB(), "", src.GetDBType()))
	if !object.GetByPK(objectType, id) {
		return "", false
	}
	return object.Name.String(), true
}

// copyingAvailable: Copy Data copies the table (and the tables copied with it are there)
func copyingAvailable(src ftables.Source, tableID int) bool {
	ids, ok := copyTables[tableID]
	if !ok {
		return false
	}
	for _, id := range ids {
		_, exists := objectName(src, gtables.Object_Type.Table, id)
		if !exists {
			return false
		}
	}
	return true
}

// UpdateChecklist (UpdateChecklist): the company's checklist lines and areas made again from
// the checklist — changed lines updated (their status, dates and comments kept), lines of
// tables that are gone deleted, new ones inserted
func UpdateChecklist(src ftables.Source) error {
	var lines []*gtables.SetupChecklistLine
	var areas []*gtables.ApplicationAreaLine
	areaID, areaLineNo, lineNo := 0, 0, 0
	areaUsed := map[int]bool{}
	for _, entry := range checklist {
		if entry.area != 0 {
			areaID = entry.area
			areaLineNo++
			area := gtables.NewApplicationAreaLine(src)
			area.Init()
			area.Id = entry.area
			area.Name = types.NewText(entry.name)
			area.Line_no = areaLineNo
			areas = append(areas, area)
			continue
		}
		lineNo++
		tableName, exists := objectName(src, gtables.Object_Type.Table, entry.table)
		if !exists {
			continue // not in this installation (classic: not in the license)
		}
		line := gtables.NewSetupChecklistLine(src)
		line.Init()
		line.Table_id = entry.table
		line.Table_name = types.NewText(tableName)
		line.Page_id = entry.page
		pageName, _ := objectName(src, gtables.Object_Type.Page, entry.page)
		line.Page_name = types.NewText(pageName)
		line.Line_no = lineNo
		line.Copying_available = copyingAvailable(src, entry.table)
		line.Application_area_id = areaID
		areaUsed[areaID] = true
		lines = append(lines, line)
	}

	wanted := map[int]*gtables.SetupChecklistLine{}
	for _, line := range lines {
		wanted[line.Table_id] = line
	}
	setupChecklistLine := gtables.NewSetupChecklistLine(src)
	for found := setupChecklistLine.FindSetBuffered(); found; found = setupChecklistLine.Next() {
		line, ok := wanted[setupChecklistLine.Table_id]
		if !ok {
			stale := gtables.NewSetupChecklistLine(src)
			stale.Copy(setupChecklistLine)
			err := stale.DeleteOrError(true)
			if err != nil {
				return err
			}
			continue
		}
		delete(wanted, setupChecklistLine.Table_id)
		if setupChecklistLine.Line_no != line.Line_no {
			// the line moved in the checklist: again under its new number, its status kept
			moved := gtables.NewSetupChecklistLine(src)
			moved.Copy(setupChecklistLine)
			err := moved.DeleteOrError(false)
			if err != nil {
				return err
			}
			line.Starting_date = setupChecklistLine.Starting_date
			line.Ending_date = setupChecklistLine.Ending_date
			line.Responsible_id = setupChecklistLine.Responsible_id
			line.Status = setupChecklistLine.Status
			wanted[line.Table_id] = line
			continue
		}
		if setupChecklistLine.Table_name != line.Table_name || setupChecklistLine.Page_id != line.Page_id ||
			setupChecklistLine.Page_name != line.Page_name || setupChecklistLine.Copying_available != line.Copying_available ||
			setupChecklistLine.Application_area_id != line.Application_area_id {
			changed := gtables.NewSetupChecklistLine(src)
			changed.Copy(setupChecklistLine)
			changed.Table_name = line.Table_name
			changed.Page_id = line.Page_id
			changed.Page_name = line.Page_name
			changed.Copying_available = line.Copying_available
			changed.Application_area_id = line.Application_area_id
			err := changed.ModifyOrError(false)
			if err != nil {
				return err
			}
		}
	}
	for _, line := range lines {
		if wanted[line.Table_id] == nil {
			continue
		}
		err := line.InsertOrError(false)
		if err != nil {
			return err
		}
	}

	wantedAreas := map[int]*gtables.ApplicationAreaLine{}
	for _, area := range areas {
		if areaUsed[area.Id] {
			wantedAreas[area.Id] = area
		}
	}
	areaLine := gtables.NewApplicationAreaLine(src)
	for found := areaLine.FindSetBuffered(); found; found = areaLine.Next() {
		area, ok := wantedAreas[areaLine.Id]
		if !ok {
			stale := gtables.NewApplicationAreaLine(src)
			stale.Copy(areaLine)
			err := stale.DeleteOrError(true)
			if err != nil {
				return err
			}
			continue
		}
		delete(wantedAreas, areaLine.Id)
		if areaLine.Line_no != area.Line_no || areaLine.Name != area.Name {
			changed := gtables.NewApplicationAreaLine(src)
			changed.Copy(areaLine)
			changed.Line_no = area.Line_no
			changed.Name = area.Name
			err := changed.ModifyOrError(false)
			if err != nil {
				return err
			}
		}
	}
	for _, area := range areas {
		if wantedAreas[area.Id] == nil {
			continue
		}
		err := area.InsertOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}
