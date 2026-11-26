package apps

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/appaudit"
	fapps "github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/themes"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Installing and uninstalling an extension app (Installed Apps, page 907; SUPER only). An app's
// code is built into the program, so installing it takes a build: Install reads the app's
// application.yaml from its GitHub repository at the version given, checks it against the
// installed apps and those already chosen (number series, prefix, dependencies and their
// versions) and records it as Pending Install; Uninstall marks an app Pending Removal (not while
// another app depends on it). Build List gives the list of apps the next build contains — the
// base version and every app with its repository, version and folder, in dependency order —
// for the installation repository, whose workflow builds the image; once the server runs it,
// the startup sync marks the apps Installed (or removes them).

func init() {
	functions.Register("table", gtables.InstalledAppTableName, "InstallApp", functions.For(InstallApp), functions.WithDialog())
	functions.Register("table", gtables.InstalledAppTableName, "UploadApp", functions.For(UploadApp), functions.WithDialog())
	functions.Register("table", gtables.InstalledAppTableName, "UninstallApp", functions.For(UninstallApp))
	functions.Register("table", gtables.InstalledAppTableName, "CancelPending", functions.For(CancelPending))
	functions.Register("table", gtables.InstalledAppTableName, "BuildList", functions.For(BuildList))
}

// newGitHubFetcher reads files from GitHub (GITHUB_TOKEN for private repositories); tests
// replace it with a fake GitHub
var newGitHubFetcher = func() *themes.Fetcher { return themes.NewFetcher() }

// manifestMax: the largest application.yaml read
const manifestMax = 64 << 10

// superOnly refuses a user who is not SUPER (internal code without a user is allowed)
func superOnly(lang string) error {
	access := fcodeunits.CurrentAccess()
	if access != nil && !access.IsSuper {
		recordRefused("app action (SUPER only)")
		return errors.New(i18n.GetInstance().Message("ERR_APP_SUPER_ONLY", lang))
	}
	return nil
}

// InstallApp asks for the app's repository, version and folder, reads and checks its
// application.yaml and records it as Pending Install (an installed app in another version: an
// update)
func InstallApp(rec *gtables.InstalledApp) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	err := superOnly(lang)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	values := fcodeunits.RequestInput(ts.Message("APP_INSTALL_TITLE", lang), []fcodeunits.InputField{
		{Name: "repository", Label: ts.Message("APP_INSTALL_REPOSITORY", lang), Type: "text", Required: true},
		{Name: "version", Label: ts.Message("APP_INSTALL_VERSION", lang), Type: "text", Required: true},
		{Name: "folder", Label: ts.Message("APP_INSTALL_FOLDER", lang), Type: "text", Default: "extensions"},
	})
	if values == nil {
		return fcodeunits.Result{Success: true}, nil
	}
	repo, err := themes.ParseRepoURL(values["repository"])
	if err != nil || repo.Ref != "" {
		return fcodeunits.Result{}, errors.New(ts.Message("ERR_APP_REPOSITORY", lang))
	}
	// only from the owners and repositories App Build Setup names (blank: none)
	setup := gtables.GetAppBuildSetup(rec)
	if !fapps.RepositoryAllowed(setup.Allowed_repositories.String(), repo.Owner, repo.Name) {
		return fcodeunits.Result{}, errors.New(ts.MessageWithParams("ERR_APP_REPOSITORY_NOT_ALLOWED", lang, repo.Owner+"/"+repo.Name))
	}
	tag := strings.TrimSpace(values["version"])
	folder := strings.Trim(strings.TrimSpace(values["folder"]), "/")
	file := "application.yaml"
	if folder != "" {
		file = folder + "/" + file
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	fetcher := newGitHubFetcher()
	// the app is pinned to the commit its tag points to now: the build takes that commit and
	// refuses when the tag has moved
	commit, err := fetcher.TagCommit(ctx, repo, tag)
	if err != nil {
		return fcodeunits.Result{}, errors.New(ts.MessageWithParams("ERR_APP_TAG", lang, tag, repo.URL(), err.Error()))
	}
	repo.Ref = commit
	data, err := fetcher.File(ctx, repo, file, manifestMax)
	if err != nil {
		return fcodeunits.Result{}, errors.New(ts.MessageWithParams("ERR_APP_READ", lang, repo.URL()+" ("+tag+") "+file, err.Error()))
	}
	// the repository's address without the version: the build fetches the commit (Source
	// Commit) and checks that the tag (Source Ref) still points to it
	address := repo
	address.Ref = ""
	return installManifest(rec, data, address.URL(), tag, commit, folder)
}

