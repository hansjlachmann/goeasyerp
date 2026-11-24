package handlers

import (
	"fmt"
	"testing"
)

// A list without a window and /ids answer at most their caps and say so (S30): a million
// ledger entries as JSON would run the server out of memory.
func TestListAndIDsAreCapped(t *testing.T) {
	app := newTablesTestApp(t)
	for i := 1; i <= 5; i++ {
		postJSON(t, app, "/api/tables/Customer/insert", fmt.Sprintf(`{"no":"C%02d","name":"Customer %d"}`, i, i))
	}

	oldRows, oldIDs := maxUnwindowedRows, maxRecordIDs
	t.Cleanup(func() { maxUnwindowedRows, maxRecordIDs = oldRows, oldIDs })
	maxUnwindowedRows, maxRecordIDs = 3, 4

	// No window: the first maxUnwindowedRows records, truncated
	_, out := getJSON(t, app, "/api/tables/Customer/list")
	nos, _ := listNos(t, out)
	data := out["data"].(map[string]interface{})
	if len(nos) != 3 || data["truncated"] != true {
		t.Errorf("no window: %v truncated %v, want 3 records, truncated", nos, data["truncated"])
	}

	// A window is not cut by the cap (it has its own, 1000) and is not truncated
	_, out = getJSON(t, app, "/api/tables/Customer/list?offset=0&limit=5")
	nos, total := listNos(t, out)
	data = out["data"].(map[string]interface{})
	if len(nos) != 5 || total != 5 || data["truncated"] != false {
		t.Errorf("window: %v total %d truncated %v, want 5 of 5", nos, total, data["truncated"])
	}

	// /ids: at most maxRecordIDs keys
	_, out = getJSON(t, app, "/api/tables/Customer/ids")
	data = out["data"].(map[string]interface{})
	ids, _ := data["ids"].([]interface{})
	if len(ids) != 4 || data["truncated"] != true {
		t.Errorf("/ids: %v truncated %v, want 4 keys, truncated", ids, data["truncated"])
	}

	// Under the caps nothing is truncated
	maxUnwindowedRows, maxRecordIDs = 10, 10
	_, out = getJSON(t, app, "/api/tables/Customer/list")
	nos, _ = listNos(t, out)
	data = out["data"].(map[string]interface{})
	if len(nos) != 5 || data["truncated"] != false {
		t.Errorf("under the cap: %v truncated %v, want all 5", nos, data["truncated"])
	}
	_, out = getJSON(t, app, "/api/tables/Customer/ids")
	data = out["data"].(map[string]interface{})
	ids, _ = data["ids"].([]interface{})
	if len(ids) != 5 || data["truncated"] != false {
		t.Errorf("/ids under the cap: %d keys truncated %v, want 5", len(ids), data["truncated"])
	}
}
