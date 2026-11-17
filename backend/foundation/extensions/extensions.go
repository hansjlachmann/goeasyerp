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
