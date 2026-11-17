package apps

import (
	"strings"
	"testing"
)

func TestBaseApplication(t *testing.T) {
	app, err := LoadBase()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		objectType string
		id         int
		want       bool
	}{
		{Table, 18, true}, {Page, 999, true}, {Codeunit, 1000, true}, {Codeunit, 1300, false}, {Report, 0, false},
		{Table, 2000000002, true}, {Table, 2000000200, false},
		{Page, 2000000002, false}, {Codeunit, 2000000006, false}, // system range: tables only
	}
	for _, c := range cases {
		if got := app.ObjectAllowed(c.objectType, c.id); got != c.want {
			t.Errorf("%s %d allowed = %v, want %v", c.objectType, c.id, got, c.want)
		}
	}
	if !app.FieldAllowed(999) || app.FieldAllowed(1000) || app.FieldAllowed(0) {
		t.Error("base fields are 1–999")
	}
	if err := app.CheckObject(Codeunit, 50100, "Create Demo Data"); err == nil || !strings.Contains(err.Error(), "codeunit 50100 Create Demo Data is outside") {
		t.Errorf("CheckObject: %v", err)
	}
}

func TestParseRejects(t *testing.T) {
	base := `application:
  id: 760e20ec-7a66-4e3f-b6e5-83c94db5a4bd
  name: X
  number_series:
    objects: [{from: 1, to: 999}]
    fields: [{from: 1, to: 999}]
`
	if _, err := Parse([]byte(base)); err != nil {
		t.Fatalf("valid: %v", err)
	}
	cases := map[string]string{
		"bad id":        strings.Replace(base, "760e20ec-7a66-4e3f-b6e5-83c94db5a4bd", "abc", 1),
		"no name":       strings.Replace(base, "name: X", "name: ''", 1),
		"no fields":     strings.Replace(base, "    fields: [{from: 1, to: 999}]\n", "", 1),
		"reversed":      strings.Replace(base, "objects: [{from: 1, to: 999}]", "objects: [{from: 9, to: 1}]", 1),
		"overlap":       strings.Replace(base, "objects: [{from: 1, to: 999}]", "objects: [{from: 1, to: 999}, {from: 500, to: 1500}]", 1),
		"unknown field": base + "  colour: red\n",
	}
	for name, data := range cases {
		if _, err := Parse([]byte(data)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestCheckOverlap(t *testing.T) {
	app := func(id string, from, to int) *App {
		a := &App{ID: id, Name: id}
		a.NumberSeries.Objects = []Range{{from, to}}
		a.NumberSeries.Fields = []Range{{from, to}}
		return a
	}
	base := app("760e20ec-7a66-4e3f-b6e5-83c94db5a4bd", 1, 999)
	if err := CheckOverlap([]*App{base, app("11111111-1111-4111-8111-111111111111", 50000, 50999)}); err != nil {
		t.Errorf("separate series: %v", err)
	}
	if err := CheckOverlap([]*App{base, app("11111111-1111-4111-8111-111111111111", 900, 1999)}); err == nil {
		t.Error("overlapping series accepted")
	}
	if err := CheckOverlap([]*App{base, base}); err == nil {
		t.Error("same id twice accepted")
	}
}

func testApp(name, id, version string, deps ...Dependency) *App {
	return &App{ID: id, Name: name, Version: version, Prefix: "x" + name[:1], Dependencies: deps}
}

// Apps are ordered after the apps they depend on; missing, too old, renamed and circular
// dependencies are refused
func TestOrder(t *testing.T) {
	base := testApp("Base", "b", "0.1.160")
	onBase := Dependency{ID: "b", Name: "Base", MinVersion: "0.1.100"}
	loyalty := testApp("Loyalty", "l", "1.2.0", onBase)
	rewards := testApp("Rewards", "r", "1.0.0", onBase, Dependency{ID: "l", Name: "Loyalty", MinVersion: "1.1"})

	ordered, err := Order(base, []*App{rewards, loyalty})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, a := range ordered {
		names = append(names, a.Name)
	}
	if strings.Join(names, ",") != "Base,Loyalty,Rewards" {
		t.Errorf("order %v", names)
	}

	for name, c := range map[string]struct {
		apps []*App
		want string
	}{
		"missing":      {[]*App{rewards}, "Rewards needs Loyalty (id l, version 1.1 or later), which is not installed"},
		"too old":      {[]*App{rewards, testApp("Loyalty", "l", "1.0.9", onBase)}, "Rewards needs Loyalty version 1.1 or later, 1.0.9 is installed"},
		"base too old": {[]*App{testApp("Loyalty", "l", "1", Dependency{ID: "b", MinVersion: "0.2.0"})}, "needs Base version 0.2.0 or later"},
		"renamed":      {[]*App{rewards, testApp("Points", "l", "2.0.0", onBase)}, "depends on Loyalty (id l), but the app with that id is Points"},
		"circle": {[]*App{
			testApp("Aa", "a", "1", onBase, Dependency{ID: "c"}),
			testApp("Cc", "c", "1", onBase, Dependency{ID: "a"}),
		}, "in a circle: Aa → Cc → Aa"},
	} {
		if _, err := Order(base, c.apps); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", name, err, c.want)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"0.1.160", "0.1.97", 1}, {"1.0", "1.0.0", 0}, {"v2.0.1", "2.1.0", -1}, {"1.2.3-beta", "1.2.3", 0},
	} {
		if got := CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("CompareVersions(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// An extension app needs a prefix and a dependency on the base application; its names start
// with the prefix
func TestCheckExtension(t *testing.T) {
	base := testApp("Base", "b", "1")
	app := &App{Name: "Loyalty", Prefix: "lp", Dependencies: []Dependency{{ID: "b"}}}
	if err := app.CheckExtension(base); err != nil {
		t.Error(err)
	}
	for _, p := range []string{"", "L", "1lp", "loyaltypoint", "l-p"} {
		bad := &App{Name: "Loyalty", Prefix: p, Dependencies: app.Dependencies}
		if err := bad.CheckExtension(base); err == nil {
			t.Errorf("prefix %q accepted", p)
		}
	}
	if err := (&App{Name: "Loyalty", Prefix: "lp"}).CheckExtension(base); err == nil || !strings.Contains(err.Error(), "depends on Base") {
		t.Errorf("without the base: %v", err)
	}
	for name, want := range map[string]bool{"lp_points": true, "LP_Loyalty_Tier": true, "LP Loyalty Tiers": true, "lpoints": false, "lp_": false, "points": false} {
		if got := app.HasPrefix(name); got != want {
			t.Errorf("HasPrefix(%q) = %v", name, got)
		}
	}
}
