package codeunits

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/format"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// SecurityEventAlerts - Codeunit 909: Security Event Alerts (S48)
//
// Looks at the security events not checked yet (table 2000000158) and reports what an
// administrator should see: lockouts, wrong setup tokens, changes sent from other web sites,
// and many refused requests or failed logins. Run by the Job Queue (e.g. every hour) it marks
// the events checked, deletes events older than KEEP_DAYS, and fails with the findings as the
// error's details — the job goes to Error and its notification e-mail goes out. Run from the
// Security Events page it only shows the findings (nothing is marked, so the next scheduled run
// still reports them).
//
// Parameter (optional, "name=value" separated by ";" or spaces): REFUSALS=20 (refused requests
// of one user), FAILED_LOGINS=50 (failed logins in all), KEEP_DAYS=365 (0: keep every event;
// otherwise at least securityKeepDaysMin, so a job cannot wipe the recent events).
const SecurityEventAlertsID = 909
const SecurityEventAlertsName = "security-event-alerts"

// securityKeepDaysMin: the fewest days KEEP_DAYS keeps the events (0 keeps them all)
const securityKeepDaysMin = 30

// securityAlertMaxRows: the rows shown per section of the findings
const securityAlertMaxRows = 500

func init() {
	Register(SecurityEventAlertsID, SecurityEventAlertsName, NewSecurityEventAlerts)
}

type SecurityEventAlerts struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewSecurityEventAlerts creates a new instance of the codeunit
func NewSecurityEventAlerts(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &SecurityEventAlerts{db: db, company: company, dbType: dbType}
}

func (c *SecurityEventAlerts) ID() int             { return SecurityEventAlertsID }
func (c *SecurityEventAlerts) Name() string        { return SecurityEventAlertsName }
func (c *SecurityEventAlerts) SourceTable() string { return gtables.SecurityEventTableName }
func (c *SecurityEventAlerts) UsesProgress() bool  { return false }

// securityAlertLimits are the thresholds of a run
type securityAlertLimits struct {
	refusals     int // refused requests of one user (Access Refused, Company Refused)
	failedLogins int // failed logins in all (Login Failed, Password Change Failed)
	keepDays     int // events older are deleted by a scheduled run (0: none)
}

// parseSecurityAlertLimits reads the job's parameter (unknown names and bad numbers are refused)
func parseSecurityAlertLimits(parameter string) (securityAlertLimits, error) {
	limits := securityAlertLimits{refusals: 20, failedLogins: 50, keepDays: 365}
	fields := strings.FieldsFunc(parameter, func(r rune) bool { return r == ';' || r == ' ' || r == '\n' || r == ',' })
	for _, field := range fields {
		name, value, found := strings.Cut(field, "=")
		number, err := strconv.Atoi(strings.TrimSpace(value))
		if !found || err != nil || number < 0 {
			return limits, apperrors.SecurityAlertParameter(field)
		}
		switch strings.ToUpper(strings.TrimSpace(name)) {
		case "REFUSALS":
			limits.refusals = number
		case "FAILED_LOGINS":
			limits.failedLogins = number
		case "KEEP_DAYS":
			if number > 0 && number < securityKeepDaysMin {
				return limits, apperrors.SecurityAlertKeepDays(number, securityKeepDaysMin)
			}
			limits.keepDays = number
		default:
			return limits, apperrors.SecurityAlertParameter(field)
		}
	}
	return limits, nil
}

// securityFindings is what a run found among the events
type securityFindings struct {
	checked      int                     // events looked at
	lastEntryNo  int                     // the last of them
	alerts       []gtables.SecurityEvent // events reported one by one
	refusals     map[string]int          // refused requests per user
	failedLogins int
}

// Run checks the events; run by the Job Queue (a Job Queue record) it marks them and logs the run
func (c *SecurityEventAlerts) Run(record interface{}) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	start := types.Now()
	jobQueue, fromJobQueue := record.(*gtables.JobQueue)
	parameter := ""
	if fromJobQueue {
		parameter = jobQueue.Parameter.String()
	}
	limits, err := parseSecurityAlertLimits(parameter)
	if err != nil {
		return fcodeunits.Result{}, err
	}

	src := ftables.DBSource(c.db, "", c.dbType) // a global table
	found := c.findings(src)
	reported := c.report(found, limits, ts, lang)

	if fromJobQueue {
		err = c.markChecked(src, found.lastEntryNo, limits.keepDays)
		if err != nil {
			return fcodeunits.Result{}, err
		}
		status := gtables.JobQueueEntry_Status.Success
		message := ""
		if reported != nil {
			status = gtables.JobQueueEntry_Status.Error
			message = reported.Message(lang)
		}
		err = CreateJobQueueEntry(c.db, c.company, c.dbType, jobQueue, status, message, start)
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}
	if reported != nil {
		return fcodeunits.Result{}, reported
	}
	return fcodeunits.Message(ts.MessageWithParams("SECURITY_CHECK_OK", lang, strconv.Itoa(found.checked))), nil
}

