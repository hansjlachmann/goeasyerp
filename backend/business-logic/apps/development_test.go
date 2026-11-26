package apps

import (
	"errors"
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// finishBuilds marks the requested builds Succeeded and their apps Installed, as the builder and
// the server's switch would
func finishBuilds(t *testing.T, src *gtables.InstalledApp) {
	t.Helper()
	var builds []int
	build := gtables.NewAppBuild(src)
	for found := build.FindSet(); found; found = build.Next() {
		builds = append(builds, build.Entry_no)
	}
	for _, no := range builds {
		b := gtables.NewAppBuild(src)
		if !b.Get(no) {
			t.Fatal("build", no)
		}
		b.Status = gtables.AppBuild_Status.Succeeded
		if err := b.ModifyOrError(false); err != nil {
			t.Fatal(err)
		}
	}
	var pending []string
	app := gtables.NewInstalledApp(src)
	for found := app.FindSet(); found; found = app.Next() {
		if app.Status == gtables.InstalledApp_Status.PendingInstall {
			pending = append(pending, app.App_id.String())
		}
	}
	for _, id := range pending {
		a := gtables.NewInstalledApp(src)
		if !a.Get(types.NewText(id)) {
			t.Fatal("app", id)
		}
		a.Status = gtables.InstalledApp_Status.Installed
		if err := a.ModifyOrError(false); err != nil {
			t.Fatal(err)
		}
	}
}

// F5 in VS Code: a package is published and built; the same version again with other code is
// published again (Upload App still refuses it), not while a build is pending; the audit's
// refusal carries the findings; unpublishing builds without the app
func TestPublishDevelopment(t *testing.T) {
	src, base := buildTestDB(t)
	setBuildSetup(t, src, true, false) // a development publish needs no Allow Upload
	const id = "aaaaaaaa-0000-4000-8000-000000000051"
	pkgOf := func(code string) []byte {
		return zipOf(t, map[string]string{
			"application.yaml": string(manifest(id, "Dev App", "1.0.0", "dv", "{from: 54000, to: 54099}", "{id: "+base.ID+"}")),
			"go.mod":           "module example.com/devapp\n\nrequire github.com/hansjlachmann/goeasyerp v0.0.0\n",
			"dev.go":           code,
		})
	}
	first, err := PublishDevelopment(src, pkgOf("package devapp\n\nimport \"strings\"\n\nvar _ = strings.ToUpper\n"))
	if err != nil {
		t.Fatal(err)
	}
	if first.Build != 1 || first.AppID != id || first.Version != "1.0.0" || len(first.SHA256) != 64 {
		t.Errorf("first publish %+v", first)
	}
	build := gtables.NewAppBuild(src)
	if !build.Get(1) || build.Trigger != gtables.AppBuild_Trigger.Development || !strings.Contains(build.Apps.String(), "Dev App") {
		t.Errorf("build %+v", build)
	}
	second := pkgOf("package devapp\n\nimport \"strings\"\n\nvar _ = strings.ToLower\n")
	if _, err := PublishDevelopment(src, second); err == nil || !strings.Contains(err.Error(), "1") {
		t.Errorf("published while build 1 is requested: %v", err)
	}

	finishBuilds(t, src)
	again, err := PublishDevelopment(src, second)
	if err != nil {
		t.Fatalf("the same version again: %v", err)
	}
	if again.Build != 2 || again.SHA256 == first.SHA256 {
		t.Errorf("second publish %+v", again)
	}
	stored := gtables.NewAppPackage(src)
	if !stored.Get(map[string]interface{}{"app_id": id, "version": "1.0.0"}) || stored.Sha256.String() != again.SHA256 {
		t.Errorf("the package was not replaced: %s", stored.Sha256.String())
	}
	finishBuilds(t, src)
	if _, err := uploadPackage(src, pkgOf("package devapp\n")); err == nil || !strings.Contains(err.Error(), "installed already") {
		t.Errorf("Upload App took the same version again: %v", err)
	}

	_, err = PublishDevelopment(src, pkgOf("package devapp\n\nimport \"os/exec\"\n\nvar _ = exec.Command\n"))
	var audit *AuditError
	if !errors.As(err, &audit) || audit.Report.Passed() || audit.Report.Findings[0].File != "dev.go" || audit.Report.Findings[0].Line != 3 {
		t.Fatalf("a package the audit refuses: %v", err)
	}
	if n := gtables.NewAppBuild(src).Count(); n != 2 {
		t.Errorf("a refused publish requested a build: %d builds", n)
	}

	out, err := UnpublishDevelopment(src, id)
	if err != nil {
		t.Fatal(err)
	}
	app := gtables.NewInstalledApp(src)
	if out.Build != 3 || !app.Get(types.NewText(id)) || app.Status != gtables.InstalledApp_Status.PendingRemoval {
		t.Errorf("unpublish %+v, status %v", out, app.Status)
	}
	if _, err := UnpublishDevelopment(src, "no-such-app"); err == nil {
		t.Error("unpublished an app that is not installed")
	}
}

// The development endpoint never runs on a database that looks like production
func TestDevelopmentAllowed(t *testing.T) {
	src, _ := buildTestDB(t)
	db := src.GetDB()
	for _, rec := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.User{}, &gtables.Company{}} {
		if err := rec.CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"demo", "real"} {
		if err := (&gtables.Customer{}).CreateTableWithDBType(db, name, database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
		company := gtables.NewCompany(ftables.DBSource(db, "", database.DBTypeSQLite))
		company.Name = types.NewText(name)
		company.Demo_company = name == "demo"
		if !company.Insert(false) {
			t.Fatal(company.TriggerError())
		}
	}
	addCustomer := func(company string) {
		customer := gtables.NewCustomer(ftables.DBSource(db, company, database.DBTypeSQLite))
		customer.No = "C1"
		if !customer.Insert(false) {
			t.Fatal(customer.TriggerError())
		}
	}
	addCustomer("demo")
	if ok, reason := DevelopmentAllowed(db, database.DBTypeSQLite); !ok {
		t.Errorf("a demo company with customers and an empty company: %s", reason)
	}
	addCustomer("real")
	if ok, reason := DevelopmentAllowed(db, database.DBTypeSQLite); ok || !strings.Contains(reason, "real") {
		t.Errorf("a company with customers that is not a demo company: %v %q", ok, reason)
	}
}
