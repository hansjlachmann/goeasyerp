package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
)

// A menu extension (menus/<menu file>.extend.yaml) adds entries to a main menu — the app's own
// pages, reports and routines — and never replaces what is there:
//
//	extends: admin.yaml               # the menu file
//	items:
//	  - in: [Sales, Reports]          # the sub-menu, by its names from the top (omitted: a new
//	    after: Customer - List        #   module at the top level); after:/before: an entry
//	    name: Loyalty Points
//	    run_object: "report:50000"
//	  - name: Loyalty                 # a new module with its own entries
//	    items:
//	      - name: Loyalty Tiers
//	        page_id: 50000
//
// Everything added is marked with the app (`app: <id>`).

// mergeMenuExtension merges one menu extension file into the merged definitions
func mergeMenuExtension(extPath, outputPath string, app *apps.App, verbose bool) error {
	data, err := os.ReadFile(extPath)
	if err != nil {
		return fmt.Errorf("failed to read extension: %w", err)
	}
	var ext map[string]interface{}
	err = yaml.Unmarshal(data, &ext)
	if err != nil {
		return fmt.Errorf("failed to parse extension: %w", err)
	}
	err = checkKeys(ext, "menu extension", "extends", "items")
	if err != nil {
		return err
	}
	file := fieldString(ext, "extends")
	if file == "" || filepath.Base(file) != file || !strings.HasSuffix(file, ".yaml") {
		return fmt.Errorf("extension must specify 'extends' with the menu file (e.g. admin.yaml)")
	}
	menuPath := filepath.Join(outputPath, "menus", file)
	coreData, err := os.ReadFile(menuPath)
	if err != nil {
		return fmt.Errorf("menu %s: %w", file, err)
	}
	var def map[string]interface{}
	err = yaml.Unmarshal(coreData, &def)
	if err != nil {
		return fmt.Errorf("failed to parse menu %s: %w", file, err)
	}
	menu, _ := def["menu"].([]interface{})

	added, err := itemList(ext["items"], "items")
	if err != nil {
		return err
	}
	for _, item := range added {
		path, err := menuSubPath(item["in"])
		if err != nil {
			return fmt.Errorf("menu %s: entry %q: %w", file, fieldString(item, "name"), err)
		}
		delete(item, "in")
		menu, err = addMenuEntry(menu, path, item, app.ID)
		if err != nil {
			return fmt.Errorf("menu %s: %w", file, err)
		}
	}
	def["menu"] = menu

	merged, err := yaml.Marshal(def)
	if err != nil {
		return fmt.Errorf("failed to marshal merged menu: %w", err)
	}
	err = os.WriteFile(menuPath, merged, 0644)
	if err != nil {
		return fmt.Errorf("failed to write merged menu: %w", err)
	}
	if verbose {
		fmt.Printf("Merged menu extension: %s -> %s\n", filepath.Base(extPath), file)
	}
	return nil
}

// menuSubPath: the names of the sub-menu an entry goes into ("in": one name or a list of names)
func menuSubPath(v interface{}) ([]string, error) {
	switch in := v.(type) {
	case nil:
		return nil, nil
	case string:
		return []string{in}, nil
	case []interface{}:
		var path []string
		for _, p := range in {
			s, ok := p.(string)
			if !ok || s == "" {
				return nil, fmt.Errorf("in: the names of the sub-menus from the top")
			}
			path = append(path, s)
		}
		return path, nil
	}
	return nil, fmt.Errorf("in: a sub-menu name or a list of names from the top")
}

// addMenuEntry places the entry in the sub-menu the path names (the top level when it is empty)
func addMenuEntry(entries []interface{}, path []string, item map[string]interface{}, appID string) ([]interface{}, error) {
	if len(path) == 0 {
		return placeItem(entries, item, "name", "menu entry", appID)
	}
	i := indexOf(entries, "name", path[0])
	if i < 0 {
		return nil, fmt.Errorf("entry %q: there is no menu %q", fieldString(item, "name"), path[0])
	}
	sub := entries[i].(map[string]interface{})
	items, isMenu := sub["items"].([]interface{})
	if !isMenu {
		return nil, fmt.Errorf("entry %q: %q is not a sub-menu (it has no items)", fieldString(item, "name"), path[0])
	}
	items, err := addMenuEntry(items, path[1:], item, appID)
	if err != nil {
		return nil, err
	}
	sub["items"] = items
	return entries, nil
}
