// Package installedapps installs the Go code of the extension apps a build merged in: the build
// generates apps_gen.go here (extmerge --go-root), which runs each app's Register in dependency
// order (extensions.Install). Importing the package — the server does, and so do the tests
// that check what apps register (page functions) — installs them; without apps it installs
// nothing. It imports the base application's business logic first, so the base's logic is
// always registered before any app's.
package installedapps

import (
	// the base application's table logic, registered before the apps'
	_ "github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
)
