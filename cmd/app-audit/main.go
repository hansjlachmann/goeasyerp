// Command app-audit checks extension apps before they are built into GoEasyERP (S55): the same
// audit the installation's builder and Upload App run (foundation/appaudit). The CI pipeline of
// an installation repository runs it on every app before the build; app authors run it on their
// app before they release it.
//
//	go run github.com/hansjlachmann/goeasyerp/cmd/app-audit [-build-list apps.yaml] [-allow-module path[@version]]... [-json] <app folder>...
//
// With -build-list (an installation's apps.yaml) it takes the allowed Go modules and each app's
// approved exceptions from it, as the installation's builder does. It prints the report of each
// app and exits with 1 when an app fails.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/appaudit"
	fapps "github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
)

type list []string

func (l *list) String() string     { return strings.Join(*l, ",") }
func (l *list) Set(v string) error { *l = append(*l, v); return nil }

func main() {
	var modules list
	flag.Var(&modules, "allow-module", "a Go module the apps may require besides the base (path, or path@version); repeatable")
	asJSON := flag.Bool("json", false, "print the reports as JSON")
	buildList := flag.String("build-list", "", "an installation's apps.yaml: its allowed Go modules and the apps' approved exceptions")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: app-audit [-build-list apps.yaml] [-allow-module path[@version]]... [-json] <app folder>...")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	var list *fapps.BuildList
	if *buildList != "" {
		data, err := os.ReadFile(*buildList)
		if err != nil {
			fmt.Fprintf(os.Stderr, "app-audit: %v\n", err)
			os.Exit(2)
		}
		list = &fapps.BuildList{}
		err = yaml.Unmarshal(data, list)
		if err != nil {
			fmt.Fprintf(os.Stderr, "app-audit: %s: %v\n", *buildList, err)
			os.Exit(2)
		}
		modules = append(modules, list.AllowedGoModules...)
	}
	failed := false
	var reports []*appaudit.Report
	for _, dir := range flag.Args() {
		opts := appaudit.Options{AllowedModules: modules}
		name := filepath.Base(filepath.Clean(dir))
		if list != nil {
			app, err := fapps.Load(filepath.Join(dir, "application.yaml"))
			if err != nil {
				fmt.Fprintf(os.Stderr, "app-audit: %s: %v\n", dir, err)
				os.Exit(2)
			}
			name = app.Name + " " + app.Version
			for _, a := range list.Apps {
				if a.ID == app.ID && a.Version == app.Version {
					for _, e := range a.AuditExceptions {
						opts.Exceptions = append(opts.Exceptions, appaudit.Exception{Rule: e.Rule, Key: e.Key})
					}
				}
			}
		}
		r, err := appaudit.AuditDir(name, dir, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "app-audit: %s: %v\n", dir, err)
			os.Exit(2)
		}
		if !r.Passed() {
			failed = true
		}
		reports = append(reports, r)
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(reports)
	} else {
		for _, r := range reports {
			fmt.Print(r.Text())
		}
	}
	if failed {
		os.Exit(1)
	}
}
