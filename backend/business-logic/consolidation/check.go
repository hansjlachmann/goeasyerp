package consolidation

import (
	"sort"

	"github.com/hansjlachmann/openerp/backend/foundation/types"
)

// Missing is a consolidation account, department or project that is not in the consolidated
// company, with the business unit's code that consolidates to it
type Missing struct {
	Code types.Code // the code in the consolidated company
	From types.Code // the business unit's account, department or project
}

// Check collects what a business unit consolidates to that is missing here (Consolidation -
// Test Database / Test File: AddGLAccError, AddDeptError, AddProjError) — each code once
type Check struct {
	accounts, departments, projects map[types.Code]types.Code
}

// NewCheck is an empty check
func NewCheck() *Check {
	return &Check{accounts: map[types.Code]types.Code{}, departments: map[types.Code]types.Code{}, projects: map[types.Code]types.Code{}}
}

// Account notes a consolidation account that is missing
func (c *Check) Account(code, from types.Code) { add(c.accounts, code, from) }

// Department notes a consolidation department that is missing
func (c *Check) Department(code, from types.Code) { add(c.departments, code, from) }

// Project notes a consolidation project that is missing
func (c *Check) Project(code, from types.Code) { add(c.projects, code, from) }

func add(m map[types.Code]types.Code, code, from types.Code) {
	_, ok := m[code]
	if !ok {
		m[code] = from
	}
}

// Accounts are the missing accounts by code
func (c *Check) Accounts() []Missing { return sorted(c.accounts) }

// Departments are the missing departments by code
func (c *Check) Departments() []Missing { return sorted(c.departments) }

// Projects are the missing projects by code
func (c *Check) Projects() []Missing { return sorted(c.projects) }

func sorted(m map[types.Code]types.Code) []Missing {
	list := make([]Missing, 0, len(m))
	for code, from := range m {
		list = append(list, Missing{Code: code, From: from})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Code < list[j].Code })
	return list
}
