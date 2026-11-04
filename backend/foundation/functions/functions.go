// Package functions holds the functions pages can call with their current record: page
// actions and menu items with run_object: <type>:<object>.<function>, e.g.
//
//	run_object: table:Job_Queue_Entry.ShowLineNo
//	run_object: codeunit:900.PostLine
//
// Business logic registers each callable function explicitly (only registered functions can be
// called through the API), with the record type it takes:
//
//	functions.Register("table", gtables.JobQueueEntryTableName, "ShowLineNo",
//		functions.For(jobQueueEntryLogic{}.ShowLineNo))
//
//	func (jobQueueEntryLogic) ShowLineNo(rec *gtables.JobQueueEntry) (codeunits.Result, error)
//
// The API reads the record from the database by the key the page sends and calls the function
// like a codeunit; its Result (a message, an error) is shown as a codeunit's.
package functions

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
)

// Func is a registered function: it gets the page's current record (a generated record) and
// the arguments of the call; Params are the types of the arguments it takes
type Func struct {
	Params []string // "int", "decimal", "bool", "text" — in order
	call   func(rec interface{}, args Args) (codeunits.Result, error)
}

// For makes a Func of a function on a typed record (*gtables.<Table>) without arguments
func For[T any](fn func(rec T) (codeunits.Result, error)) Func {
	return ForArgs(func(rec T, _ Args) (codeunits.Result, error) { return fn(rec) })
}

// ForArgs makes a Func of a function on a typed record with arguments of the given types
// ("int", "decimal", "bool", "text"): run_object: table:Sales_Line.ItemAvailability(0) calls it
// with Args{0}. An option argument is an int (its number).
func ForArgs[T any](fn func(rec T, args Args) (codeunits.Result, error), params ...string) Func {
	return Func{Params: params, call: func(rec interface{}, args Args) (codeunits.Result, error) {
		r, ok := rec.(T)
		if !ok {
			var want T
			return codeunits.Result{}, fmt.Errorf("the function takes %T, the page's record is %T", want, rec)
		}
		return fn(r, args)
	}}
}

// ForAssistEdit makes a Func of an AssistEdit function (a field's "…" button, classic
// OnAssistEdit): it changes the page's record and tells whether it did; the page takes the
// changed record back (the result's Data["record"]). Register it WithAssistEdit.
func ForAssistEdit[T any](fn func(rec T) (bool, error)) Func {
	return For(func(rec T) (codeunits.Result, error) {
		changed, err := fn(rec)
		if err != nil || !changed {
			return codeunits.Result{Success: err == nil}, err
		}
		result := codeunits.Result{Success: true}
		if table, ok := any(rec).(ftables.Table); ok {
			result.Data = map[string]interface{}{"record": ftables.PublicMap(table)}
		}
		return result, nil
	})
}

// Args are the arguments of a call (checked against the function's Params)
type Args []any

// Int returns argument i as a whole number (0 when missing)
func (a Args) Int(i int) int {
	if i < len(a) {
		if v, ok := a[i].(int64); ok {
			return int(v)
		}
	}
	return 0
}

// Decimal returns argument i as a number (0 when missing)
func (a Args) Decimal(i int) float64 {
	if i < len(a) {
		switch v := a[i].(type) {
		case float64:
			return v
		case int64:
			return float64(v)
		}
	}
	return 0
}

// Bool returns argument i as true/false (false when missing)
func (a Args) Bool(i int) bool {
	if i < len(a) {
		if v, ok := a[i].(bool); ok {
			return v
		}
	}
	return false
}

// Text returns argument i as text ("" when missing)
func (a Args) Text(i int) string {
	if i < len(a) {
		if v, ok := a[i].(string); ok {
			return v
		}
	}
	return ""
}

// ParseCall splits a run_object call into the function (without arguments) and its arguments:
// "table:Sales_Line.ItemAvailability(0, true, \"x\", 1.5)" → "table:Sales_Line.ItemAvailability",
// [0 true "x" 1.5]. Arguments: whole numbers, decimals, true/false, quoted text.
func ParseCall(runObject string) (string, Args, error) {
	runObject = strings.TrimSpace(runObject)
	open := strings.Index(runObject, "(")
	if open < 0 {
		return runObject, nil, nil
	}
	if !strings.HasSuffix(runObject, ")") {
		return "", nil, fmt.Errorf("%q: missing )", runObject)
	}
	name, inner := runObject[:open], strings.TrimSpace(runObject[open+1:len(runObject)-1])
	var args Args
	for inner != "" {
		var raw string
		if inner[0] == '"' || inner[0] == '\'' {
			end := strings.IndexByte(inner[1:], inner[0])
			if end < 0 {
				return "", nil, fmt.Errorf("%q: unclosed text", runObject)
			}
			args = append(args, inner[1:end+1])
			inner = strings.TrimSpace(inner[end+2:])
		} else {
			comma := strings.IndexByte(inner, ',')
			if comma < 0 {
				raw, inner = inner, ""
			} else {
				raw, inner = inner[:comma], inner[comma:]
			}
			raw = strings.TrimSpace(raw)
			switch v := strings.ToLower(raw); {
			case v == "true" || v == "false":
				args = append(args, v == "true")
			default:
				if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
					args = append(args, n)
				} else if f, err := strconv.ParseFloat(raw, 64); err == nil {
					args = append(args, f)
				} else {
					return "", nil, fmt.Errorf("%q: %q is not a number, true/false or quoted text", runObject, raw)
				}
			}
		}
		inner = strings.TrimSpace(inner)
		if strings.HasPrefix(inner, ",") {
			inner = strings.TrimSpace(inner[1:])
			if inner == "" {
				return "", nil, fmt.Errorf("%q: an argument is missing after ,", runObject)
			}
		} else if inner != "" {
			return "", nil, fmt.Errorf("%q: expected , between the arguments", runObject)
		}
	}
	return strings.TrimSpace(name), args, nil
}

