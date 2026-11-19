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

// superOnly refuses a user with roles who is not SUPER
func superOnly(lang string) error {
	access := fcodeunits.CurrentAccess()
	if access != nil && access.HasRoles && !access.IsSuper {
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
	repo.Ref = strings.TrimSpace(values["version"])
	folder := strings.Trim(strings.TrimSpace(values["folder"]), "/")
	file := "application.yaml"
	if folder != "" {
		file = folder + "/" + file
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	data, err := newGitHubFetcher().File(ctx, repo, file, manifestMax)
	if err != nil {
		return fcodeunits.Result{}, errors.New(ts.MessageWithParams("ERR_APP_READ", lang, repo.URL()+" ("+repo.Ref+") "+file, err.Error()))
	}
	return installManifest(rec, data, repo.URL(), repo.Ref, folder)
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

// installManifest checks an app's application.yaml against the planned apps and records it as
// Pending Install, taken from a Git repository
func installManifest(src *gtables.InstalledApp, data []byte, source, ref, folder string) (fcodeunits.Result, error) {
	return installChecked(src, data, appSource{kind: gtables.InstalledApp_Source_type.Git, source: source, ref: ref, folder: folder})
}

// appSource: where the build takes an app from — a Git repository (source, ref, folder), or a
// package (save stores it once the app is checked)
type appSource struct {
	kind   gtables.InstalledAppSource_type
	source string
	ref    string
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
	if exists && rec.Status == gtables.InstalledApp_Status.Installed && rec.Version.String() == app.Version {
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
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	var list fapps.BuildList
	base, planned, err := plannedApps(src, "")
	if err != nil {
		return list, err
	}
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
			app.Repository = row.Source.String()
			app.Ref = row.Source_ref.String()
			app.Path = row.Source_path.String()
		}
		list.Apps = append(list.Apps, app)
	}
	return list, nil
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