// UploadApp asks for an app package (a .zip of the app's folder), checks it — the package as
// untrusted input, then the app as Install App does — and records the app as Pending Install
// with the package stored for the installation's builder (App Build Setup: Allow Upload)
func UploadApp(rec *gtables.InstalledApp) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	err := superOnly(lang)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	setup := gtables.GetAppBuildSetup(rec)
	if !setup.Allow_upload {
		return fcodeunits.Result{}, errors.New(ts.Message("ERR_APP_UPLOAD_NOT_ALLOWED", lang))
	}
	values := fcodeunits.RequestInput(ts.Message("APP_UPLOAD_TITLE", lang), []fcodeunits.InputField{
		{Name: "package", Label: ts.Message("APP_UPLOAD_FILE", lang), Type: "file", Required: true},
	})
	if values == nil {
		return fcodeunits.Result{Success: true}, nil
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(values["package"]))
	if err != nil || len(data) == 0 {
		return fcodeunits.Result{}, errors.New(ts.MessageWithParams("ERR_APP_PACKAGE", lang, "no file"))
	}
	return uploadPackage(rec, data)
}

// uploadPackage checks an app package and records its app as Pending Install from the package
func uploadPackage(src *gtables.InstalledApp, data []byte) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	pkg, err := fapps.ReadPackage(data)
	if err != nil {
		return fcodeunits.Result{}, errors.New(ts.MessageWithParams("ERR_APP_PACKAGE", lang, err.Error()))
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	// the app audit (S55): a package that fails it is not recorded
	err = auditPackage(src, pkg, hash)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	save := func(app *fapps.App) error {
		pkgRec := gtables.NewAppPackage(src)
		exists := pkgRec.Get(map[string]interface{}{"app_id": app.ID, "version": app.Version})
		if !exists {
			pkgRec.InitRecord()
			pkgRec.App_id = types.NewText(app.ID)
			pkgRec.Version = types.NewText(app.Version)
		}
		pkgRec.Name = types.NewText(app.Name)
		pkgRec.Package = data
		pkgRec.Sha256 = types.NewText(hash)
		pkgRec.Size = len(data)
		pkgRec.Uploaded_by = types.NewCode(fcodeunits.CurrentUserID())
		pkgRec.Uploaded_at = types.NewDateTimeFromTime(time.Now())
		if exists {
			return pkgRec.ModifyOrError(false)
		}
		return pkgRec.InsertOrError(false)
	}
	return installChecked(src, pkg.Manifest, appSource{kind: gtables.InstalledApp_Source_type.Package,
		source: "package " + hash[:12], ref: pkg.App.Version, save: save})
}

// auditPackage runs the app audit on an uploaded package, with App Build Setup's allowed modules
// and the exceptions approved for this package; refused: the report as the error (it names the
// package's SHA-256, which an exception is approved for)
func auditPackage(src *gtables.InstalledApp, pkg *fapps.Package, hash string) error {
	opts := appaudit.Options{AllowedModules: allowedGoModules(src)}
	for _, e := range auditExceptions(src, pkg.App.ID, pkg.App.Version, hash) {
		opts.Exceptions = append(opts.Exceptions, appaudit.Exception{Rule: e.Rule, Key: e.Key})
	}
	report := appaudit.Audit(pkg.App.Name+" "+pkg.App.Version, pkg.Files, opts)
	if report.Passed() {
		return nil
	}
	lang := fcodeunits.CurrentLanguage()
	return errors.New(i18n.GetInstance().MessageWithParams("ERR_APP_AUDIT_FAILED", lang, pkg.App.Name, pkg.App.Version, hash, report.Text()))
}

// installManifest checks an app's application.yaml against the planned apps and records it as
// Pending Install, taken from a Git repository
func installManifest(src *gtables.InstalledApp, data []byte, source, ref, commit, folder string) (fcodeunits.Result, error) {
	return installChecked(src, data, appSource{kind: gtables.InstalledApp_Source_type.Git, source: source, ref: ref, commit: commit, folder: folder})
}

