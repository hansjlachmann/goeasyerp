// Package extensions runs the Go code of the extension apps built into the program. Each app's
// Go module has a root package with a Register function that registers its business logic
// (gtables.Register<Table>Logic …); the build generates a call of Install per app
// (cmd/api-server/apps_gen.go, written by extmerge), in dependency order, after the base
// application's own logic is registered. While an app's Register runs, Current names it, so
// what it registers can be told apart from the base application's and other apps'.
package extensions

import (
	"fmt"
	"sync"
)

var (
	mu        sync.Mutex
	current   string
	installed []string
)

// Install runs an app's Register with the app as the current one. An app is installed once.
func Install(appID string, register func()) {
	mu.Lock()
	for _, id := range installed {
		if id == appID {
			mu.Unlock()
			panic(fmt.Sprintf("extension app %s is installed twice", appID))
		}
	}
	current = appID
	mu.Unlock()

	defer func() {
		mu.Lock()
		current = ""
		installed = append(installed, appID)
		mu.Unlock()
	}()
	register()
}

// Current is the app whose Register is running — "" for the base application (its logic
// registers in its packages' init, before any app)
func Current() string {
	mu.Lock()
	defer mu.Unlock()
	return current
}

// Installed lists the apps whose Go code was installed, in the order it was
func Installed() []string {
	mu.Lock()
	defer mu.Unlock()
	return append([]string(nil), installed...)
}

var (
	enabledMu sync.RWMutex
	disabled  = map[string]bool{}
)

// Enabled reports whether an app's code runs: the base application ("") always, an extension
// app unless it was disabled (SetDisabled)
func Enabled(appID string) bool {
	if appID == "" {
		return true
	}
	enabledMu.RLock()
	defer enabledMu.RUnlock()
	return !disabled[appID]
}

// SetDisabled sets the apps whose code does not run (the installed apps switched off); every
// other app's code runs
func SetDisabled(appIDs ...string) {
	set := map[string]bool{}
	for _, id := range appIDs {
		set[id] = true
	}
	enabledMu.Lock()
	disabled = set
	enabledMu.Unlock()
}

// EnabledLogic returns the registered logic of a table without the logic of disabled apps
// (apps[i] registered logic[i]); the list itself when no app is disabled
func EnabledLogic(logic []any, apps []string) []any {
	enabledMu.RLock()
	defer enabledMu.RUnlock()
	if len(disabled) == 0 {
		return logic
	}
	out := make([]any, 0, len(logic))
	for i, l := range logic {
		if i < len(apps) && disabled[apps[i]] {
			continue
		}
		out = append(out, l)
	}
	return out
}
