package main

import "testing"

func TestStartupCompany(t *testing.T) {
	cases := []struct {
		name      string
		requested string
		existing  []string
		want      string
		create    bool
	}{
		{"empty database, nothing asked for", "", nil, defaultCompany, true},
		{"empty database, a company asked for", "acme", nil, "acme", true},
		{"the company asked for exists", "beta", []string{"alpha", "beta"}, "beta", false},
		{"the company asked for is missing", "gone", []string{"alpha", "beta"}, "alpha", false},
		{"nothing asked for, companies exist", "", []string{"alpha", "beta"}, "alpha", false},
	}
	for _, c := range cases {
		got, create := startupCompany(c.requested, c.existing)
		if got != c.want || create != c.create {
			t.Errorf("%s: startupCompany(%q, %v) = %q, %v; want %q, %v", c.name, c.requested, c.existing, got, create, c.want, c.create)
		}
	}
}
