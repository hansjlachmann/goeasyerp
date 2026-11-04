package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration029CompanyInformation{})
}

// Migration029CompanyInformation goes with table 79 Company Information (the company's details
// printed on its documents). The READER role may read it, as all business tables; the table
// comes from the table sync.
type Migration029CompanyInformation struct{}

func (m *Migration029CompanyInformation) Version() int {
	return 29
}

func (m *Migration029CompanyInformation) Name() string {
	return "company-information"
}

func (m *Migration029CompanyInformation) Description() string {
	return "READER reads the Company Information"
}

func (m *Migration029CompanyInformation) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"COMPANY_INFORMATION"})
}
