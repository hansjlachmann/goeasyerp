package apps

// BuildList is what a build of the program with apps contains: the base version and the
// extension apps in dependency order, each with where the build takes it from. Installed Apps
// writes it — as apps.yaml for a CI pipeline (Build List), or as JSON in an App Build request
// for the installation's own builder (Build Apps) — and the builder reads it.
type BuildList struct {
	Base struct {
		Version string `yaml:"version" json:"version"`
	} `yaml:"base" json:"base"`
	Apps []BuildApp `yaml:"apps" json:"apps"`
	// AllowedGoModules: Go modules the apps' go.mod may require besides the base (App Build
	// Setup), for the app audit (S55)
	AllowedGoModules []string `yaml:"allowed_go_modules,omitempty" json:"allowed_go_modules,omitempty"`
}

// AuditException is a finding of the app audit a SUPER user approved for this app's source
// (App Audit Exception): the audit's rule and the finding's key
type AuditException struct {
	Rule string `yaml:"rule" json:"rule"`
	Key  string `yaml:"key" json:"key"`
}

// Where a build takes an app from
const (
	SourceGit     = "git"
	SourcePackage = "package"
)

// BuildApp is one app of a build list: from a Git repository (Repository, Ref — a tag — and
// Path, the folder of its application.yaml) or from an uploaded package (its SHA-256; the App
// Package of the app's id and version)
type BuildApp struct {
	ID            string `yaml:"id" json:"id"`
	Name          string `yaml:"name" json:"name"`
	Version       string `yaml:"version" json:"version"`
	Source        string `yaml:"source,omitempty" json:"source"`
	Repository    string `yaml:"repository,omitempty" json:"repository,omitempty"`
	Ref           string `yaml:"ref,omitempty" json:"ref,omitempty"`
	Commit        string `yaml:"commit,omitempty" json:"commit,omitempty"` // the tag's commit when installed: built exactly, the tag must still point to it
	Path          string `yaml:"path,omitempty" json:"path,omitempty"`
	PackageSHA256 string `yaml:"package_sha256,omitempty" json:"package_sha256,omitempty"`
	// AuditExceptions: the app audit's findings approved for exactly this source (S55)
	AuditExceptions []AuditException `yaml:"audit_exceptions,omitempty" json:"audit_exceptions,omitempty"`
}

// SourceID is what an audit exception is bound to: the Git commit, or the package's SHA-256
func (a BuildApp) SourceID() string {
	if a.Source == SourcePackage {
		return a.PackageSHA256
	}
	return a.Commit
}
