package genjnl

import (
	"strconv"

	"github.com/shopspring/decimal"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/format"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// balanceCheck holds the balance sums of the G/L entries posted in the transaction (two date
// weightings) and the range of their entry numbers. All postings in a transaction share them,
// so a posting that balances cannot hide an earlier one in the same transaction that does not.
type balanceCheck struct {
	amount      decimal.Decimal
	amount2     decimal.Decimal
	addCurr     decimal.Decimal // the additional-currency amounts, weighted the same way
	addCurr2    decimal.Decimal
	fromEntryNo int
	toEntryNo   int
}

// balance returns the balance sums of the posting's transaction; the first posting of a
// transaction registers how a commit refused by the balance check is explained
func (p *GenJnlPostLine) balance() *balanceCheck {
	if p.balanceCheck == nil {
		src := p.src
		state := ftables.TransactionState(src.GetDB(), "genjnl.balanceCheck", func() any {
			balance := &balanceCheck{}
			ftables.OnInconsistent(src.GetDB(), gtables.GLEntryTableName, func() error {
				return balance.explain(src)
			})
			return balance
		})
		p.balanceCheck = state.(*balanceCheck)
	}
	return p.balanceCheck
}

// add counts a G/L entry of the posting in the balance sums: its amount weighted by its posting
// date in two ways (the entries balance when both sums are 0 — per posting date, not only in
// total; a closing date counts as a date of its own); with an additional reporting currency
// (addCurr) its additional-currency amount as well
func (b *balanceCheck) add(glEntry *gtables.GLEntry, addCurr bool) {
	days := decimal.NewFromInt(int64(glEntry.Posting_date.Normal().Time.Unix() / 86400))
	if glEntry.Posting_date.IsClosing() {
		days = days.Add(decimal.NewFromInt(50))
	}
	b.amount = b.amount.Add(glEntry.Amount.Decimal.Mul(days.Mod(decimal.NewFromInt(99)).Add(decimal.NewFromInt(1))))
	b.amount2 = b.amount2.Add(glEntry.Amount.Decimal.Mul(days.Mod(decimal.NewFromInt(98)).Add(decimal.NewFromInt(1))))
	if addCurr {
		b.addCurr = b.addCurr.Add(glEntry.Additional_currency_amount.Decimal.Mul(days.Mod(decimal.NewFromInt(99)).Add(decimal.NewFromInt(1))))
		b.addCurr2 = b.addCurr2.Add(glEntry.Additional_currency_amount.Decimal.Mul(days.Mod(decimal.NewFromInt(98)).Add(decimal.NewFromInt(1))))
	}
	if b.fromEntryNo == 0 || glEntry.Entry_no < b.fromEntryNo {
		b.fromEntryNo = glEntry.Entry_no
	}
	if glEntry.Entry_no > b.toEntryNo {
		b.toEntryNo = glEntry.Entry_no
	}
}

// balanced: the entries posted so far balance on every posting date
func (b *balanceCheck) balanced() bool {
	return b.amount.IsZero() && b.amount2.IsZero() && b.addCurr.IsZero() && b.addCurr2.IsZero()
}

// docTotal: the sum of the transaction's G/L entries of a document on a posting date (and of
// their additional-currency amounts)
type docTotal struct {
	postingDate types.Date
	documentNo  types.Code
	amount      types.Decimal
	addCurr     types.Decimal
}

// explain is the error of a transaction whose G/L entries do not balance (run by the commit
// before the rollback): the first document and posting date that do not, by how much, with the
// transaction's G/L entries and their totals per date and document as the details
func (b *balanceCheck) explain(src ftables.Source) error {
	language := fcodeunits.CurrentLanguage()
	ts := i18n.GetInstance()
	caption := func(field string) string {
		return ts.FieldCaption(gtables.GLEntryTableName, field, language)
	}
	entries := apperrors.DetailsSection{
		Caption: ts.Message("BALANCE_CHECK_ENTRIES", language),
		Columns: []apperrors.DetailsColumn{
			{Caption: caption("entry_no"), Numeric: true},
			{Caption: caption("posting_date")},
			{Caption: caption("document_no")},
			{Caption: caption("gl_account_no")},
			{Caption: ts.FieldCaption(gtables.GLAccountTableName, "name", language)},
			{Caption: caption("description")},
			{Caption: caption("gen_posting_type")},
			{Caption: caption("amount"), Numeric: true},
		},
	}
	var totals []*docTotal
	totalOf := map[string]*docTotal{}
	accountNames := map[types.Code]string{}
	glEntry := gtables.NewGLEntry(src)
	glEntry.SetRange("entry_no", b.fromEntryNo, b.toEntryNo)
	for found := glEntry.FindSet(); found; found = glEntry.Next() {
		name, ok := accountNames[glEntry.Gl_account_no]
		if !ok {
			glAcc := gtables.NewGLAccount(src)
			if glAcc.Get(glEntry.Gl_account_no) {
				name = glAcc.Name.String()
			}
			accountNames[glEntry.Gl_account_no] = name
		}
		genPostingType := ""
		if glEntry.Gen_posting_type != 0 {
			genPostingType = ts.OptionCaption(gtables.GLEntryTableName, "gen_posting_type", i18n.Key(glEntry.Gen_posting_type.String()), language)
		}
		entries.Rows = append(entries.Rows, []string{
			strconv.Itoa(glEntry.Entry_no),
			format.Date(glEntry.Posting_date, language),
			glEntry.Document_no.String(),
			glEntry.Gl_account_no.String(),
			name,
			glEntry.Description.String(),
			genPostingType,
			format.Decimal(glEntry.Amount, "2:2", language),
		})
		key := glEntry.Posting_date.String() + "|" + glEntry.Document_no.String()
		total, ok := totalOf[key]
		if !ok {
			total = &docTotal{postingDate: glEntry.Posting_date, documentNo: glEntry.Document_no}
			totalOf[key] = total
			totals = append(totals, total)
		}
		total.amount = total.amount.Add(glEntry.Amount)
		total.addCurr = total.addCurr.Add(glEntry.Additional_currency_amount)
	}

	totalsSection := apperrors.DetailsSection{
		Caption: ts.Message("BALANCE_CHECK_TOTALS", language),
		Columns: []apperrors.DetailsColumn{
			{Caption: caption("posting_date")},
			{Caption: caption("document_no")},
			{Caption: ts.Message("BALANCE_CHECK_DIFFERENCE", language), Numeric: true},
		},
	}
	var first, firstAddCurr *docTotal
	for i, total := range totals {
		totalsSection.Rows = append(totalsSection.Rows, []string{
			format.Date(total.postingDate, language),
			total.documentNo.String(),
			format.Decimal(total.amount, "2:2", language),
		})
		if !total.amount.IsZero() || !total.addCurr.IsZero() {
			totalsSection.Marked = append(totalsSection.Marked, i)
		}
		if !total.amount.IsZero() && first == nil {
			first = total
		}
		if !total.addCurr.IsZero() && firstAddCurr == nil {
			firstAddCurr = total
		}
	}
	details := &apperrors.Details{
		Title:    ts.Message("BALANCE_CHECK_TITLE", language),
		Sections: []apperrors.DetailsSection{totalsSection, entries},
	}
	if first == nil && firstAddCurr != nil {
		// the LCY amounts balance, the additional-currency amounts do not
		return apperrors.GLNotBalanced(firstAddCurr.documentNo.String(), format.Date(firstAddCurr.postingDate, language),
			format.Decimal(firstAddCurr.addCurr, "2:2", language)+" ("+caption("additional_currency_amount")+")").WithDetails(details)
	}
	if first == nil {
		// every document balances on its date; the imbalance is elsewhere (not in the entries
		// read back): name the table
		return apperrors.Inconsistent(ts.TableCaption(gtables.GLEntryTableName, language)).WithDetails(details)
	}
	return apperrors.GLNotBalanced(first.documentNo.String(), format.Date(first.postingDate, language),
		format.Decimal(first.amount, "2:2", language)).WithDetails(details)
}
