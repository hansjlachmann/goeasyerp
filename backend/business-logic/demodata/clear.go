package demodata

import (
	"fmt"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/company"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// keptTables are the company's tables a new demo run leaves as they are: the job queue (the
// job running codeunit 901 and its log) and the users' setup. Demo data writes none of them.
var keptTables = []string{
	gtables.JobQueueTableName,
	gtables.JobQueueEntryTableName,
	gtables.UserSetupTableName,
}

// clearCompany deletes the records of every table of the company but the kept ones, in the
// demo run's transaction, so the demo data can be created again. The first pass empties
// the tables — deleting entries runs their SIFT triggers, which may write totals rows into
// a totals table already emptied —, the second pass empties those totals tables again.
func clearCompany(exec database.Executor, companyName string, dbType database.DBType) error {
	tables, err := company.Tables(exec, dbType, companyName)
	if err != nil {
		return err
	}
	kept := make(map[string]bool, len(keptTables))
	for _, t := range keptTables {
		kept[companyName+"$"+t] = true
	}
	for pass := 0; pass < 2; pass++ {
		for _, t := range tables {
			if kept[t] {
				continue
			}
			_, err = exec.Exec(`DELETE FROM "` + strings.ReplaceAll(t, `"`, `""`) + `"`)
			if err != nil {
				return fmt.Errorf("clear %s: %w", t, err)
			}
		}
	}
	return nil
}
