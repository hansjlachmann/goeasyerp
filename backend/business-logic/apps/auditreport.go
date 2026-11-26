package apps

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/appaudit"
	fapps "github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The app audit's report of an installed app (S55): Installed Apps → Build → Audit Report.

func init() {
	functions.Register("table", gtables.InstalledAppTableName, "ShowAuditReport", functions.For(ShowAuditReport))
}

// buildsSearched: how many of the latest builds Audit Report looks through
const buildsSearched = 100

// ShowAuditReport gives an app's latest audit report as a text file: the report of the latest
// build that took the app's exact source (the audit of its source and, when it brings in
// modules of its own, of their known vulnerabilities). An uploaded package no build has taken
// yet is audited now — its source only: the vulnerability check needs the build.
func ShowAuditReport(rec *gtables.InstalledApp) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	err := superOnly(lang)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if isBaseApp(rec.App_id.String()) {
		return fcodeunits.Result{}, errors.New(ts.MessageWithParams("ERR_APP_AUDIT_BASE", lang, rec.Name.String()))
	}
	appID := rec.App_id.String()
	version := rec.Version.String()
	sourceID := rec.Source_commit.String()
	var pkg *gtables.AppPackage
	if rec.Source_type == gtables.InstalledApp_Source_type.Package {
		pkg = gtables.NewAppPackage(rec)
		found := pkg.Get(map[string]interface{}{"app_id": appID, "version": version})
		if found {
			sourceID = pkg.Sha256.String()
		} else {
			pkg = nil
		}
	}
	text, found := latestBuildAudit(rec, appID, version, sourceID)
	if !found && pkg != nil {
		text, err = auditStoredPackage(rec, pkg)
		if err != nil {
			return fcodeunits.Result{}, err
		}
		found = true
	}
	if !found {
		return fcodeunits.Result{}, errors.New(ts.MessageWithParams("ERR_APP_AUDIT_NO_REPORT", lang, rec.Name.String(), version))
	}
	return fcodeunits.Result{Success: true, Data: map[string]interface{}{
		"file":     base64.StdEncoding.EncodeToString([]byte(text)),
		"filename": fmt.Sprintf("app-audit-%s-%s.txt", fileNamePart(rec.Name.String()), fileNamePart(version)),
	}}, nil
}

// latestBuildAudit: the report of the latest build that audited the app's version from the
// source (any source when the app has no recorded one: installed before commits were recorded)
func latestBuildAudit(src *gtables.InstalledApp, appID, version, sourceID string) (string, bool) {
	build := gtables.NewAppBuild(src)
	searched := 0
	for found := build.Find("+"); found && searched < buildsSearched; found = build.Next(-1) {
		searched++
		if len(build.Audit_reports) == 0 {
			continue
		}
		var records []appaudit.Record
		err := json.Unmarshal(build.Audit_reports, &records)
		if err != nil {
			continue
		}
		for _, r := range records {
			if r.AppID == appID && r.Version == version && (sourceID == "" || r.SourceID == sourceID) {
				head := fmt.Sprintf("%s %s, source %s — audited by build %d (%s, %s)\n\n", r.Name, r.Version, r.SourceID,
					build.Entry_no, build.Status.String(), build.Finished_at.String())
				return head + r.Text(), true
			}
		}
	}
	return "", false
}

// auditStoredPackage audits an uploaded package as Upload App does, now (with the exceptions
// approved for it since)
func auditStoredPackage(src *gtables.InstalledApp, pkg *gtables.AppPackage) (string, error) {
	read, err := fapps.ReadPackage(pkg.Package)
	if err != nil {
		return "", err
	}
	hash := pkg.Sha256.String()
	opts := appaudit.Options{AllowedModules: allowedGoModules(src)}
	for _, e := range auditExceptions(src, read.App.ID, read.App.Version, hash) {
		opts.Exceptions = append(opts.Exceptions, appaudit.Exception{Rule: e.Rule, Key: e.Key})
	}
	report := appaudit.Audit(read.App.Name+" "+read.App.Version, read.Files, opts)
	head := fmt.Sprintf("%s %s, package %s — audited now (no build has taken it yet; known vulnerabilities are checked when it is built)\n\n",
		read.App.Name, read.App.Version, hash)
	return head + report.Text(), nil
}

// isBaseApp: the app is the base application (the first of the installed apps)
func isBaseApp(appID string) bool {
	installedMu.RLock()
	defer installedMu.RUnlock()
	return len(installed) > 0 && installed[0].ID == appID
}

var fileNameUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// fileNamePart: a name as a part of a file name
func fileNamePart(s string) string {
	return fileNameUnsafe.ReplaceAllString(s, "-")
}
