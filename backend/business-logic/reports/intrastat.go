package reports

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/hansjlachmann/openerp/backend/business-logic/intrastatjnl"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The Intrastat reports, run from the Intrastat Journal (page 311) on its batch:
//   - 594 Get Item Ledger Entries: the purchases and sales of the period (from the batch's
//     Statistics Period) with countries that have an Intrastat Code as journal lines — a line
//     per item entry not yet in a journal, Receipt or Shipment by its sign, the amount the entry's
//     (when it is not invoiced: the invoiced part plus the rest at the item's average cost /
//     unit price), the item's tariff number, weight and country of origin;
//   - 502 Intrastat - Checklist: the batch's lines by type, country, tariff number, transaction
//     type and transport method, each group's totals, the batch's total;
//   - 593 Intrastat - Make Disk Tax Auth: the declaration as a file (one record per group with an
//     internal reference number; the batch is then Reported).
const (
	IntrastatGetEntriesID = 594
	IntrastatChecklistID  = 502
	IntrastatMakeDiskID   = 593
)

func init() {
	Register(IntrastatGetEntriesID, "get-item-ledger-entries", func() Report { return &intrastatGetEntries{} })
	Register(IntrastatChecklistID, "intrastat-checklist", func() Report { return &intrastatChecklist{} })
	Register(IntrastatMakeDiskID, "intrastat-make-disk-tax-auth", func() Report { return &intrastatMakeDisk{} })
}

// intrastatBatchOf: the batch of the journal the report is run from (the context line, or the
// page's filters on an empty journal)
func intrastatBatchOf(run *Run) (*gtables.IntrastatJnlBatch, bool) {
	intraJnlLine, ok := run.Context().(*gtables.IntrastatJnlLine)
	if !ok {
		return nil, false
	}
	templateName := intraJnlLine.Journal_template_name
	if templateName == "" {
		templateName = types.Code(intraJnlLine.GetFilter("journal_template_name"))
	}
	batchName := intraJnlLine.Journal_batch_name
	if batchName == "" {
		batchName = types.Code(intraJnlLine.GetFilter("journal_batch_name"))
	}
	intraJnlBatch := gtables.NewIntrastatJnlBatch(run)
	if !intraJnlBatch.Get(map[string]interface{}{"journal_template_name": templateName, "name": batchName}) {
		return nil, false
	}
	return intraJnlBatch, true
}

// Report 594 Get Item Ledger Entries (intrastatjnl.GetEntries)
type intrastatGetEntries struct{}

func (r *intrastatGetEntries) Definition() *Definition {
	return &Definition{
		Options: []Option{
			{Name: "starting_date", Type: "date", Required: true},
			{Name: "ending_date", Type: "date", Required: true},
			{Name: "cost_regulation_percent", Type: "number"},
		},
		Transaction:  true,
		ContextTable: gtables.IntrastatJnlLineTableName,
		OnInitReport: r.onInitReport,
		OnPreReport:  r.onPreReport,
	}
}

// onInitReport: the batch's Statistics Period as the dates
func (r *intrastatGetEntries) onInitReport(run *Run) error {
	intraJnlBatch, ok := intrastatBatchOf(run)
	if !ok {
		return nil
	}
	startDate, endDate, ok := intrastatjnl.PeriodDates(intraJnlBatch.Statistics_period, system.WorkDate())
	if ok {
		run.SetOption("starting_date", startDate.String())
		run.SetOption("ending_date", endDate.String())
	}
	return nil
}

func (r *intrastatGetEntries) onPreReport(run *Run) error {
	intraJnlBatch, ok := intrastatBatchOf(run)
	if !ok {
		return fmt.Errorf("report %d runs from an Intrastat journal", IntrastatGetEntriesID)
	}
	inserted, err := intrastatjnl.GetEntries(intraJnlBatch, run.OptionDate("starting_date"), run.OptionDate("ending_date"),
		run.OptionDecimal("cost_regulation_percent"))
	if err != nil {
		return err
	}
	run.Message(run.Text("LINES_CREATED", inserted))
	return nil
}

// intrastatGroupKey: the fields a declaration groups the lines by
func intrastatGroupKey(line *gtables.IntrastatJnlLine) string {
	return fmt.Sprintf("%d|%s|%s|%s|%s", line.Type, line.Country_code, line.Tariff_no, line.Transaction_type, line.Transport_method)
}

