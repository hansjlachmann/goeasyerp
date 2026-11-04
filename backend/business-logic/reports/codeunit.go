package reports

import (
	"encoding/base64"
	"strings"

	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
)

// CodeunitFactory runs a report like a codeunit, so pages (run_object: report:<id>) start it
// through the job API with a dialog: the request page, then the report; its message is shown
// at the end
func CodeunitFactory(id int) (fcodeunits.Factory, bool) {
	_, ok := Get(id)
	if !ok {
		return nil, false
	}
	return func(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
		return &reportCodeunit{id: id, db: db, company: company, dbType: dbType}
	}, true
}

type reportCodeunit struct {
	id      int
	db      database.Executor
	company string
	dbType  database.DBType
}

func (c *reportCodeunit) ID() int            { return c.id }
func (c *reportCodeunit) Name() string       { return List()[c.id] }
func (c *reportCodeunit) UsesProgress() bool { return true }

// SourceTable: the table of the first data item (the API makes a record of it)
func (c *reportCodeunit) SourceTable() string {
	factory, _ := Get(c.id)
	def := factory().Definition()
	if def.ContextTable != "" {
		return def.ContextTable
	}
	if len(def.DataItems) > 0 {
		return def.DataItems[0].Table
	}
	return ""
}

func (c *reportCodeunit) Run(record interface{}) (fcodeunits.Result, error) {
	opts := RunOptions{RequestPage: true}
	factory, _ := Get(c.id)
	if factory().Definition().ContextTable != "" {
		opts.Context = record // the page's record (with its filters) the report works on
	}
	result, err := Execute(c.id, c.db, c.company, c.dbType, opts)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return CodeunitResult(result), nil
}

// CodeunitResult is a report's result as a codeunit's (run from a page): a printout's PDF, a
// file to download, its message, the page it opens and the printouts it made
func CodeunitResult(result Result) fcodeunits.Result {
	if result.PDF != nil {
		// A printout: the frontend shows, prints or downloads it (the output chosen)
		return fcodeunits.Result{
			Success: true,
			Message: result.Message,
			Data: map[string]interface{}{
				"pdf":      base64.StdEncoding.EncodeToString(result.PDF),
				"filename": result.FileName,
				"output":   result.Output,
			},
		}
	}
	if result.File != nil {
		// a file to send: the browser downloads it (after the report's message)
		res := fcodeunits.Result{Success: true}
		if result.Message != "" {
			res = fcodeunits.Message(result.Message)
		}
		if res.Data == nil {
			res.Data = map[string]interface{}{}
		}
		res.Data["file"] = base64.StdEncoding.EncodeToString(result.File)
		res.Data["filename"] = result.FileName
		return res
	}
	res := fcodeunits.Result{Success: true}
	if result.Message != "" {
		res = fcodeunits.Message(result.Message)
	}
	res.OpenPage = result.OpenPage
	res.Refresh = result.Refresh
	return WithPrintouts(res, result.Printouts)
}

// ParseJobParameter reads a job's Parameter for a report it runs: name=value pairs separated
// by ";" or line breaks — an option of the request page (option=value) or a data item's filter
// (<DataItem>.<field>=<filter expression>)
func ParseJobParameter(parameter string) (map[string]string, map[string][]ftables.FieldFilter) {
	options := map[string]string{}
	filters := map[string][]ftables.FieldFilter{}
	for _, part := range strings.FieldsFunc(parameter, func(r rune) bool { return r == ';' || r == '\n' || r == '\r' }) {
		name, value, ok := strings.Cut(part, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			continue
		}
		value = strings.TrimSpace(value)
		item, field, isFilter := strings.Cut(name, ".")
		if isFilter {
			filters[item] = append(filters[item], ftables.FieldFilter{Field: field, Expression: value})
			continue
		}
		options[name] = value
	}
	return options, filters
}
