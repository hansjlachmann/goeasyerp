package consolidation

import (
	"sort"

	"github.com/hansjlachmann/openerp/backend/foundation/types"
)

// Missing is a consolidation account, dimension 1 or 2 that is not in the consolidated
// company, with the business unit's code that consolidates to it
type Missing struct {
	Code types.Code // the code in the consolidated company
	From types.Code // the business unit's account, dimension 1 or 2
}

// Check collects what a business unit consolidates to that is missing here (Consolidation -
// Test Database / Test File: AddGLAccError, AddDeptError, AddProjError) — each code once
type Check struct {
	accounts, dimension1, dimension2 map[types.Code]types.Code
}

// NewCheck is an empty check
func NewCheck() *Check {
	return &Check{accounts: map[types.Code]types.Code{}, dimension1: map[types.Code]types.Code{}, dimension2: map[types.Code]types.Code{}}
}

// Account notes a consolidation account that is missing
func (c *Check) Account(code, from types.Code) { add(c.accounts, code, from) }

// Dimension 1 notes a consolidation dimension 1 that is missing
func (c *Check) Dimension1(code, from types.Code) { add(c.dimension1, code, from) }

// Dimension 2 notes a consolidation dimension 2 that is missing
func (c *Check) Dimension2(code, from types.Code) { add(c.dimension2, code, from) }

func add(m map[types.Code]types.Code, code, from types.Code) {
	_, ok := m[code]
	if !ok {
		m[code] = from
	}
}

// Accounts are the missing accounts by code
func (c *Check) Accounts() []Missing { return sorted(c.accounts) }

// Dimension 1 values are the missing dimension 1 values by code
func (c *Check) Dimension1Values() []Missing { return sorted(c.dimension1) }

// Dimension 2 values are the missing dimension 2 values by code
func (c *Check) Dimension2Values() []Missing { return sorted(c.dimension2) }

func sorted(m map[types.Code]types.Code) []Missing {
	list := make([]Missing, 0, len(m))
	for code, from := range m {
		list = append(list, Missing{Code: code, From: from})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Code < list[j].Code })
	return list
}
