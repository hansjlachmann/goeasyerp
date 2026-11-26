package apps

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/appaudit"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Audit Report (S55): an uploaded package no build took is audited now; once a build audited
// the app's exact source, its report is shown; a Git app no build took has none yet
func TestShowAuditReport(t *testing.T) {
	src, base := buildTestDB(t)
	setBuildSetup(t, src, true, true)
	const id = "aaaaaaaa-0000-4000-8000-000000000041"
	pkg := zipOf(t, map[string]string{
		"application.yaml": string(manifest(id, "Reported", "1.0.0", "rp", "{from: 54000, to: 54099}", "{id: "+base.ID+"}")),
		"go.mod":           "module example.com/reported\n\nrequire github.com/hansjlachmann/goeasyerp v0.0.0\n",
		"reported.go":      "package reported\n\nimport \"strings\"\n\nvar _ = strings.ToUpper\n",
	})
	if _, err := uploadPackage(src, pkg); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(pkg)
	sha := hex.EncodeToString(sum[:])
	app := gtables.NewInstalledApp(src)
	if !app.Get(types.NewText(id)) {
		t.Fatal("the uploaded app is not in Installed App")
	}
	text := auditReportText(t, app)
	if !strings.Contains(text, "audited now") || !strings.Contains(text, sha) || !strings.Contains(text, "passed") {
		t.Errorf("before a build:\n%s", text)
	}

	// a build's report of another source is not this one's; the one of this package is shown
	for i, source := range []string{"0000", sha} {
		build := gtables.NewAppBuild(src)
		build.Entry_no = i + 1
		build.Status = gtables.AppBuild_Status.Succeeded
		records := []appaudit.Record{{AppID: id, Name: "Reported", Version: "1.0.0", SourceID: source, Reports: []*appaudit.Report{
			{App: "Reported 1.0.0", Notes: []string{"from build " + source}}}}}
		build.Audit_reports, _ = json.Marshal(records)
		if !build.Insert(false) {
			t.Fatal(build.TriggerError())
		}
	}
	text = auditReportText(t, app)
	if !strings.Contains(text, "audited by build 2") || !strings.Contains(text, "from build "+sha) {
		t.Errorf("after the build:\n%s", text)
	}

	// a Git app no build took
	app.Source_type = gtables.InstalledApp_Source_type.Git
	app.Source_commit = "c0ffee"
	if _, err := ShowAuditReport(app); err == nil || !strings.Contains(err.Error(), "not been audited yet") {
		t.Errorf("a Git app without a build: %v", err)
	}
}

func auditReportText(t *testing.T, app *gtables.InstalledApp) string {
	t.Helper()
	result, err := ShowAuditReport(app)
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(result.Data["file"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if name := result.Data["filename"].(string); name != "app-audit-Reported-1.0.0.txt" {
		t.Errorf("file name %q", name)
	}
	return string(data)
}