// appSource: where the build takes an app from — a Git repository (source, ref, folder), or a
// package (save stores it once the app is checked)
type appSource struct {
	kind   gtables.InstalledAppSource_type
	source string
	ref    string
	commit string // the commit the tag pointed to when the app was chosen (Git)
	folder string
	save   func(app *fapps.App) error
}

// installChecked checks an app's application.yaml against the planned apps and records it as
// Pending Install with its source
func installChecked(src *gtables.InstalledApp, data []byte, from appSource) (fcodeunits.Result, error) {
	source, ref, folder := from.source, from.ref, from.folder
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	app, err := fapps.Parse(data)
	if err != nil {
		return fcodeunits.Result{}, errors.New(ts.MessageWithParams("ERR_APP_INSTALL_REFUSED", lang, source, err.Error()))
	}
	refuse := func(err error) (fcodeunits.Result, error) {
		return fcodeunits.Result{}, errors.New(ts.MessageWithParams("ERR_APP_INSTALL_REFUSED", lang, app.Name, err.Error()))
	}
	base, planned, err := plannedApps(src, app.ID)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	err = app.CheckExtension(base)
	if err != nil {
		return refuse(err)
	}
	planned = append(planned, app)
	err = fapps.CheckOverlap(append([]*fapps.App{base}, planned...))
	if err != nil {
		return refuse(err)
	}
	_, err = fapps.Order(base, planned)
	if err != nil {
		return refuse(err)
	}

	rec := gtables.NewInstalledApp(src)
	exists := rec.Get(types.NewText(app.ID))
	sameVersion := exists && rec.Status == gtables.InstalledApp_Status.Installed && rec.Version.String() == app.Version
	switch {
	// the same version again from Git: only to pin an app installed before commits were recorded
	case sameVersion && from.kind == gtables.InstalledApp_Source_type.Git && rec.Source_type == from.kind && rec.Source != "" && rec.Source_commit == "":
	case sameVersion && from.kind == gtables.InstalledApp_Source_type.Git && rec.Source_commit != "" && rec.Source_commit.String() != from.commit:
		// the version's tag points to another commit than the one installed: other code under
		// the same version — refused (the publisher must give it a new version)
		return fcodeunits.Result{}, errors.New(ts.MessageWithParams("ERR_APP_TAG_MOVED", lang, app.Name, from.ref, from.commit, rec.Source_commit.String()))
	case sameVersion:
		return fcodeunits.Result{}, errors.New(ts.MessageWithParams("ERR_APP_ALREADY_INSTALLED", lang, app.Name, app.Version))
	}
	if !exists {
		rec.InitRecord()
		rec.App_id = types.NewText(app.ID)
	}
	if from.save != nil {
		err = from.save(app)
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}
	rec.Name = types.NewText(app.Name)
	rec.Publisher = types.NewText(app.Publisher)
	rec.Contact = types.NewText(app.Contact)
	rec.Version = types.NewText(app.Version)
	rec.Object_series = types.NewText(seriesText(app.NumberSeries.Objects))
	rec.Field_series = types.NewText(seriesText(app.NumberSeries.Fields))
	rec.System_tables = types.NewText(seriesText(app.SystemTables))
	rec.Status = gtables.InstalledApp_Status.PendingInstall
	rec.Source = types.NewText(source)
	rec.Source_ref = types.NewText(ref)
	rec.Source_path = types.NewText(folder)
	rec.Source_commit = types.NewText(from.commit)
	rec.Source_type = from.kind
	rec.Manifest = data
	rec.Updated_at = types.NewDateTimeFromTime(time.Now())
	if exists {
		err = rec.ModifyOrError(false)
	} else {
		err = rec.InsertOrError(false)
	}
	if err != nil {
		return fcodeunits.Result{}, err
	}
	recordAdministration("install app " + app.Name + " " + app.Version + " from " + source)
	text := ts.MessageWithParams("APP_PENDING_INSTALL", lang, app.Name, app.Version)
	return fcodeunits.Result{Success: true, Message: text, Refresh: true}, nil
}

