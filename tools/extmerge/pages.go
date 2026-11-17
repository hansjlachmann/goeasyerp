package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
)

// A page extension (pages/<id>-<name>.extend.yaml) adds to an existing page — card sections and
// their fields, list columns, actions — and never replaces what is there:
//
//	extends: 21                      # the page
//	layout:
//	  sections:                      # a card's sections
//	    - name: General              #   an existing section: its fields are added
//	      fields:
//	        - source: lp_loyalty_points
//	          after: credit_limit_lcy
//	    - name: Loyalty              #   a new section (after:/before: a section)
//	      after: Payments
//	      fields: [...]
//	  repeater:                      # a list page's columns
//	    fields:
//	      - source: lp_loyalty_points
//	        after: balance_lcy
//	actions:
//	  - name: Loyalty Tiers          # a button (after:/before: an action)
//	    after: Customer
//	    run_page: 50000
//	  - name: Recalculate Points     # an item of an existing menu button
//	    in: Functions
//	    run_object: "table:Customer.LpRecalculate"
//
// The page is merged as it is written — every key of the base definition and of the extension
// is kept — and everything added is marked with the app (`app: <id>`).

// mergePageExtension merges one page extension file into the merged definitions
func mergePageExtension(extPath, outputPath string, app *apps.App, verbose bool) error {
	data, err := os.ReadFile(extPath)
	if err != nil {
		return fmt.Errorf("failed to read extension: %w", err)
	}
	var ext map[string]interface{}
	err = yaml.Unmarshal(data, &ext)
	if err != nil {
		return fmt.Errorf("failed to parse extension: %w", err)
	}
	err = checkKeys(ext, "page extension", "extends", "version", "layout", "actions")
	if err != nil {
		return err
	}
	pageID, _ := ext["extends"].(int)
	if pageID == 0 {
		return fmt.Errorf("extension must specify 'extends' field with page ID")
	}

	corePagePath, err := findCorePageFile(outputPath, pageID)
	if err != nil {
		return fmt.Errorf("failed to find core page %d: %w", pageID, err)
	}
	coreData, err := os.ReadFile(corePagePath)
	if err != nil {
		return fmt.Errorf("failed to read core page: %w", err)
	}
	var def map[string]interface{}
	err = yaml.Unmarshal(coreData, &def)
	if err != nil {
		return fmt.Errorf("failed to parse core page: %w", err)
	}
	page, ok := def["page"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("core page %d has no page section", pageID)
	}

	err = mergePage(page, ext, app.ID)
	if err != nil {
		return fmt.Errorf("page %d: %w", pageID, err)
	}

	merged, err := yaml.Marshal(def)
	if err != nil {
		return fmt.Errorf("failed to marshal merged page: %w", err)
	}
	err = os.WriteFile(corePagePath, merged, 0644)
	if err != nil {
		return fmt.Errorf("failed to write merged page: %w", err)
	}
	if verbose {
		fmt.Printf("Merged page extension: %s -> page %d\n", filepath.Base(extPath), pageID)
	}
	return nil
}

// mergePage adds an extension's sections, list columns and actions to a page definition
func mergePage(page, ext map[string]interface{}, appID string) error {
	extLayout, _ := ext["layout"].(map[string]interface{})
	if ext["layout"] != nil && extLayout == nil {
		return fmt.Errorf("layout must be a definition")
	}
	if extLayout != nil {
		err := checkKeys(extLayout, "layout", "sections", "repeater")
		if err != nil {
			return err
		}
		layout, _ := page["layout"].(map[string]interface{})
		if layout == nil {
			layout = map[string]interface{}{}
			page["layout"] = layout
		}
		err = mergeSections(layout, extLayout["sections"], appID)
		if err != nil {
			return err
		}
		err = mergeRepeater(layout, extLayout["repeater"], appID)
		if err != nil {
			return err
		}
	}
	return mergeActions(page, ext["actions"], appID)
}

