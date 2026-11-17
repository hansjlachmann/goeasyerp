package main

import (
	"testing"

	fapps "github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/extensions"
)

// The extension apps whose Go code the build installed (apps_gen.go, written by extmerge) are
// installed apps, in the installed apps' dependency order (none without extensions)
func TestExtensionCodeOfInstalledApps(t *testing.T) {
	installed, err := fapps.LoadInstalled()
	if err != nil {
		t.Fatal(err)
	}
	position := map[string]int{}
	for i, a := range installed {
		position[a.ID] = i
	}
	last := 0
	for _, id := range extensions.Installed() {
		i, ok := position[id]
		if !ok {
			t.Errorf("the Go code of app %s is installed, but the app is not", id)
			continue
		}
		if i < last {
			t.Errorf("app %s is installed before an app it comes after", id)
		}
		last = i
	}
}