// checkIntrastatLine: a line the declaration takes has its tariff number, country, transaction
// type, transport method, weight — and its quantity when the tariff number has supplementary
// units
func checkIntrastatLine(line *gtables.IntrastatJnlLine) error {
	for _, field := range []string{"tariff_no", "country_code", "transaction_type", "transport_method", "total_weight"} {
		err := line.TestField(field)
		if err != nil {
			return err
		}
	}
	if line.Supplementary_units {
		return line.TestField("quantity")
	}
	return nil
}

// Report 502 Intrastat - Checklist
type intrastatChecklist struct {
	groupKey                               string
	groupWeight, groupQuantity, groupValue types.Decimal
	totalWeight, totalQuantity, totalValue types.Decimal
	noOfRecords                            int
	groupLine                              *gtables.IntrastatJnlLine
}

func (r *intrastatChecklist) Definition() *Definition {
	return &Definition{
		ContextTable: gtables.IntrastatJnlLineTableName,
		OnInitReport: func(run *Run) error {
			intraJnlBatch, ok := intrastatBatchOf(run)
			if ok {
				run.SetRequestFilter("IntrastatJnlBatch", "journal_template_name", intraJnlBatch.Journal_template_name.String())
				run.SetRequestFilter("IntrastatJnlBatch", "name", intraJnlBatch.Name.String())
			}
			return nil
		},
		OnPreReport: func(run *Run) error {
			setReportHeadValues(run)
			companyInfo := gtables.GetCompanyInformation(run)
			run.SetValue("vat_registration_no", companyInfo.Vat_registration_no.String())
			return nil
		},
		DataItems: []*DataItem{{
			Name:                "IntrastatJnlBatch",
			Table:               gtables.IntrastatJnlBatchTableName,
			Key:                 []string{"journal_template_name", "name"},
			RequestFilterFields: []string{"journal_template_name", "name"},
			OnAfterGetRecord: Trigger(func(rec *gtables.IntrastatJnlBatch, run *Run) error {
				run.SetPageNo(1)
				run.SetValue("statistics_period", run.Text("STATISTICS_PERIOD", rec.Statistics_period.String()))
				*r = intrastatChecklist{}
				return nil
			}),
			DataItems: []*DataItem{{
				Name:                "IntrastatJnlLine",
				Table:               gtables.IntrastatJnlLineTableName,
				Key:                 []string{"type", "country_code", "tariff_no", "transaction_type", "transport_method"},
				Link:                map[string]string{"journal_template_name": "journal_template_name", "journal_batch_name": "name"},
				RequestFilterFields: []string{"type"},
				OnAfterGetRecord:    Trigger(r.lineOnAfterGetRecord),
				OnPostDataItem:      Trigger(r.lineOnPostDataItem),
			}},
		}},
	}
}

// lineOnAfterGetRecord: the line, after the totals of the group before it when the group changes
func (r *intrastatChecklist) lineOnAfterGetRecord(rec *gtables.IntrastatJnlLine, run *Run) error {
	err := checkIntrastatLine(rec)
	if err != nil {
		return err
	}
	key := intrastatGroupKey(rec)
	run.SetValue("group_break", r.groupLine != nil && key != r.groupKey)
	if r.groupLine != nil && key != r.groupKey {
		r.setGroupValues(run, "prev_")
		r.groupWeight, r.groupQuantity, r.groupValue = types.Decimal{}, types.Decimal{}, types.Decimal{}
	}
	if r.groupLine == nil || key != r.groupKey {
		r.noOfRecords++
		run.SetValue("group_heading", true)
	} else {
		run.SetValue("group_heading", false)
	}
	r.groupKey = key
	line := gtables.NewIntrastatJnlLine(run)
	line.Copy(rec)
	r.groupLine = line
	r.setGroupHeading(run, rec)
	r.groupWeight = r.groupWeight.Add(rec.Total_weight)
	r.groupQuantity = r.groupQuantity.Add(rec.Quantity)
	r.groupValue = r.groupValue.Add(rec.Statistical_value)
	r.totalWeight = r.totalWeight.Add(rec.Total_weight)
	r.totalQuantity = r.totalQuantity.Add(rec.Quantity)
	r.totalValue = r.totalValue.Add(rec.Statistical_value)
	r.setTotals(run)
	return nil
}

// lineOnPostDataItem: the last group's totals (printed by the footer)
func (r *intrastatChecklist) lineOnPostDataItem(rec *gtables.IntrastatJnlLine, run *Run) error {
	if r.groupLine != nil {
		r.setGroupValues(run, "last_")
	}
	return nil
}