// plannedApps: the base application and the apps the next build contains — the installed ones
// not chosen to uninstall (as built), and those chosen to install (as their application.yaml) —
// without the app of id skip
func plannedApps(src *gtables.InstalledApp, skip string) (*fapps.App, []*fapps.App, error) {
	installedMu.RLock()
	built := installed
	installedMu.RUnlock()
	if len(built) == 0 {
		return nil, nil, errors.New("the installed apps are not known yet")
	}
	byID := map[string]*fapps.App{}
	for _, a := range built[1:] {
		byID[a.ID] = a
	}
	var planned []*fapps.App
	rec := gtables.NewInstalledApp(src)
	for found := rec.FindSet(); found; found = rec.Next() {
		id := rec.App_id.String()
		if id == skip || id == built[0].ID || rec.Status == gtables.InstalledApp_Status.PendingRemoval {
			continue
		}
		if rec.Status == gtables.InstalledApp_Status.PendingInstall {
			a, err := fapps.Parse(rec.Manifest)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", rec.Name, err)
			}
			planned = append(planned, a)
			continue
		}
		a, ok := byID[id]
		if ok {
			planned = append(planned, a)
		}
	}
	return built[0], planned, nil
}

// UninstallApp marks an app Pending Removal (an app chosen to install and not built in yet is
// taken back instead; an installed app with an update chosen is removed, update and all); refused
// for the base application and while another planned app depends on it
func UninstallApp(rec *gtables.InstalledApp) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	err := superOnly(lang)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	// an app chosen to install that is not built in yet: Uninstall takes the choice back
	if rec.Status == gtables.InstalledApp_Status.PendingInstall && builtIn(rec.App_id.String()) == nil {
		return CancelPending(rec)
	}
	base, planned, err := plannedApps(rec, rec.App_id.String())
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if rec.App_id.String() == base.ID {
		return fcodeunits.Result{}, errors.New(ts.MessageWithParams("ERR_APP_BASE_ALWAYS_ON", lang, rec.Name.String()))
	}
	for _, a := range planned {
		for _, d := range a.Dependencies {
			if d.ID == rec.App_id.String() {
				return fcodeunits.Result{}, errors.New(ts.MessageWithParams("ERR_APP_UNINSTALL_DEPENDED_ON", lang, rec.Name.String(), a.Name))
			}
		}
	}
	rec.Status = gtables.InstalledApp_Status.PendingRemoval
	err = rec.ModifyOrError(false)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	recordAdministration("uninstall app " + rec.Name.String())
	text := ts.MessageWithParams("APP_PENDING_REMOVAL", lang, rec.Name.String())
	return fcodeunits.Result{Success: true, Message: text, Refresh: true}, nil
}

