package demodata

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/resjnl"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/sift"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// insertResCapacity gives each demo resource its capacity per working day (Monday to Friday)
// from a year ago to three months ahead: people 7.5 hours, machines 8 hours, a day-rate
// resource 1 day
func insertResCapacity(exec database.Executor, company string, dbType database.DBType, masterData []tableRecords, today time.Time, counts *Counts) error {
	var entries []*gtables.ResCapacityEntry
	entryNo := 0
	start := today.AddDate(-1, 0, 1)
	end := today.AddDate(0, 3, 0)
	for _, t := range masterData {
		if t.Table != gtables.ResourceTableName {
			continue
		}
		for _, r := range t.Records {
			capacity := types.MustDecimal("7.5")
			if r["type"] == 1 {
				capacity = types.NewDecimalFromInt(8)
			}
			if r["unit_of_measure_code"] == "DAY" {
				capacity = types.NewDecimalFromInt(1)
			}
			for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
				if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
					continue
				}
				entryNo++
				e := &gtables.ResCapacityEntry{}
				e.InitWithDBType(exec, company, dbType)
				e.Entry_no = entryNo
				e.Resource_no = types.NewCode(r["no"].(string))
				e.Resource_group_no = types.NewCode(r["resource_group_no"].(string))
				e.Date = types.NewDateFromTime(date)
				e.Capacity = capacity
				entries = append(entries, e)
			}
		}
	}
	if len(entries) == 0 {
		return nil
	}
	// The SIFT totals are built once after the load instead of updated per entry
	err := sift.Suspend(exec, dbType, company, gtables.ResCapacityEntryTableName)
	if err != nil {
		return err
	}
	err = entries[0].InsertAll(entries, false)
	if err != nil {
		return err
	}
	counts.ResCapacityEntries += len(entries)
	return (&gtables.ResCapacityEntry{}).SyncKeys(exec, company, dbType)
}

// postDemoResources posts a year of resource use through the resource posting (codeunit 212):
// every week each resource's usage — normal hours, sometimes overtime or travel — and every
// month the chargeable hours as a sale; one register per month
func postDemoResources(src ftables.Source, today time.Time, masterData []tableRecords, counts *Counts, p *progress) error {
	rng := rand.New(rand.NewSource(seed + 156))
	var resources []map[string]interface{}
	for _, t := range masterData {
		if t.Table == gtables.ResourceTableName {
			resources = append(resources, t.Records...)
		}
	}
	start := today.AddDate(-1, 0, 1)
	// the Monday of the first week
	for start.Weekday() != time.Monday {
		start = start.AddDate(0, 0, 1)
	}
	// about a usage per resource and week and a sale per resource and month
	p.setTotal((int(today.Sub(start).Hours()/24/7) + 13) * len(resources))
	lineNo := 0
	docNo := 0
	var resJnlPostLine *resjnl.ResJnlPostLine
	post := func(date time.Time, entryType gtables.ResJournalLineEntry_type, resNo, workType string, qty types.Decimal, chargeable bool) error {
		lineNo = lineNo + 10000
		line := gtables.NewResJournalLine(src)
		line.InitRecord()
		line.Journal_template_name = "RESOURCES"
		line.Journal_batch_name = "DEFAULT"
		line.Line_no = lineNo
		line.Source_code = "RESJNL"
		line.Document_no = types.Code(fmt.Sprintf("RES%05d", docNo))
		line.Chargeable = chargeable
		for _, step := range []struct {
			field string
			value interface{}
		}{
			{"posting_date", types.NewDateFromTime(date)},
			{"entry_type", entryType},
			{"resource_no", types.Code(resNo)},
			{"work_type_code", types.Code(workType)},
			{"quantity", qty},
		} {
			err := line.Validate(step.field, step.value)
			if err != nil {
				return fmt.Errorf("demo resource journal %s %s: %w", resNo, step.field, err)
			}
		}
		err := resJnlPostLine.Run(line)
		if err != nil {
			return fmt.Errorf("demo resource journal %s %s: %w", resNo, line.Document_no, err)
		}
		counts.ResLedgerEntries++
		p.step()
		return nil
	}

	month := start.Month()
	chargeableHours := map[string]int64{}
	resJnlPostLine = resjnl.NewResJnlPostLine(src)
	docNo++
	invoiceMonth := func(date time.Time) error {
		docNo++
		for _, r := range resources {
			no := r["no"].(string)
			if chargeableHours[no] > 0 {
				err := post(date, gtables.ResJournalLine_Entry_type.Sale, no, "", types.NewDecimalFromInt(chargeableHours[no]), true)
				if err != nil {
					return err
				}
			}
		}
		chargeableHours = map[string]int64{}
		return nil
	}
	for week := start; week.Before(today); week = week.AddDate(0, 0, 7) {
		if week.Month() != month {
			// the month's sales on its last Friday, then a new register for the next month
			err := invoiceMonth(week.AddDate(0, 0, -3))
			if err != nil {
				return err
			}
			month = week.Month()
			resJnlPostLine = resjnl.NewResJnlPostLine(src)
		}
		docNo++
		friday := week.AddDate(0, 0, 4)
		for _, r := range resources {
			no := r["no"].(string)
			hours := 30 + rng.Intn(8) // 30-37 hours of the week
			workType := "NORMAL"
			if r["unit_of_measure_code"] == "DAY" {
				hours = 1 + rng.Intn(4) // days of use
				workType = "DAY"
			}
			chargeable := rng.Intn(10) < 8
			err := post(friday, gtables.ResJournalLine_Entry_type.Usage, no, workType, types.NewDecimalFromInt(int64(hours)), chargeable)
			if err != nil {
				return err
			}
			if chargeable {
				chargeableHours[no] += int64(hours)
			}
			if r["type"] == 0 && rng.Intn(5) == 0 {
				extra := "OVERTIME"
				if rng.Intn(2) == 0 {
					extra = "TRAVEL"
				}
				err = post(friday, gtables.ResJournalLine_Entry_type.Usage, no, extra, types.NewDecimalFromInt(int64(2+rng.Intn(4))), true)
				if err != nil {
					return err
				}
			}
		}
	}
	return invoiceMonth(today.AddDate(0, 0, -1))
}
