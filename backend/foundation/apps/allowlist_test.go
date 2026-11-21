package apps

import "testing"

func TestRepositoryAllowed(t *testing.T) {
	list := "Acme, hansjlachmann/goeasyerp-test-app; https://github.com/other/tools/"
	for _, c := range []struct {
		owner, name string
		want        bool
	}{
		{"acme", "loyalty", true},
		{"ACME", "anything", true},
		{"hansjlachmann", "goeasyerp-test-app", true},
		{"hansjlachmann", "other-repo", false},
		{"other", "tools", true},
		{"other", "tools2", false},
		{"evil", "loyalty", false},
	} {
		if got := RepositoryAllowed(list, c.owner, c.name); got != c.want {
			t.Errorf("%s/%s: %v, want %v", c.owner, c.name, got, c.want)
		}
	}
	if RepositoryAllowed("", "acme", "loyalty") {
		t.Error("an empty list allows a repository")
	}
	for _, bad := range []string{"acme/a/b", "-acme", "acme/../x", "https://gitlab.com/acme"} {
		if _, err := ParseAllowedRepositories(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
