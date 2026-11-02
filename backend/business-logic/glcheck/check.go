// Package glcheck checks the whole general ledger: every posting date and every transaction of
// the G/L entries adds up to 0, the entry numbers have no gaps (up to the last date compression:
// it deletes entries), and the G/L registers cover the entries once each. The balance check of the posting (Consistent) keeps a posting from saving
// G/L entries that do not balance; this check finds what reached the table some other way
// (direct database changes, imports, restores, a fault in posting code).
package glcheck

import (
	"sort"

	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// DateTotal: the G/L entries of a posting date that do not add up to 0
type DateTotal struct {
	PostingDate types.Date
	Amount      types.Decimal
}

// TransactionTotal: the G/L entries of a transaction that do not add up to 0 (its first
// entry's posting date and document)
type TransactionTotal struct {
	TransactionNo int
	PostingDate   types.Date
	DocumentNo    types.Code
	Amount        types.Decimal
}

// Gap: entry numbers that are missing (From..To)
type Gap struct {
	From, To int
}

// RegisterProblem: a G/L register whose entry range is wrong
type RegisterProblem struct {
	RegisterNo int
	From, To   int
	Problem    Problem
	OtherNo    int // the other register (Overlaps)
}

// Problem: what is wrong with a register's range
type Problem int

const (
	// Overlaps: the range overlaps another register's
	Overlaps Problem = iota + 1
	// Reversed: From Entry No. is after To Entry No.
	Reversed
	// EntriesMissing: entries of the range do not exist
	EntriesMissing
	// NotRegistered: entries no register covers (From..To)
	NotRegistered
)

// Result is what the check found
type Result struct {
	Entries      int
	Registers    int
	Dates        []DateTotal
	Transactions []TransactionTotal
	Gaps         []Gap
	RegisterList []RegisterProblem
}

// Balanced: nothing was found
func (r *Result) Balanced() bool {
	return len(r.Dates) == 0 && len(r.Transactions) == 0 && len(r.Gaps) == 0 && len(r.RegisterList) == 0
}

// Check reads every G/L entry and register of the company; progress (may be nil) is told the
// share done (0-100) now and then
func Check(src ftables.Source, progress func(percent int)) *Result {
	result := &Result{}
	dateTotals := map[string]*DateTotal{} // by the date's text (closing dates are dates of their own)
	var dateOrder []string
	transactions := map[int]*TransactionTotal{}
	var transactionOrder []int
	var entryNos []int

	glEntry := gtables.NewGLEntry(src)
	total := glEntry.Count()
	glEntry.SetCurrentKey("entry_no")
	for found := glEntry.FindSetBuffered(); found; found = glEntry.Next() {
		result.Entries++
		if progress != nil && total > 0 && result.Entries%1000 == 0 {
			progress(result.Entries * 90 / total)
		}
		entryNos = append(entryNos, glEntry.Entry_no)
		dateTotal, ok := dateTotals[glEntry.Posting_date.String()]
		if !ok {
			dateTotal = &DateTotal{PostingDate: glEntry.Posting_date}
			dateTotals[glEntry.Posting_date.String()] = dateTotal
			dateOrder = append(dateOrder, glEntry.Posting_date.String())
		}
		dateTotal.Amount = dateTotal.Amount.Add(glEntry.Amount)
		transaction, ok := transactions[glEntry.Transaction_no]
		if !ok {
			transaction = &TransactionTotal{TransactionNo: glEntry.Transaction_no, PostingDate: glEntry.Posting_date, DocumentNo: glEntry.Document_no}
			transactions[glEntry.Transaction_no] = transaction
			transactionOrder = append(transactionOrder, glEntry.Transaction_no)
		}
		transaction.Amount = transaction.Amount.Add(glEntry.Amount)
	}

	for _, date := range dateOrder {
		dateTotal := dateTotals[date]
		if !dateTotal.Amount.IsZero() {
			result.Dates = append(result.Dates, *dateTotal)
		}
	}
	sort.Slice(result.Dates, func(i, j int) bool { return result.Dates[i].PostingDate.Before(result.Dates[j].PostingDate) })
	for _, no := range transactionOrder {
		transaction := transactions[no]
		if !transaction.Amount.IsZero() {
			result.Transactions = append(result.Transactions, *transaction)
		}
	}
	compressedUpTo := compressedEntryNos(src)
	result.Gaps = gaps(entryNos, compressedUpTo)
	result.RegisterList, result.Registers = checkRegisters(src, entryNos, compressedUpTo)
	if progress != nil {
		progress(100)
	}
	return result
}

// compressedEntryNos: the last entry number of the latest date compression of entries numbered
// with the G/L entries (G/L, tax, customer, vendor and bank account entries: the new entries'
// G/L register) — the entries up to it were compressed, their numbers are gone (0: none)
func compressedEntryNos(src ftables.Source) int {
	upTo := 0
	dateComprReg := gtables.NewDateComprRegister(src)
	dateComprReg.SetFilter("table_id", "17|21|25|254|271")
	for found := dateComprReg.FindSet(); found; found = dateComprReg.Next() {
		glReg := gtables.NewGLRegister(src)
		if glReg.Get(dateComprReg.Register_no) && glReg.To_entry_no > upTo {
			upTo = glReg.To_entry_no
		}
	}
	return upTo
}

// gaps: the entry numbers missing from 1 to the last entry (entryNos ascending), those up to
// compressedUpTo left out (date compression deleted them)
func gaps(entryNos []int, compressedUpTo int) []Gap {
	var result []Gap
	next := compressedUpTo + 1
	for _, no := range entryNos {
		if no > next {
			result = append(result, Gap{From: next, To: no - 1})
		}
		if no+1 > next {
			next = no + 1
		}
	}
	return result
}

// checkRegisters: registers whose range is reversed, overlaps another's or holds entry numbers
// that do not exist (up to compressedUpTo date compression deleted them), and the entries no
// register covers (entryNos ascending)
func checkRegisters(src ftables.Source, entryNos []int, compressedUpTo int) ([]RegisterProblem, int) {
	exists := make(map[int]bool, len(entryNos))
	for _, no := range entryNos {
		exists[no] = true
	}
	covered := make(map[int]bool, len(entryNos))
	type span struct{ no, from, to int }
	var spans []span
	var result []RegisterProblem
	count := 0

	glReg := gtables.NewGLRegister(src)
	for found := glReg.FindSet(); found; found = glReg.Next() {
		count++
		from, to := glReg.From_entry_no, glReg.To_entry_no
		if from == 0 && to == 0 {
			continue // a register without G/L entries
		}
		if from > to {
			result = append(result, RegisterProblem{RegisterNo: glReg.No, From: from, To: to, Problem: Reversed})
			continue
		}
		missing := false
		for no := from; no <= to; no++ {
			if !exists[no] && no > compressedUpTo {
				missing = true
			}
			covered[no] = true
		}
		if missing {
			result = append(result, RegisterProblem{RegisterNo: glReg.No, From: from, To: to, Problem: EntriesMissing})
		}
		spans = append(spans, span{glReg.No, from, to})
	}

	sort.Slice(spans, func(i, j int) bool { return spans[i].from < spans[j].from })
	for i := 1; i < len(spans); i++ {
		if spans[i].from <= spans[i-1].to {
			result = append(result, RegisterProblem{RegisterNo: spans[i].no, From: spans[i].from, To: spans[i].to, Problem: Overlaps, OtherNo: spans[i-1].no})
		}
	}

	var notRegistered []int
	for _, no := range entryNos {
		if !covered[no] {
			notRegistered = append(notRegistered, no)
		}
	}
	for i := 0; i < len(notRegistered); {
		j := i
		for j+1 < len(notRegistered) && notRegistered[j+1] == notRegistered[j]+1 {
			j++
		}
		result = append(result, RegisterProblem{From: notRegistered[i], To: notRegistered[j], Problem: NotRegistered})
		i = j + 1
	}
	return result, count
}
