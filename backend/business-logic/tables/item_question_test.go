package tables

import (
	"errors"
	"testing"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// A question in a table trigger, answered in rounds (classic CONFIRM in Item Standard Cost): the
// user's change of a standard cost item's Standard Cost asks first — the first round records the
// question, answers No and refuses the commit; with Yes the cost is kept and the unit cost
// follows, with No the old cost stays; code (CurrFieldNo 0) is not asked
func TestItemStandardCostQuestion(t *testing.T) {
	db := newTestDB(t)
	createTables(t, db, &gtables.Item{}, &gtables.GeneralLedgerSetup{}, &gtables.InventorySetup{})
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	item := gtables.NewItem(src)
	item.InitRecord()
	item.No, item.Costing_method, item.Standard_cost, item.Unit_cost = "I1", gtables.Item_Costing_method.Standard, types.NewDecimalFromInt(5), types.NewDecimalFromInt(5)
	must(t, item.InsertOrError(false))

	changeByUser := func(answers []bool) (*gtables.Item, string) {
		t.Helper()
		fcodeunits.StartQuestions(answers)
		defer fcodeunits.TakeQuestion()
		fcodeunits.SetCurrFieldNo(item.FieldNo("standard_cost"))
		defer fcodeunits.SetCurrFieldNo(0)
		rec := gtables.NewItem(src)
		must(t, rec.GetOrError(types.Code("I1")))
		must(t, rec.Validate("standard_cost", types.NewDecimalFromInt(8)))
		return rec, fcodeunits.PendingQuestion()
	}

	// first round: asked, No for now, nothing may be committed
	rec, question := changeByUser(nil)
	if question == "" || !rec.Standard_cost.Equal(types.NewDecimalFromInt(5)) {
		t.Fatalf("first round: question %q, standard cost %s", question, rec.Standard_cost)
	}
	fcodeunits.StartQuestions(nil)
	fcodeunits.SetCurrFieldNo(item.FieldNo("standard_cost"))
	tx, err := ftables.BeginTransaction(db)
	must(t, err)
	txItem := gtables.NewItem(ftables.DBSource(tx, "c1", database.DBTypeSQLite))
	must(t, txItem.GetOrError(types.Code("I1")))
	must(t, txItem.Validate("standard_cost", types.NewDecimalFromInt(8)))
	must(t, txItem.ModifyOrError(true))
	var questionErr *fcodeunits.QuestionError
	if err := ftables.Commit(tx); !errors.As(err, &questionErr) {
		t.Errorf("commit with a question not answered: %v", err)
	}
	fcodeunits.SetCurrFieldNo(0)
	fcodeunits.TakeQuestion()

	// Yes: the cost is kept, the unit cost follows
	rec, question = changeByUser([]bool{true})
	if question != "" || !rec.Standard_cost.Equal(types.NewDecimalFromInt(8)) || !rec.Unit_cost.Equal(types.NewDecimalFromInt(8)) {
		t.Errorf("Yes: question %q, standard cost %s, unit cost %s", question, rec.Standard_cost, rec.Unit_cost)
	}
	// No: the old cost stays
	rec, question = changeByUser([]bool{false})
	if question != "" || !rec.Standard_cost.Equal(types.NewDecimalFromInt(5)) {
		t.Errorf("No: question %q, standard cost %s", question, rec.Standard_cost)
	}
	// code: not asked
	rec = gtables.NewItem(src)
	must(t, rec.GetOrError(types.Code("I1")))
	must(t, rec.Validate("standard_cost", types.NewDecimalFromInt(9)))
	if !rec.Standard_cost.Equal(types.NewDecimalFromInt(9)) {
		t.Errorf("from code: standard cost %s", rec.Standard_cost)
	}
}