// mergeSections: the fields of an existing section are added to it, a new section is placed
// among the sections
func mergeSections(layout map[string]interface{}, extSections interface{}, appID string) error {
	sectionsAdded, err := itemList(extSections, "layout.sections")
	if err != nil || len(sectionsAdded) == 0 {
		return err
	}
	sections, _ := layout["sections"].([]interface{})
	if layout["repeater"] != nil && sections == nil {
		return fmt.Errorf("the page is a list: add its columns under layout.repeater, not sections")
	}
	for _, s := range sectionsAdded {
		name := fieldString(s, "name")
		i := indexOf(sections, "name", name)
		if i < 0 {
			sections, err = placeItem(sections, s, "name", "section", appID)
			if err != nil {
				return err
			}
			continue
		}
		// an existing section: only its fields are added (its own settings stay the base's)
		err = checkKeys(s, fmt.Sprintf("section %q (already on the page)", name), "name", "fields")
		if err != nil {
			return err
		}
		section := sections[i].(map[string]interface{})
		fields, _ := section["fields"].([]interface{})
		fields, err = placeFields(fields, s["fields"], fmt.Sprintf("section %q", name), appID)
		if err != nil {
			return err
		}
		section["fields"] = fields
	}
	layout["sections"] = sections
	return nil
}

// mergeRepeater adds columns to a list page
func mergeRepeater(layout map[string]interface{}, extRepeater interface{}, appID string) error {
	if extRepeater == nil {
		return nil
	}
	ext, ok := extRepeater.(map[string]interface{})
	if !ok {
		return fmt.Errorf("layout.repeater must be a definition")
	}
	err := checkKeys(ext, "layout.repeater", "fields")
	if err != nil {
		return err
	}
	repeater, _ := layout["repeater"].(map[string]interface{})
	if repeater == nil {
		return fmt.Errorf("the page has no list (layout.repeater): add its fields under layout.sections")
	}
	fields, _ := repeater["fields"].([]interface{})
	fields, err = placeFields(fields, ext["fields"], "the list", appID)
	if err != nil {
		return err
	}
	repeater["fields"] = fields
	return nil
}

// placeFields places the extension's fields among a section's or list's fields
func placeFields(fields []interface{}, added interface{}, where, appID string) ([]interface{}, error) {
	list, err := itemList(added, where+": fields")
	if err != nil {
		return nil, err
	}
	for _, f := range list {
		fields, err = placeItem(fields, f, "source", "field", appID)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", where, err)
		}
	}
	return fields, nil
}

// mergeActions adds buttons among the page's actions, or — with `in: <menu button>` — items to
// one of its menu buttons (found at any depth). Action names stay unique on the page.
func mergeActions(page map[string]interface{}, extActions interface{}, appID string) error {
	added, err := itemList(extActions, "actions")
	if err != nil || len(added) == 0 {
		return err
	}
	actions, _ := page["actions"].([]interface{})
	for _, a := range added {
		name := fieldString(a, "name")
		if name != "" && findAction(actions, name) != nil {
			return fmt.Errorf("action %q is already on the page: an extension adds, it does not replace", name)
		}
		menuName := fieldString(a, "in")
		delete(a, "in")
		if menuName == "" {
			actions, err = placeItem(actions, a, "name", "action", appID)
			if err != nil {
				return err
			}
			continue
		}
		menu := findAction(actions, menuName)
		if menu == nil {
			return fmt.Errorf("action %q: the page has no menu button %q", name, menuName)
		}
		items, isMenu := menu["items"].([]interface{})
		if !isMenu {
			return fmt.Errorf("action %q: %q is a button, not a menu button (it has no items)", name, menuName)
		}
		items, err = placeItem(items, a, "name", "action", appID)
		if err != nil {
			return fmt.Errorf("menu %q: %w", menuName, err)
		}
		menu["items"] = items
	}
	page["actions"] = actions
	return nil
}

// findAction: the action of that name at any depth of the page's menus, nil when there is none
func findAction(actions []interface{}, name string) map[string]interface{} {
	for _, entry := range actions {
		a, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		if strings.EqualFold(fieldString(a, "name"), name) {
			return a
		}
		items, _ := a["items"].([]interface{})
		found := findAction(items, name)
		if found != nil {
			return found
		}
	}
	return nil
}

// findCorePageFile finds the page definition <id>-<name>.yaml in the merged definitions
func findCorePageFile(outputPath string, pageID int) (string, error) {
	pagesPath := filepath.Join(outputPath, "pages")
	entries, err := os.ReadDir(pagesPath)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".extend.yaml") {
			continue
		}
		var id int
		_, err := fmt.Sscanf(strings.SplitN(name, "-", 2)[0], "%d", &id)
		if err == nil && id == pageID {
			return filepath.Join(pagesPath, name), nil
		}
	}
	return "", fmt.Errorf("page with ID %d not found", pageID)
}