// CancelPending takes back an install or uninstall not built yet: a pending install of an app
// not built in goes, an update goes back to the version built in, a pending removal is
// Installed again
func CancelPending(rec *gtables.InstalledApp) (fcodeunits.Result, error) {
	lang := fcodeunits.CurrentLanguage()
	err := superOnly(lang)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	switch rec.Status {
	case gtables.InstalledApp_Status.PendingRemoval:
		rec.Status = gtables.InstalledApp_Status.Installed
	case gtables.InstalledApp_Status.PendingInstall:
		builtApp := builtIn(rec.App_id.String())
		if builtApp == nil {
			err = rec.DeleteOrError(false)
			if err != nil {
				return fcodeunits.Result{}, err
			}
			return fcodeunits.Result{Success: true, Refresh: true}, nil
		}
		rec.Version = types.NewText(builtApp.Version)
		rec.Status = gtables.InstalledApp_Status.Installed
	default:
		return fcodeunits.Result{Success: true}, nil
	}
	err = rec.ModifyOrError(false)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// builtIn: the app of that id as built into the running program (nil when it is not)
func builtIn(id string) *fapps.App {
	installedMu.RLock()
	defer installedMu.RUnlock()
	for _, a := range installed {
		if a.ID == id {
			return a
		}
	}
	return nil
}

// plannedBuild: the build list of the next build — the base version and the planned apps in
// dependency order, each with its source (a package's SHA-256 from the stored App Package)
func plannedBuild(src *gtables.InstalledApp) (fapps.BuildList, error) {
	base, planned, err := plannedApps(src, "")
	if err != nil {
		return fapps.BuildList{}, err
	}
	return buildListOf(src, base, planned)
}

// buildListOf: the build list of the base and the planned apps, in dependency order, each with
// its source
func buildListOf(src *gtables.InstalledApp, base *fapps.App, planned []*fapps.App) (fapps.BuildList, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	var list fapps.BuildList
	ordered, err := fapps.Order(base, planned)
	if err != nil {
		return list, err
	}
	list.Base.Version = base.Version
	list.Apps = []fapps.BuildApp{}
	row := gtables.NewInstalledApp(src)
	for _, a := range ordered[1:] {
		if !row.Get(types.NewText(a.ID)) || row.Source == "" {
			return list, errors.New(ts.MessageWithParams("ERR_APP_NO_SOURCE", lang, a.Name))
		}
		app := fapps.BuildApp{ID: a.ID, Name: a.Name, Version: a.Version}
		if row.Source_type == gtables.InstalledApp_Source_type.Package {
			pkg := gtables.NewAppPackage(src)
			if !pkg.Get(map[string]interface{}{"app_id": a.ID, "version": a.Version}) {
				return list, errors.New(ts.MessageWithParams("ERR_APP_NO_SOURCE", lang, a.Name))
			}
			app.Source = fapps.SourcePackage
			app.PackageSHA256 = pkg.Sha256.String()
		} else {
			app.Source = fapps.SourceGit
			app.Repository = repositoryAddress(row.Source.String())
			app.Ref = row.Source_ref.String()
			app.Commit = row.Source_commit.String()
			app.Path = row.Source_path.String()
		}
		app.AuditExceptions = auditExceptions(src, a.ID, a.Version, app.SourceID())
		list.Apps = append(list.Apps, app)
	}
	list.AllowedGoModules = allowedGoModules(src)
	return list, nil
}

// auditExceptions: the app audit's findings a SUPER user approved for an app's exact source
// (App Audit Exception; S55)
func auditExceptions(src *gtables.InstalledApp, appID, version, sourceID string) []fapps.AuditException {
	exception := gtables.NewAppAuditException(src)
	exception.SetRange("app_id", appID)
	exception.SetRange("version", version)
	exception.SetRange("source_id", sourceID)
	var result []fapps.AuditException
	for found := exception.FindSet(); found; found = exception.Next() {
		result = append(result, fapps.AuditException{Rule: exception.Rule.String(), Key: exception.Finding_key.String()})
	}
	return result
}

// allowedGoModules: App Build Setup's Allowed Go Modules as a list
func allowedGoModules(src *gtables.InstalledApp) []string {
	setup := gtables.GetAppBuildSetup(src)
	return strings.FieldsFunc(setup.Allowed_go_modules.String(), func(r rune) bool {
		return r == ' ' || r == ',' || r == ';' || r == '\n' || r == '\t'
	})
}

// BuildList gives the list of apps the next build contains (apps.yaml) for a CI pipeline: the
// base version and the planned apps in dependency order, each with its repository, version and
// folder (an uploaded package refused: a pipeline cannot take it)
func BuildList(rec *gtables.InstalledApp) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	err := superOnly(lang)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	list, err := plannedBuild(rec)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	for i, a := range list.Apps {
		if a.Source == fapps.SourcePackage {
			return fcodeunits.Result{}, errors.New(ts.MessageWithParams("ERR_APP_PACKAGE_NEEDS_LOCAL_BUILDER", lang, a.Name))
		}
		list.Apps[i].Source = ""
	}
	out, err := yaml.Marshal(list)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	header := "# The apps of this installation's next build (Installed Apps → Build List): commit it as\n# apps.yaml to the installation repository, whose workflow builds the image\n"
	file := append([]byte(header), out...)
	return fcodeunits.Result{Success: true, Data: map[string]interface{}{
		"file":     base64.StdEncoding.EncodeToString(file),
		"filename": "apps.yaml",
	}}, nil
}

// repositoryAddress: a repository's address as git clones it — without a version in the address
// (Install App of 0.1.163 and before stored https://github.com/owner/repo/tree/<tag>)
func repositoryAddress(source string) string {
	repo, err := themes.ParseRepoURL(source)
	if err != nil {
		return source
	}
	repo.Ref = ""
	return repo.URL()
}
