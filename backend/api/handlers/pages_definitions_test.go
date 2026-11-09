package handlers

import (
	"path/filepath"
	"testing"

	_ "github.com/hansjlachmann/goeasyerp/backend/business-logic/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// checkMatrix: a matrix section's rows table has its value FlowField, row fields and filters;
// its record filters are FlowFilters of the rows set from fields of the card's table
func checkMatrix(t *testing.T, id int, mx *pages.Matrix, cardFields map[string]bool, exists func(int) bool) {
	t.Helper()
	factory, ok := gtables.GetTableFactory(mx.Table)
	if !ok {
		t.Errorf("page %d: matrix table %q does not exist", id, mx.Table)
		return
	}
	rec := factory()
	fields := map[string]bool{}
	for _, f := range rec.GetFields() {
		fields[f.Name] = true
	}
	flowFields := map[string]bool{}
	for _, f := range rec.GetFlowFields() {
		flowFields[f] = true
	}
	flowFilters := map[string]bool{}
	for _, f := range rec.GetFlowFilterFields() {
		flowFilters[f.Name] = true
	}
	if mx.Calculated {
		if !pages.HasMatrixCalculate(id) {
			t.Errorf("page %d: a calculated matrix without an OnMatrixCalculate trigger", id)
		}
		for rowField, cardField := range mx.RowFilters {
			if !fields[rowField] || !cardFields[cardField] {
				t.Errorf("page %d: matrix row filter %s: %s", id, rowField, cardField)
			}
		}
	} else if !flowFields[mx.Value] {
		t.Errorf("page %d: matrix value %q is not a FlowField of %s", id, mx.Value, mx.Table)
	}
	// with card_date_filter and no date filter the period goes into the card record instead; a
	// calculated matrix without periods needs none
	periodOnCard := mx.CardDateFilter != "" && mx.DateFilter == ""
	noPeriods := mx.Calculated && mx.NoPeriods && mx.DateFilter == ""
	if !periodOnCard && !noPeriods && !flowFilters[mx.DateFilter] {
		t.Errorf("page %d: matrix date filter %q is not a FlowFilter of %s", id, mx.DateFilter, mx.Table)
	}
	for rowField, cardField := range mx.RecordFilters {
		if !flowFilters[rowField] || !cardFields[cardField] {
			t.Errorf("page %d: matrix record filter %s: %s", id, rowField, cardField)
		}
	}
	for _, f := range mx.Filters {
		if !flowFilters[f.Source] {
			t.Errorf("page %d: matrix filter %q is not a FlowFilter of %s", id, f.Source, mx.Table)
		}
	}
	for _, f := range mx.Fields {
		if !fields[f.Source] {
			t.Errorf("page %d: matrix field %q is not a field of %s", id, f.Source, mx.Table)
		}
	}
	if mx.Drilldown != 0 && !exists(mx.Drilldown) {
		t.Errorf("page %d: matrix drills down to page %d, which does not exist", id, mx.Drilldown)
	}
	if mx.Editable && !pages.HasMatrixValidate(id) {
		t.Errorf("page %d: an editable matrix without an OnMatrixValidate trigger", id)
	}
}

// Every page definition loads; its source table exists, its fields are fields of that table
// (or calculated or virtual masked page fields), and the pages it opens (card, list, drilldowns, actions) exist
func TestPageDefinitions(t *testing.T) {
	registry := pages.GetRegistry()
	if err := registry.LoadPages(); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob("../../definitions/pages/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	all := registry.GetAllPages()
	if len(all) != len(files) {
		t.Errorf("%d page files, %d pages loaded (a file that does not load is only a startup warning)", len(files), len(all))
	}
	exists := func(id int) bool {
		_, ok := all[id]
		return ok
	}
	for id, p := range all {
		factory, ok := gtables.GetTableFactory(p.Page.SourceTable)
		if !ok {
			t.Errorf("page %d: source table %q does not exist", id, p.Page.SourceTable)
			continue
		}
		rec := factory()
		fields := map[string]bool{}
		for _, f := range rec.GetFields() {
			fields[f.Name] = true
		}
		for _, f := range rec.GetFlowFilterFields() {
			fields[f.Name] = true
		}
		var pageFields []pages.Field
		for _, s := range p.Page.Layout.Sections {
			pageFields = append(pageFields, s.Fields...)
		}
		if p.Page.Layout.Repeater != nil {
			pageFields = append(pageFields, p.Page.Layout.Repeater.Fields...)
		}
		for _, f := range pageFields {
			// a masked page field may be virtual (the User Card's password)
			if !f.Calculated && !f.Masked && !fields[f.Source] {
				t.Errorf("page %d %s: %q is not a field of %s", id, p.Page.Name, f.Source, p.Page.SourceTable)
			}
			if f.Drilldown != 0 && !exists(f.Drilldown) {
				t.Errorf("page %d: field %s drills down to page %d, which does not exist", id, f.Source, f.Drilldown)
			}
			if f.DrilldownPageField != "" && !fields[f.DrilldownPageField] {
				t.Errorf("page %d: field %s drills down to the page in %q, not a field of %s", id, f.Source, f.DrilldownPageField, p.Page.SourceTable)
			}
		}
		for _, s := range p.Page.Layout.Sections {
			if s.Matrix != nil {
				checkMatrix(t, id, s.Matrix, fields, exists)
			}
		}
		if p.Page.Layout.Matrix != nil {
			checkMatrix(t, id, p.Page.Layout.Matrix, fields, exists)
		}
		for _, other := range []int{p.Page.CardPageID, p.Page.ListPageID} {
			if other != 0 && !exists(other) {
				t.Errorf("page %d: card/list page %d does not exist", id, other)
			}
		}
		// The page finds an action by its name, also inside its menu buttons: a menu and one of
		// its items with the same name would open nothing.
		actionNames := map[string]bool{}
		var check func(actions []pages.Action)
		check = func(actions []pages.Action) {
			for _, a := range actions {
				if actionNames[a.Name] {
					t.Errorf("page %d: action name %q is used twice", id, a.Name)
				}
				actionNames[a.Name] = true
				if a.RunPage != 0 && !exists(a.RunPage) {
					t.Errorf("page %d: action %s opens page %d, which does not exist", id, a.Name, a.RunPage)
				}
				if a.RunPageField != "" && !fields[a.RunPageField] {
					t.Errorf("page %d: action %s opens the page in %q, not a field of %s", id, a.Name, a.RunPageField, p.Page.SourceTable)
				}
				for param, field := range a.RunPageParams {
					if !fields[field] {
						t.Errorf("page %d: action %s sets parameter %s from %q, not a field of %s", id, a.Name, param, field, p.Page.SourceTable)
					}
				}
				check(a.Items)
			}
		}
		check(p.Page.Actions)
	}
}
