package reports

import (
	"strings"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The fixed asset printouts (reports 5603–5606) start each page with the classic report head:
// the report's title, the company name, today, the page and the user (setReportHeadValues), and
// on the first page the filters of the data item (filter_text).

// setReportHeadValues: company_name, today and user_id for a list printout's head
func setReportHeadValues(run *Run) {
	companyInfo := gtables.GetCompanyInformation(run)
	run.SetValue("company_name", companyInfo.Name.String())
	run.SetValue("today", system.Today())
	run.SetValue("user_id", system.UserId())
}

// filteredRecord: a data item's record with its filters (GETFILTERS) and captions
type filteredRecord interface {
	GetFilters() string
	TableCaption(language ...string) string
	GetFieldCaption(field, language string) string
}

// setFilterText (TABLENAME + ': ' + GETFILTERS): the data item's filters with the fields'
// captions as filter_text, blank without filters
func setFilterText(run *Run, rec filteredRecord) {
	filters := rec.GetFilters()
	if filters == "" {
		run.SetValue("filter_text", "")
		return
	}
	language := fcodeunits.CurrentLanguage()
	var parts []string
	for _, part := range strings.Split(filters, ", ") {
		field, expression, ok := strings.Cut(part, ": ")
		if ok {
			part = rec.GetFieldCaption(field, language) + ": " + expression
		}
		parts = append(parts, part)
	}
	run.SetValue("filter_text", rec.TableCaption(language)+": "+strings.Join(parts, ", "))
}

// faReportDeprBook: the Depreciation Book option (the FA Setup's default book first) and the
// head's "Depreciation Book: X" text
func faReportDeprBook(run *Run, option string) (*gtables.DepreciationBook, error) {
	deprBook := gtables.NewDepreciationBook(run)
	err := deprBook.GetOrError(types.Code(system.UpperCase(run.Option(option))))
	if err != nil {
		return nil, err
	}
	run.SetValue("depr_book_text", deprBook.TableCaption(fcodeunits.CurrentLanguage())+": "+deprBook.Code.String())
	return deprBook, nil
}

// defaultDeprBook (OnOpenForm): the FA Setup's default depreciation book as the option
func defaultDeprBook(run *Run, option string) error {
	faSetup := gtables.GetFASetup(run)
	if run.Option(option) == "" {
		run.SetOption(option, faSetup.Default_depr_book.String())
	}
	return nil
}
