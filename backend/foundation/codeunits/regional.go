package codeunits

import "github.com/hansjlachmann/goeasyerp/backend/foundation/types"

// Regional are a user's regional settings (My Settings): the work date (blank: today) and how
// dates and numbers are shown — DateFormat "dd.mm.yy", "dd.mm.yyyy", "dd/mm/yyyy", "mm/dd/yyyy",
// "yyyy-mm-dd" and NumberFormat "1,234.56", "1.234,56", "1 234,56", "1234.56"; blank: the
// language's
type Regional struct {
	WorkDate     types.Date
	DateFormat   string
	NumberFormat string
}

// RegionalLoader reads a user's regional settings; set at startup (the API server), nil in
// tests and tools — then every user has the defaults (today, the language's formats)
var RegionalLoader func(userID string) Regional

// CurrentRegional returns the regional settings of the current goroutine's user
func CurrentRegional() Regional {
	ctx := GetCurrentContext()
	if ctx == nil {
		return Regional{}
	}
	return ctx.Regional
}

// CurrentWorkDate returns the current user's work date (blank: today)
func CurrentWorkDate() types.Date {
	return CurrentRegional().WorkDate
}

// SetCurrentWorkDate sets the work date of the current goroutine (blank: today) — a scheduled
// job runs on today
func SetCurrentWorkDate(workDate types.Date) {
	ctx := GetCurrentContext()
	if ctx == nil {
		ctx = &goroutineContext{}
		currentContexts.Store(getGoroutineID(), ctx)
	}
	ctx.Regional.WorkDate = workDate
}
