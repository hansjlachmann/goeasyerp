package handlers

import (
	"fmt"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// writingValidateCustomer is a customer whose Name trigger writes — it saves the record and
// inserts another customer (as a changed sell-to customer recreates an order's lines)
type writingValidateCustomer struct {
	gtables.Customer
}

func (t *writingValidateCustomer) InitWithDBType(db database.Executor, company string, dbType database.DBType) {
	t.Customer.InitWithDBType(db, company, dbType)
	t.SetLogic(writingValidate{})
}

type writingValidate struct{}

func (writingValidate) OnValidate_Name(rec *gtables.Customer) error {
	rec.City = "Written"
	err := rec.ModifyOrError(false)
	if err != nil {
		return err
	}
	other := gtables.NewCustomer(rec)
	other.No = "ZZINSERTED"
	return other.InsertOrError(false)
}

func init() {
	gtables.RegisterTableFactory("Test_WritingValidate", 99003, func() ftables.Table {
		return &writingValidateCustomer{}
	})
}

// /validate only computes a record's new values: what its triggers write is never saved (it
// needs Read permission only) — the response still shows the trigger's values
func TestValidateNeverSaves(t *testing.T) {
	app := newTablesTestApp(t)
	postJSON(t, app, "/api/tables/Customer/insert", `{"no":"C01","name":"Anna","city":"Oslo"}`)
	out := postJSON(t, app, "/api/tables/Test_WritingValidate/validate", `{"field":"name","value":"Changed","record":{"no":"C01","name":"Anna","city":"Oslo"}}`)
	if out["success"] != true {
		t.Fatalf("validate: %v", out)
	}
	if data, _ := out["data"].(map[string]interface{}); data["city"] != "Written" || data["name"] != "Changed" {
		t.Errorf("the response: %v (want the trigger's values)", data)
	}
	_, list := getJSON(t, app, "/api/tables/Customer/list")
	nos, _ := listNos(t, list)
	if fmt.Sprint(nos) != "[C01]" {
		t.Errorf("customers after validate: %v (the trigger's insert was saved)", nos)
	}
	records, _ := list["data"].(map[string]interface{})["records"].([]interface{})
	if len(records) == 1 {
		if city := records[0].(map[string]interface{})["city"]; city != "Oslo" {
			t.Errorf("the stored customer's city is %v (the trigger's Modify was saved)", city)
		}
	}
}
