package appaudit

// Record is what a build keeps of an app's audit (App Build: Audit Reports, JSON): the app's
// exact source and its reports — the audit of its source, and of known vulnerabilities when
// the app brings in modules of its own
type Record struct {
	AppID    string    `json:"app_id"`
	Name     string    `json:"name"`
	Version  string    `json:"version"`
	SourceID string    `json:"source_id"` // the Git commit or the package's SHA-256
	Reports  []*Report `json:"reports"`
}

// Passed: every report passed
func (r *Record) Passed() bool {
	for _, rep := range r.Reports {
		if !rep.Passed() {
			return false
		}
	}
	return true
}

// Text: the reports one after another
func (r *Record) Text() string {
	text := ""
	for _, rep := range r.Reports {
		text += rep.Text()
	}
	return text
}
