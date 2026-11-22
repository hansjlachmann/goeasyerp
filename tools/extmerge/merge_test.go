package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const rewardsApp = `application:
  id: 22222222-2222-4222-8222-222222222222
  name: Rewards
  version: 1.0.0
  prefix: rw
  number_series:
    objects: [{from: 51000, to: 51999}]
    fields: [{from: 51000, to: 51999}]
  dependencies:
    - id: 760e20ec-7a66-4e3f-b6e5-83c94db5a4bd
    - id: 11111111-1111-4111-8111-111111111111
      name: Loyalty
`

// twoApps writes the base, Loyalty (its own tier table and page) and Rewards, which depends on
// Loyalty and extends Loyalty's table and page; it returns the directory and the merge options
func twoApps(t *testing.T, rewards string) (string, options) {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "core", "application.yaml"), baseApp)
	writeFile(t, filepath.Join(dir, "core", "tables", "customer.yaml"), coreCustomer)
	writeFile(t, filepath.Join(dir, "core", "pages", "21-customer-card.yaml"), coreCard)
	writeFile(t, filepath.Join(dir, "translations", "en-US", "tables.yaml"), "tables:\n  customer:\n    caption: Customer\n    fields:\n      name: Name\n")
	writeFile(t, filepath.Join(dir, "translations", "nb-NO", "tables.yaml"), "tables:\n  customer:\n    caption: Kunde\n")

	loyalty := filepath.Join(dir, "apps", "loyalty")
	writeFile(t, filepath.Join(loyalty, "application.yaml"), strings.Replace(extApp, "  name: Loyalty\n", "  name: Loyalty\n  version: 1.2.0\n", 1))
	writeFile(t, filepath.Join(loyalty, "custom", "tables", "50000-tier.yaml"), "table:\n  id: 50000\n  name: LP_Tier\n  fields:\n    - name: code\n      no: 50000\n      type: types.Code\n")
	writeFile(t, filepath.Join(loyalty, "custom", "pages", "50000-lp-tiers.yaml"), "page:\n  id: 50000\n  type: List\n  name: LP Tiers\n  source_table: LP_Tier\n  layout:\n    repeater:\n      fields:\n        - source: code\n")
	writeFile(t, filepath.Join(loyalty, "translations", "en-US", "tables.yaml"), "tables:\n  lp_tier:\n    caption: Loyalty Tier\n")

	rw := filepath.Join(dir, "apps", "rewards")
	writeFile(t, filepath.Join(rw, "application.yaml"), rewards)
	writeFile(t, filepath.Join(rw, "tables", "tier.extend.yaml"), "extends_table: 50000\nfields:\n  - name: rw_bonus\n    no: 51000\n    type: int\n")
	writeFile(t, filepath.Join(rw, "pages", "50000.extend.yaml"), "extends: 50000\nlayout:\n  repeater:\n    fields:\n      - source: rw_bonus\n        after: code\n")

	// the apps in the "wrong" order: Rewards first, it is merged after Loyalty
	return dir, options{
		core:               filepath.Join(dir, "core"),
		apps:               []string{rw, loyalty},
		output:             filepath.Join(dir, "out"),
		translations:       filepath.Join(dir, "translations"),
		translationsOutput: filepath.Join(dir, "out-translations"),
	}
}

