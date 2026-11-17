package main

import (
	"fmt"
	"strings"
)

// Placement of what an extension adds into a list of the base definition (fields of a section
// or list, sections, actions, menu entries): `after: <key>` or `before: <key>` names an entry
// already in the list by its key (a field's source, a section's, action's or menu entry's name);
// without them the new entry goes at the end. An extension adds — it never replaces: an entry
// whose key is already in the list is refused.

// placeItem inserts item into list by its after/before key (both removed from the item) and
// marks it, and everything below it, with the app
func placeItem(list []interface{}, item map[string]interface{}, keyField, what, appID string) ([]interface{}, error) {
	key := fieldString(item, keyField)
	if key == "" {
		return nil, fmt.Errorf("a %s without %s", what, keyField)
	}
	if indexOf(list, keyField, key) >= 0 {
		return nil, fmt.Errorf("%s %q is already there: an extension adds, it does not replace", what, key)
	}
	after, before := fieldString(item, "after"), fieldString(item, "before")
	delete(item, "after")
	delete(item, "before")
	if after != "" && before != "" {
		return nil, fmt.Errorf("%s %q: give after or before, not both", what, key)
	}
	markApp(item, appID)

	at := len(list)
	anchor := after
	if anchor == "" {
		anchor = before
	}
	if anchor != "" {
		i := indexOf(list, keyField, anchor)
		if i < 0 {
			return nil, fmt.Errorf("%s %q: there is no %s %q to place it next to", what, key, what, anchor)
		}
		at = i
		if after != "" {
			at = i + 1
		}
	}
	list = append(list, nil)
	copy(list[at+1:], list[at:])
	list[at] = item
	return list, nil
}

// indexOf: the position of the entry whose key field has the value (names compared without
// letter case), -1 when there is none
func indexOf(list []interface{}, keyField, value string) int {
	for i, entry := range list {
		m, ok := entry.(map[string]interface{})
		if ok && strings.EqualFold(fieldString(m, keyField), value) {
			return i
		}
	}
	return -1
}

// markApp marks an added definition and the definitions inside it (a section's fields, a menu
// button's or menu entry's items) with the app that added it: the page and menu metadata carry
// the mark, so a disabled app's parts can be left out
func markApp(item map[string]interface{}, appID string) {
	item["app"] = appID
	for _, key := range []string{"fields", "items"} {
		children, _ := item[key].([]interface{})
		for _, c := range children {
			m, ok := c.(map[string]interface{})
			if ok {
				markApp(m, appID)
			}
		}
	}
}

// itemList reads a list of definitions (maps) from an extension
func itemList(v interface{}, what string) ([]map[string]interface{}, error) {
	if v == nil {
		return nil, nil
	}
	list, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be a list", what)
	}
	items := make([]map[string]interface{}, 0, len(list))
	for _, entry := range list {
		m, ok := entry.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("%s: every entry must be a definition (key: value)", what)
		}
		items = append(items, m)
	}
	return items, nil
}

// checkKeys refuses keys an extension file does not know (a typo would otherwise be ignored)
func checkKeys(m map[string]interface{}, what string, allowed ...string) error {
	for key := range m {
		known := false
		for _, a := range allowed {
			if key == a {
				known = true
			}
		}
		if !known {
			return fmt.Errorf("%s: unknown key %q (allowed: %s)", what, key, strings.Join(allowed, ", "))
		}
	}
	return nil
}