// argType is the parameter type an argument fits
func argType(a any) string {
	switch a.(type) {
	case int64:
		return "int"
	case float64:
		return "decimal"
	case bool:
		return "bool"
	default:
		return "text"
	}
}

// CheckArgs reports whether a call's arguments fit the function's parameters (a whole number
// also fits a decimal parameter)
func (f *Function) CheckArgs(args Args) error {
	if len(args) != len(f.Fn.Params) {
		return fmt.Errorf("%s takes %d argument(s), the call gives %d", f.Name, len(f.Fn.Params), len(args))
	}
	for i, p := range f.Fn.Params {
		got := argType(args[i])
		if got != p && !(p == "decimal" && got == "int") {
			return fmt.Errorf("%s: argument %d is %s, the function takes %s", f.Name, i+1, got, p)
		}
	}
	return nil
}

// Resolve finds the function of a run_object call and checks its arguments
func Resolve(runObject string) (*Function, Args, error) {
	name, args, err := ParseCall(runObject)
	if err != nil {
		return nil, nil, err
	}
	f, ok := Lookup(name)
	if !ok {
		return nil, nil, fmt.Errorf("function %s is not registered", name)
	}
	if err := f.CheckArgs(args); err != nil {
		return nil, nil, err
	}
	return f, args, nil
}

// Function is a registered function with the table of the record it takes
type Function struct {
	ObjectType string // "table" or "codeunit"
	Object     string // the table's name, or the codeunit's number
	Name       string
	Table      string // the record's table (for a table function: the table itself)
	// Dialog: run with a codeunit dialog (Confirm, StrMenu, progress) — async
	Dialog bool
	// AssistEdit: a field's "…" function — it gets the record as the page has it (also a new,
	// unsaved one), and runs with a dialog (it may open a page with RunModal)
	AssistEdit bool
	// Print: the function prints (Document-Print, PrintRecords) — it runs with a dialog for the
	// request pages, needs no Modify/Execute permission of its own (whoever reads the record may
	// print it) and checks Execute on each report it runs
	Print bool
	Fn    Func
}

// Option changes how a function is registered
type Option func(*Function)

// OnTable says which table's record a codeunit function takes (a table function takes its own)
func OnTable(table string) Option { return func(f *Function) { f.Table = table } }

// WithDialog runs the function with a codeunit dialog, so it can ask (Confirm, StrMenu) and
// show progress
func WithDialog() Option { return func(f *Function) { f.Dialog = true } }

// WithPrint registers a function that prints the record (see Function.Print)
func WithPrint() Option { return func(f *Function) { f.Print, f.Dialog = true, true } }

// WithAssistEdit registers a field's AssistEdit function (made with ForAssistEdit)
func WithAssistEdit() Option {
	return func(f *Function) { f.AssistEdit, f.Dialog = true, true }
}

var (
	mu       sync.RWMutex
	registry = map[string]*Function{}
)

func key(objectType, object, name string) string {
	return strings.ToLower(objectType + ":" + object + "." + name)
}

// Register adds a function of a table ("table", table name) or a codeunit ("codeunit", its
// number as text; give the record's table with OnTable)
func Register(objectType, object, name string, fn Func, opts ...Option) {
	f := &Function{ObjectType: strings.ToLower(objectType), Object: object, Name: name, Fn: fn}
	if f.ObjectType == "table" {
		f.Table = object
	}
	for _, o := range opts {
		o(f)
	}
	mu.Lock()
	defer mu.Unlock()
	registry[key(f.ObjectType, object, name)] = f
}

// Lookup finds a function by its run_object text without arguments
// ("table:Job_Queue_Entry.ShowLineNo"; see Resolve for a call with arguments)
func Lookup(runObject string) (*Function, bool) {
	if i := strings.Index(runObject, "("); i >= 0 {
		runObject = runObject[:i]
	}
	objectType, rest, ok := strings.Cut(strings.TrimSpace(runObject), ":")
	if !ok {
		return nil, false
	}
	dot := strings.LastIndex(rest, ".")
	if dot <= 0 || dot == len(rest)-1 {
		return nil, false
	}
	mu.RLock()
	defer mu.RUnlock()
	f, ok := registry[key(objectType, rest[:dot], rest[dot+1:])]
	return f, ok
}

// All returns the registered functions
func All() []*Function {
	mu.RLock()
	defer mu.RUnlock()
	all := make([]*Function, 0, len(registry))
	for _, f := range registry {
		all = append(all, f)
	}
	return all
}

// Codeunit runs the function like a codeunit (the job API's way of running page objects) with
// the call's arguments: on the record of its table, async with a dialog when it was
// registered WithDialog
func (f *Function) Codeunit(args Args) codeunits.Codeunit { return functionCodeunit{f, args} }

type functionCodeunit struct {
	f    *Function
	args Args
}

func (c functionCodeunit) ID() int             { return 0 }
func (c functionCodeunit) Name() string        { return c.f.ObjectType + ":" + c.f.Object + "." + c.f.Name }
func (c functionCodeunit) SourceTable() string { return c.f.Table }
func (c functionCodeunit) UsesProgress() bool  { return c.f.Dialog }
func (c functionCodeunit) Run(record interface{}) (codeunits.Result, error) {
	return c.f.Fn.call(record, c.args)
}
