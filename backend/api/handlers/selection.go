package handlers

import (
	"github.com/gofiber/fiber/v2"

	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
)

// maxSelectionRecords: the most records a codeunit can be run on from a list selection
const maxSelectionRecords = 1000

// listSelection is the selection of a list a codeunit is run from: keys of the selected
// records, or all records of the list (its query in the URL's query parameters)
type listSelection struct {
	Keys []string `json:"keys"`
	All  bool     `json:"all"`
}

// selectionSetter: a generated record that keeps a list selection (SetSelectionFilter)
type selectionSetter interface {
	SetSelection(keys []map[string]interface{})
}

// applySelection passes the records selected on the list to the codeunit's record, which
// limits itself to them with SetSelectionFilter. newTable returns a record of the same
// table (the list's query is applied to it, not to the codeunit's record).
func applySelection(c *fiber.Ctx, record ftables.Table, sel *listSelection, newTable func() ftables.Table, language string) error {
	if sel == nil || (!sel.All && len(sel.Keys) == 0) {
		return nil
	}
	setter, ok := record.(selectionSetter)
	if !ok {
		return nil
	}
	keys, err := selectedKeys(c, newTable(), language, sel.Keys, sel.All, maxSelectionRecords)
	if err != nil {
		return err
	}
	if len(keys) > maxSelectionRecords {
		return apperrors.SelectionTooMany(maxSelectionRecords)
	}
	var pkFields []string
	for _, f := range record.GetFields() {
		if f.PrimaryKey {
			pkFields = append(pkFields, f.Name)
		}
	}
	maps := make([]map[string]interface{}, 0, len(keys))
	for _, id := range keys {
		switch key := parseRecordKey(id, record).(type) {
		case map[string]interface{}:
			maps = append(maps, key)
		default:
			if len(pkFields) == 1 {
				maps = append(maps, map[string]interface{}{pkFields[0]: key})
			}
		}
	}
	setter.SetSelection(maps)
	return nil
}
