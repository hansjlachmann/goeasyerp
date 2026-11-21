package migrations

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration060MarkDemoCompanies{})
}

// Migration060MarkDemoCompanies goes with "demo data replaces only a demo company's data": the
// demo data now marks its company (Company Demo Company), and a company with customers that is
// not marked is never emptied. Companies filled with demo data before are marked here, so the
// demo can still be created again in them: a company whose customer C00010 is the demo's
// Fjordlys Interiør AS.
type Migration060MarkDemoCompanies struct{}

// demoCustomerNo, demoCustomerName: the demo's first customer (demodata/data/customers.yaml)
const (
	demoCustomerNo   = "C00010"
	demoCustomerName = "Fjordlys Interiør AS"
)

func (m *Migration060MarkDemoCompanies) Version() int {
	return 60
}

func (m *Migration060MarkDemoCompanies) Name() string {
	return "mark_demo_companies"
}

func (m *Migration060MarkDemoCompanies) Description() string {
	return "Companies holding the demo data are marked as demo companies"
}

func (m *Migration060MarkDemoCompanies) Up(ctx *fmigrations.Context) error {
	exists, err := ctx.TableExists("Company")
	if err != nil || !exists {
		return err
	}
	// the column comes with the table sync, which runs after the migrations
	err = ctx.AddColumnIfNotExists("Company", "demo_company", "BOOLEAN DEFAULT FALSE")
	if err != nil {
		return err
	}
	p1, p2 := "?", "?"
	if ctx.DBType == database.DBTypePostgres {
		p1, p2 = "$1", "$2"
	}
	for _, company := range ctx.Companies {
		customers := company + "$Customer"
		exists, err := ctx.TableExists(customers)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		var n int
		err = ctx.Tx().QueryRow(`SELECT COUNT(*) FROM "`+customers+`" WHERE no = `+p1+` AND name = `+p2,
			demoCustomerNo, demoCustomerName).Scan(&n)
		if err != nil {
			return err
		}
		if n == 0 {
			continue
		}
		err = ctx.ExecuteSQL(`UPDATE "Company" SET demo_company = `+p1+` WHERE name = `+p2, true, company)
		if err != nil {
			return err
		}
	}
	return nil
}