// Several apps are merged in dependency order — an app extends what another app added — and each
// app's application.yaml is written for the server
func TestMergeSeveralApps(t *testing.T) {
	dir, opts := twoApps(t, rewardsApp)
	if err := merge(opts); err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(filepath.Join(dir, "out", "pages", "50000-lp-tiers.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var def map[string]interface{}
	if err := yaml.Unmarshal(page, &def); err != nil {
		t.Fatal(err)
	}
	if got := keys(list(def["page"].(map[string]interface{}), "layout", "repeater", "fields"), "source"); got != "code,rw_bonus" {
		t.Errorf("Loyalty's page extended by Rewards: %s", got)
	}
	for _, id := range []string{"11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"} {
		if _, err := os.Stat(filepath.Join(dir, "out", "apps", id+".yaml")); err != nil {
			t.Errorf("application.yaml of %s not written: %v", id, err)
		}
	}
	texts, err := os.ReadFile(filepath.Join(dir, "out-translations", "en-US", "lp-tables.yaml"))
	if err != nil || !strings.Contains(string(texts), "Loyalty Tier") {
		t.Errorf("the app's texts: %s %v", texts, err)
	}
	if base, _ := os.ReadFile(filepath.Join(dir, "out-translations", "nb-NO", "tables.yaml")); !strings.Contains(string(base), "Kunde") {
		t.Error("the base translations were not copied")
	}
}

func TestMergeDependencies(t *testing.T) {
	_, opts := twoApps(t, rewardsApp)
	opts.apps = opts.apps[:1] // Rewards without Loyalty
	if err := merge(opts); err == nil || !strings.Contains(err.Error(), "Rewards needs Loyalty") {
		t.Errorf("missing dependency: %v", err)
	}
	_, opts = twoApps(t, strings.Replace(rewardsApp, "      name: Loyalty\n", "      name: Loyalty\n      min_version: 2.0.0\n", 1))
	if err := merge(opts); err == nil || !strings.Contains(err.Error(), "version 2.0.0 or later, 1.2.0 is installed") {
		t.Errorf("version too low: %v", err)
	}
	_, opts = twoApps(t, strings.Replace(rewardsApp, "  dependencies:\n    - id: 760e20ec-7a66-4e3f-b6e5-83c94db5a4bd\n", "  dependencies:\n", 1))
	if err := merge(opts); err == nil || !strings.Contains(err.Error(), "depends on Base Application") {
		t.Errorf("no dependency on the base: %v", err)
	}
	_, opts = twoApps(t, strings.Replace(rewardsApp, "  prefix: rw\n", "", 1))
	if err := merge(opts); err == nil || !strings.Contains(err.Error(), "prefix") {
		t.Errorf("no prefix: %v", err)
	}
}

// An app adds texts only: a key the base or another app has, or a language the base has not, is
// refused; an app with texts needs the translation paths
func TestMergeTranslations(t *testing.T) {
	cases := map[string]struct{ file, content, want string }{
		"a base text":    {"en-US/tables.yaml", "tables:\n  customer:\n    fields:\n      name: Customer Name\n", "it never changes them — already there: tables.customer.fields.name (in tables.yaml)"},
		"another app's":  {"en-US/x.yaml", "tables:\n  lp_tier:\n    caption: Tier\n", "tables.lp_tier.caption (in lp-tables.yaml)"},
		"a new language": {"sv-SE/tables.yaml", "tables:\n  rw:\n    caption: X\n", "sv-SE is not a language of the base application"},
	}
	for name, c := range cases {
		dir, opts := twoApps(t, rewardsApp)
		writeFile(t, filepath.Join(dir, "apps", "rewards", "translations", c.file), c.content)
		if err := merge(opts); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", name, err, c.want)
		}
	}
	_, opts := twoApps(t, rewardsApp)
	opts.translations = ""
	if err := merge(opts); err == nil || !strings.Contains(err.Error(), "Loyalty has translations") {
		t.Errorf("without translation paths: %v", err)
	}
}

// With --go-root the apps that have a Go module are linked in: go.work joins their modules and
// points the base release they require at the base being built, apps_gen.go installs them in
// dependency order; an app's go.mod must require the base
func TestGoWiring(t *testing.T) {
	dir, opts := twoApps(t, rewardsApp)
	writeFile(t, filepath.Join(dir, "apps", "loyalty", "go.mod"), "module example.com/loyalty\n\ngo 1.26.0\n\nrequire (\n\tgithub.com/hansjlachmann/goeasyerp v0.1.160 // the base\n\tgithub.com/other/x v1.0.0\n)\n")
	writeFile(t, filepath.Join(dir, "apps", "rewards", "go.mod"), "module example.com/rewards\n\ngo 1.26.0\n\nrequire github.com/hansjlachmann/goeasyerp v0.1.170\n")
	opts.goRoot = filepath.Join(dir, "repo")
	if err := os.MkdirAll(filepath.Join(opts.goRoot, "backend", "installedapps"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := merge(opts); err != nil {
		t.Fatal(err)
	}
	work, _ := os.ReadFile(filepath.Join(opts.goRoot, "go.work"))
	for _, want := range []string{"\t.\n", filepath.Join(dir, "apps", "loyalty"), "replace github.com/hansjlachmann/goeasyerp v0.1.160 => ./", "replace github.com/hansjlachmann/goeasyerp v0.1.170 => ./"} {
		if !strings.Contains(string(work), want) {
			t.Errorf("go.work lacks %q:\n%s", want, work)
		}
	}
	gen, _ := os.ReadFile(filepath.Join(opts.goRoot, "backend", "installedapps", "apps_gen.go"))
	code := string(gen)
	lp, rw := strings.Index(code, `extensions.Install("11111111`), strings.Index(code, `extensions.Install("22222222`)
	if lp < 0 || rw < 0 || lp > rw || !strings.Contains(code, `lp "example.com/loyalty"`) || !strings.Contains(code, "lp.Register") {
		t.Errorf("apps_gen.go:\n%s", code)
	}

	writeFile(t, filepath.Join(dir, "apps", "rewards", "go.mod"), "module example.com/rewards\n\ngo 1.26.0\n")
	if err := merge(opts); err == nil || !strings.Contains(err.Error(), "must require github.com/hansjlachmann/goeasyerp") {
		t.Errorf("an app not requiring the base: %v", err)
	}
}