func (r *intrastatChecklist) setGroupHeading(run *Run, rec *gtables.IntrastatJnlLine) {
	country := gtables.NewCountryRegion(run)
	country.Get(rec.Country_code)
	run.SetValue("type_text", run.Text(strings.ToUpper(rec.Type.String())))
	run.SetValue("country_intrastat_code", country.Intrastat_code.String())
	run.SetValue("country_name", country.Name.String())
}

// setGroupValues: the group's fields and totals with a prefix (prev_: the group before the line,
// last_: the last group)
func (r *intrastatChecklist) setGroupValues(run *Run, prefix string) {
	line := r.groupLine
	country := gtables.NewCountryRegion(run)
	country.Get(line.Country_code)
	run.SetValue(prefix+"type", run.Text(strings.ToUpper(line.Type.String())))
	run.SetValue(prefix+"tariff_no", line.Tariff_no.String())
	run.SetValue(prefix+"country_code", country.Intrastat_code.String())
	run.SetValue(prefix+"country_name", country.Name.String())
	run.SetValue(prefix+"transaction_type", line.Transaction_type.String())
	run.SetValue(prefix+"transport_method", line.Transport_method.String())
	run.SetValue(prefix+"weight", system.Round(r.groupWeight, types.NewDecimalFromInt(1), ">"))
	run.SetValue(prefix+"quantity", r.groupQuantity)
	run.SetValue(prefix+"value", r.groupValue)
}

func (r *intrastatChecklist) setTotals(run *Run) {
	run.SetValue("total_weight", r.totalWeight)
	run.SetValue("total_quantity", r.totalQuantity)
	run.SetValue("total_value", r.totalValue)
	run.SetValue("no_of_entries", types.NewDecimalFromInt(int64(r.noOfRecords)))
}

// Report 593 Intrastat - Make Disk Tax Auth
type intrastatMakeDisk struct{}

func (r *intrastatMakeDisk) Definition() *Definition {
	return &Definition{
		Transaction:  true,
		ContextTable: gtables.IntrastatJnlLineTableName,
		OnPreReport:  r.onPreReport,
	}
}

// intrastatGroup: the lines of one declaration record
type intrastatGroup struct {
	line                    *gtables.IntrastatJnlLine
	refNo                   string
	weight, quantity, value types.Decimal
}