// findings reads the events not checked yet
func (c *SecurityEventAlerts) findings(src ftables.Source) securityFindings {
	found := securityFindings{refusals: map[string]int{}}
	securityEvent := gtables.NewSecurityEvent(src)
	securityEvent.SetRange("checked", false)
	for ok := securityEvent.FindSet(); ok; ok = securityEvent.Next() {
		found.checked++
		found.lastEntryNo = max(found.lastEntryNo, securityEvent.Entry_no)
		switch securityEvent.Event {
		case gtables.SecurityEvent_Event.LoginLocked, gtables.SecurityEvent_Event.SetupTokenRefused,
			gtables.SecurityEvent_Event.CrossSiteRefused:
			found.alerts = append(found.alerts, *securityEvent)
		case gtables.SecurityEvent_Event.AccessRefused, gtables.SecurityEvent_Event.CompanyRefused:
			found.refusals[securityEvent.User_id.String()]++
		case gtables.SecurityEvent_Event.LoginFailed, gtables.SecurityEvent_Event.PasswordChangeFailed:
			found.failedLogins++
		}
	}
	return found
}

// report is the error naming what the run found (nil: nothing to report), with the events as
// its details
func (c *SecurityEventAlerts) report(found securityFindings, limits securityAlertLimits, ts *i18n.TranslationService, lang string) *apperrors.AppError {
	caption := func(field string) string {
		return ts.FieldCaption(gtables.SecurityEventTableName, field, lang)
	}
	var users []string
	for user, n := range found.refusals {
		if limits.refusals > 0 && n >= limits.refusals {
			users = append(users, user)
		}
	}
	sort.Strings(users)
	tooManyFailed := limits.failedLogins > 0 && found.failedLogins >= limits.failedLogins
	if len(found.alerts) == 0 && len(users) == 0 && !tooManyFailed {
		return nil
	}

	details := &apperrors.Details{Title: ts.Message("SECURITY_CHECK_TITLE", lang)}
	if len(found.alerts) > 0 {
		section := apperrors.DetailsSection{
			Caption: ts.Message("SECURITY_CHECK_EVENTS", lang),
			Columns: []apperrors.DetailsColumn{
				{Caption: caption("entry_no"), Numeric: true}, {Caption: caption("occurred_at")}, {Caption: caption("event")},
				{Caption: caption("user_id")}, {Caption: caption("address")}, {Caption: caption("company")}, {Caption: caption("details")},
			},
		}
		for _, e := range found.alerts {
			if len(section.Rows) >= securityAlertMaxRows {
				break
			}
			when := format.Date(e.Occurred_at.Date(), lang) + " " + e.Occurred_at.Format("15:04:05")
			event := ts.OptionCaption(gtables.SecurityEventTableName, "event", i18n.Key(e.Event.String()), lang)
			section.Rows = append(section.Rows, []string{strconv.Itoa(e.Entry_no), when, event, e.User_id.String(),
				e.Address.String(), e.Company.String(), e.Details.String()})
		}
		details.Sections = append(details.Sections, section)
	}
	if len(users) > 0 {
		section := apperrors.DetailsSection{
			Caption: ts.MessageWithParams("SECURITY_CHECK_REFUSALS", lang, strconv.Itoa(limits.refusals)),
			Columns: []apperrors.DetailsColumn{{Caption: caption("user_id")}, {Caption: ts.Message("SECURITY_CHECK_COUNT", lang), Numeric: true}},
		}
		for _, user := range users {
			section.Rows = append(section.Rows, []string{user, strconv.Itoa(found.refusals[user])})
		}
		details.Sections = append(details.Sections, section)
	}
	failed := 0
	if tooManyFailed {
		failed = found.failedLogins
	}
	return apperrors.SecurityEventsFound(strconv.Itoa(len(found.alerts)), strconv.Itoa(len(users)), strconv.Itoa(failed)).WithDetails(details)
}

// markChecked marks the events up to lastEntryNo checked and deletes those older than keepDays
func (c *SecurityEventAlerts) markChecked(src ftables.Source, lastEntryNo, keepDays int) error {
	securityEvent := gtables.NewSecurityEvent(src)
	if lastEntryNo > 0 {
		securityEvent.SetRange("checked", false)
		securityEvent.SetFilter("entry_no", fmt.Sprintf("..%d", lastEntryNo))
		securityEvent.ModifyAll("checked", true)
	}
	if keepDays > 0 {
		securityEvent.Reset()
		before := time.Now().AddDate(0, 0, -keepDays).UTC().Format("2006-01-02T15:04:05")
		securityEvent.SetFilter("occurred_at", ".."+before)
		securityEvent.DeleteAll()
	}
	return nil
}
