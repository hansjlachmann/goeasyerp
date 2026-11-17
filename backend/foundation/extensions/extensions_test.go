package extensions

import "testing"

func TestInstall(t *testing.T) {
	var during string
	Install("app-1", func() { during = Current() })
	if during != "app-1" || Current() != "" {
		t.Errorf("current during Register %q, after %q", during, Current())
	}
	if got := Installed(); len(got) != 1 || got[0] != "app-1" {
		t.Errorf("installed %v", got)
	}
	defer func() {
		if recover() == nil {
			t.Error("an app installed twice was accepted")
		}
	}()
	Install("app-1", func() {})
}
