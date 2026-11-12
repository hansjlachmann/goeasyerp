package reports

import (
	"encoding/json"
	"time"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Saved request page values (classic SaveValues): the options and data item filters the user
// last ran a report with, per user, report and company, in User Preferences (global) — the
// report number in Page ID, type RequestPreferenceType, the company in Preference Name. A
// request page starts with them: the options before OnInitReport and the caller's values (so
// those decide where they set one), the filters of the data items nobody else gave a starting
// filter. File options and the output are not saved; a run without request page (scheduled,
// from code) neither reads nor writes them.

// RequestPreferenceType is the User Preferences type of saved request page values
const RequestPreferenceType = "REPORT"

// savedValues is what a request page keeps
type savedValues struct {
	Options map[string]string                `json:"options,omitempty"`
	Filters map[string][]ftables.FieldFilter `json:"filters,omitempty"`
}

// maxSavedValues: the most one report's saved values may hold (JSON bytes)
const maxSavedValues = 64 * 1024

// savedValuesRecord reads the user's saved values of the run's report into pref; false when
// there are none (or no user)
func savedValuesRecord(run *Run, pref *gtables.UserPreferences) bool {
	user := fcodeunits.CurrentUserID()
	if user == "" {
		return false
	}
	return pref.GetByPK(types.NewCode(user), run.ID, types.NewCode(RequestPreferenceType), types.NewCode(run.Company))
}

// loadSavedValues returns the values the user last ran the report with (empty when none)
func loadSavedValues(run *Run) savedValues {
	saved := savedValues{}
	pref := gtables.NewUserPreferences(ftables.DBSource(run.DB, "", run.DBType))
	found := savedValuesRecord(run, pref)
	if !found {
		return saved
	}
	err := json.Unmarshal([]byte(pref.Preference_data.String()), &saved)
	if err != nil {
		return savedValues{}
	}
	return saved
}

// applySavedOptions puts the saved options into the run: only options the report has, never a
// file
func applySavedOptions(run *Run, def *Definition, saved savedValues) {
	for _, o := range def.Options {
		if o.Type == "file" {
			continue
		}
		value, ok := saved.Options[o.Name]
		if ok {
			run.options[o.Name] = value
		}
	}
}

// applySavedFilters gives the data items without starting filters their saved filters
func applySavedFilters(run *Run, def *Definition, saved savedValues) {
	for _, item := range allDataItems(def.DataItems) {
		if len(run.filters[item.Name]) > 0 {
			continue
		}
		filters, ok := saved.Filters[item.Name]
		if ok {
			run.filters[item.Name] = append([]ftables.FieldFilter(nil), filters...)
		}
	}
}

// saveValues keeps the options and filters the user ran the report with. Options the caller
// set and the filters of data items that started filtered by the caller (a record, the job's
// parameter, OnInitReport) keep what was saved before: they belong to that run, not to the user.
// A failure to save does not stop the report.
func saveValues(run *Run, def *Definition, callerOptions map[string]string, callerFiltered map[string]bool, previous savedValues) {
	user := fcodeunits.CurrentUserID()
	if user == "" {
		return
	}
	values := savedValues{Options: map[string]string{}, Filters: map[string][]ftables.FieldFilter{}}
	for _, o := range def.Options {
		if o.Type == "file" {
			continue
		}
		_, byCaller := callerOptions[o.Name]
		if byCaller {
			value, ok := previous.Options[o.Name]
			if ok {
				values.Options[o.Name] = value
			}
		} else {
			values.Options[o.Name] = run.options[o.Name]
		}
	}
	for _, item := range allDataItems(def.DataItems) {
		filters := previous.Filters[item.Name]
		if !callerFiltered[item.Name] {
			filters = nil
			for _, f := range run.filters[item.Name] {
				if f.Expression != "" {
					filters = append(filters, f)
				}
			}
		}
		if len(filters) > 0 {
			values.Filters[item.Name] = filters
		}
	}
	data, err := json.Marshal(values)
	if err != nil || len(data) > maxSavedValues {
		return
	}
	now := types.NewDateTimeFromTime(time.Now())
	pref := gtables.NewUserPreferences(ftables.DBSource(run.DB, "", run.DBType))
	found := savedValuesRecord(run, pref)
	if found {
		pref.Preference_data = types.NewText(string(data))
		pref.Updated_at = now
		pref.Modify(true)
		return
	}
	pref.InitRecord()
	pref.User_id = types.NewCode(user)
	pref.Page_id = run.ID
	pref.Preference_type = types.NewCode(RequestPreferenceType)
	pref.Preference_name = types.NewCode(run.Company)
	pref.Preference_data = types.NewText(string(data))
	pref.Created_at = now
	pref.Updated_at = now
	pref.Insert(true)
}