// onPreReport: the batch's lines get their internal reference numbers (per type and group), the
// file has a record per group, the batch is Reported
func (r *intrastatMakeDisk) onPreReport(run *Run) error {
	intraJnlBatch, ok := intrastatBatchOf(run)
	if !ok {
		return fmt.Errorf("report %d runs from an Intrastat journal", IntrastatMakeDiskID)
	}
	err := intraJnlBatch.TestField("reported", false)
	if err != nil {
		return err
	}
	err = intraJnlBatch.TestField("statistics_period")
	if err != nil {
		return err
	}
	period := intraJnlBatch.Statistics_period.String()
	companyInfo := gtables.GetCompanyInformation(run)
	vatRegNo := system.ConvertStr(companyInfo.Vat_registration_no.String(), "WwWw", "    ")

	intraJnlLine := gtables.NewIntrastatJnlLine(run)
	intraJnlLine.SetCurrentKey("type", "country_code", "tariff_no", "transaction_type", "transport_method")
	intraJnlLine.SetRange("journal_template_name", intraJnlBatch.Journal_template_name)
	intraJnlLine.SetRange("journal_batch_name", intraJnlBatch.Name)
	var groups []*intrastatGroup
	var group *intrastatGroup
	intraReferenceNo := period + "000000"
	tempType := -1
	for found := intraJnlLine.FindSetBuffered(); found; found = intraJnlLine.Next() {
		if intraJnlLine.Tariff_no == "" && intraJnlLine.Country_code == "" && intraJnlLine.Transaction_type == "" &&
			intraJnlLine.Transport_method == "" && intraJnlLine.Total_weight.IsZero() {
			continue
		}
		err = checkIntrastatLine(intraJnlLine)
		if err != nil {
			return err
		}
		key := intrastatGroupKey(intraJnlLine)
		switch {
		case int(intraJnlLine.Type) != tempType || group == nil:
			tempType = int(intraJnlLine.Type)
			intraReferenceNo = intraReferenceNo[:4] + strconv.Itoa(int(intraJnlLine.Type)+1) + "01001"
		case intrastatGroupKey(group.line) != key:
			if intraReferenceNo[7:10] == "999" {
				intraReferenceNo = system.IncStr(intraReferenceNo[:7]) + "001"
			} else {
				intraReferenceNo = system.IncStr(intraReferenceNo)
			}
		}
		if group == nil || group.refNo != intraReferenceNo {
			line := gtables.NewIntrastatJnlLine(run)
			line.Copy(intraJnlLine)
			group = &intrastatGroup{line: line, refNo: intraReferenceNo}
			groups = append(groups, group)
		}
		group.weight = group.weight.Add(intraJnlLine.Total_weight)
		group.quantity = group.quantity.Add(intraJnlLine.Quantity)
		group.value = group.value.Add(intraJnlLine.Statistical_value)
		line := gtables.NewIntrastatJnlLine(run)
		line.Copy(intraJnlLine)
		line.Internal_ref_no = types.Text(intraReferenceNo)
		err = line.ModifyOrError(false)
		if err != nil {
			return err
		}
	}

	var file bytes.Buffer
	writeLine := func(text string) { file.WriteString(fixedWidth(text, 80) + "\r\n") }
	writeLine("00" + fixedWidth(vatRegNo, 8) + "INTRASTAT")
	writeLine("0100004")
	receipt, shipment := false, false
	totalValue := types.Decimal{}
	for i, g := range groups {
		// the first group of a type and its kind of declaration: a header with the number of groups
		if g.refNo[7:10] == "001" {
			noOfEntries := 0
			for _, other := range groups[i:] {
				if other.refNo[:7] == g.refNo[:7] {
					noOfEntries++
				}
			}
			importExport := strconv.Itoa(int(g.line.Type) + 1)
			writeLine("02" + zeroText(strconv.Itoa(noOfEntries), 3) + fixedWidth(g.refNo[:7]+"000", 10) +
				fixedWidth(vatRegNo, 8) + importExport + fixedWidth(period, 4))
		}
		if g.line.Type == gtables.IntrastatJnlLine_Type.Receipt {
			receipt = true
		} else {
			shipment = true
		}
		country := gtables.NewCountryRegion(run)
		err = country.GetOrError(g.line.Country_code)
		if err == nil {
			err = country.TestField("intrastat_code")
		}
		if err != nil {
			return err
		}
		tariffNo := system.DelChr(g.line.Tariff_no.String(), "=", " ")
		writeLine("03" + zeroText(g.refNo[7:10], 3) + fixedWidth(g.refNo, 10) + fixedWidth(country.Intrastat_code.String(), 3) +
			fixedWidth(g.line.Transaction_type.String(), 2) + "0" + fixedWidth(g.line.Transport_method.String(), 1) +
			system.PadStr(tariffNo, 9, "0") + zeroNumber(system.Round(g.weight, types.NewDecimalFromInt(1), ">"), 15) +
			zeroNumber(g.quantity, 10) + zeroNumber(g.value, 15))
		totalValue = totalValue.Add(g.value)
	}
	if !receipt {
		writeLine("02000" + fixedWidth(period, 4) + "100000" + fixedWidth(vatRegNo, 8) + "1" + fixedWidth(period, 4))
	}
	if !shipment {
		writeLine("02000" + fixedWidth(period, 4) + "200000" + fixedWidth(vatRegNo, 8) + "2" + fixedWidth(period, 4))
	}
	writeLine("10" + zeroNumber(totalValue, 16))
	intraJnlBatch.Reported = true
	err = intraJnlBatch.ModifyOrError(false)
	if err != nil {
		return err
	}
	run.WriteFile(run.Text("FILE_NAME", period), file.Bytes())
	run.Message(run.Text("DONE", len(groups)))
	return nil
}

// fixedWidth: a text cut or padded with spaces to a width (classic FORMAT(Text, Length))
func fixedWidth(text string, width int) string {
	runes := []rune(text)
	if len(runes) >= width {
		return string(runes[:width])
	}
	return text + strings.Repeat(" ", width-len(runes))
}

// zeroText (TextZeroFormat): a text right-aligned with leading zeros to a length
func zeroText(text string, length int) string {
	if len(text) >= length {
		return text[len(text)-length:]
	}
	return strings.Repeat("0", length-len(text)) + text
}

// zeroNumber (DecimalNumeralZeroFormat): the whole part of a decimal's absolute value with
// leading zeros
func zeroNumber(d types.Decimal, length int) string {
	whole := system.Round(d.Abs(), types.NewDecimalFromInt(1), "<")
	return zeroText(whole.StringFixed(0), length)
}
