package pages

import (
	"errors"
	"testing"
)

type openTestRec struct{}

// OnOpenPage: the trigger reads the page's parameters, sets filters and parameters, or asks the
// user to pick a record first (the following triggers do not run then)
func TestRunOpenPage(t *testing.T) {
	const pageID = 99901
	OnOpenPage(pageID, func(rec *openTestRec, page *Open) error {
		if page.Param("Template") == "" {
			page.SelectRecord(250, "template", "name", FixedFilter{Field: "type", Value: 0})
			return nil
		}
		if page.Param("template") == "BAD" {
			return errors.New("no such template")
		}
		page.SetFilter("journal_template_name", page.Param("template"))
		page.SetFilter("Journal_Batch_Name", "X")
		page.SetFilter("journal_batch_name", "DEFAULT") // replaces the filter
		page.SetParam("batch", "DEFAULT")
		return nil
	})
	ran := 0
	OnOpenPage(pageID, func(rec *openTestRec, page *Open) error {
		ran++
		return nil
	})
	if !HasOpenTrigger(pageID) || HasOpenTrigger(pageID+1) {
		t.Fatal("HasOpenTrigger")
	}

	open, err := RunOpenPage(pageID, &openTestRec{}, nil)
	if err != nil || open.Selection() == nil || open.Selection().PageID != 250 || open.Selection().Param != "template" || ran != 0 {
		t.Fatalf("no template: selection %+v, err %v, later triggers ran %d", open.Selection(), err, ran)
	}

	open, err = RunOpenPage(pageID, &openTestRec{}, map[string]string{"template": "GENERAL"})
	if err != nil || open.Selection() != nil || ran != 1 {
		t.Fatalf("template given: %v %+v ran %d", err, open.Selection(), ran)
	}
	filters := open.Filters()
	if len(filters) != 2 || filters[0] != (FixedFilter{Field: "journal_template_name", Value: "GENERAL"}) || filters[1] != (FixedFilter{Field: "journal_batch_name", Value: "DEFAULT"}) {
		t.Errorf("filters %+v", filters)
	}
	if open.Params()["batch"] != "DEFAULT" || open.Params()["template"] != "GENERAL" {
		t.Errorf("params %+v", open.Params())
	}

	if _, err := RunOpenPage(pageID, &openTestRec{}, map[string]string{"template": "BAD"}); err == nil {
		t.Error("the trigger's error must stop the page")
	}
	if _, err := RunOpenPage(pageID, "wrong record", map[string]string{"template": "X"}); err == nil {
		t.Error("a trigger of another record type must fail")
	}
}
