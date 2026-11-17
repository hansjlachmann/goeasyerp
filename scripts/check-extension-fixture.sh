#!/usr/bin/env bash
# Builds the base application with the test extension app (testdata/apps/loyalty-points) the
# way an installation's build does — extmerge merges the app's definitions into the base,
# tablegen generates the table code again — and checks the result: it builds, every page
# definition loads with its captions in every language, and the server's installed apps (the
# base and the test app, in dependency order) cover every object; the app's Go code is linked
# into the program (go.work, cmd/api-server/apps_gen.go) and its own tests pass. Runs on a copy of the repository (the files git knows, also new ones not
# committed yet), so the working tree is never changed. CI runs it; run it locally the same way.
set -euo pipefail

root=$(git rev-parse --show-toplevel)
app=${1:-testdata/apps/loyalty-points}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

cd "$root"
git ls-files -co --exclude-standard -z -- ':!frontend' | while IFS= read -r -d '' f; do
  [ -e "$f" ] && printf '%s\0' "$f"
done | tar --null -T - -cf - | tar -C "$work" -xf -

cd "$work"
echo "Merging $app into the base definitions..."
go run ./tools/extmerge --core backend/definitions --extensions "$app" --output merged \
  --translations translations --translations-output merged-translations --go-root .
rm -rf backend/definitions translations
mv merged backend/definitions
mv merged-translations translations

echo "Generating the table code from the merged definitions..."
go generate ./backend/generated/tables > generate.log 2>&1 || { cat generate.log; exit 1; }

echo "Checking that the app's parts are in the merged definitions..."
grep -q 'lp_loyalty_points' backend/definitions/tables/customer.yaml
grep -q 'lp_loyalty_points' backend/definitions/pages/21-customer-card.yaml
grep -q 'lp_loyalty_points' backend/definitions/pages/22-customer-list.yaml
grep -q 'Loyalty Tiers' backend/definitions/menus/admin.yaml
ls backend/definitions/apps/*.yaml > /dev/null
ls translations/en-US/lp-tables.yaml > /dev/null
grep -q 'extensions.Install' cmd/api-server/apps_gen.go

echo "Building..."
go build ./...

echo "Checking the page and menu definitions, their captions and the installed apps..."
go test ./backend/api/handlers ./backend/business-logic/apps ./backend/foundation/apps ./cmd/api-server -count=1

echo "Running the app's own tests (its Go code against the merged base)..."
(cd "$app" && go test ./... -count=1)

echo "✓ The base application builds with $app"
