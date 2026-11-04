package tables

import (
	"fmt"
	"sort"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/company"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// companyLogic is the business logic of table 2000000006 Company
type companyLogic struct{}

func init() { gtables.RegisterCompanyLogic(companyLogic{}) }

// OnRename: the name prefixes every table of the company ("name$Customer"), so the
// company's data moves with it: its tables, indexes and sequences are renamed and its
// indexes and SIFT totals built for the new name, in the transaction of the Modify (the
// API's modifyInTransaction). The generated Modify then carries the name over to User
// Member (table relation).
func (companyLogic) OnRename(rec *gtables.Company) error {
	oldName := rec.XRec().Name.String()
	newName, err := company.NormalizeName(rec.Name.String())
	if err != nil {
		return err
	}
	rec.Name = types.NewText(newName)
	if newName == oldName {
		return nil
	}

	var existing gtables.Company
	existing.InitWithDBType(rec.GetDB(), "", rec.GetDBType())
	if existing.Get(types.NewText(newName)) {
		return apperrors.CompanyAlreadyExists(newName)
	}

	err = company.RenameObjects(rec.GetDB(), rec.GetDBType(), oldName, newName)
	if err != nil {
		return err
	}
	return syncCompanyKeys(rec.GetDB(), rec.GetDBType(), newName)
}

// syncCompanyKeys creates the indexes and SIFT totals of every company table of company
// (after a rename: SQLite indexes and all SIFT objects are built again for the new name).
func syncCompanyKeys(db database.Executor, dbType database.DBType, companyName string) error {
	existing, err := company.Tables(db, dbType, companyName)
	if err != nil {
		return err
	}
	has := map[string]bool{}
	for _, name := range existing {
		has[name] = true
	}
	done := map[string]bool{}
	names := gtables.ListTableNames()
	sort.Strings(names)
	for _, name := range names {
		factory, ok := gtables.GetTableFactory(name)
		if !ok {
			continue
		}
		table := factory()
		table.InitWithDBType(db, companyName, dbType)
		syncer, ok := table.(interface {
			SyncKeys(db database.Executor, company string, dbType database.DBType) error
			IsGlobal() bool
		})
		if !ok || syncer.IsGlobal() || done[table.GetTableName()] || !has[companyName+"$"+table.GetTableName()] {
			continue
		}
		done[table.GetTableName()] = true
		err := syncer.SyncKeys(db, companyName, dbType)
		if err != nil {
			return fmt.Errorf("%s: %w", table.GetTableName(), err)
		}
	}
	return nil
}

// CreateCompanyTables creates every company table of the application in a company (global
// tables are left out) — for tests and tools that need a whole company
func CreateCompanyTables(db database.Executor, companyName string, dbType database.DBType) error {
	names := gtables.ListTableNames()
	sort.Strings(names)
	done := map[string]bool{}
	for _, name := range names {
		factory, ok := gtables.GetTableFactory(name)
		if !ok {
			continue
		}
		table := factory()
		creator, ok := table.(interface {
			CreateTableWithDBType(db database.Executor, company string, dbType database.DBType) error
			IsGlobal() bool
		})
		if !ok || creator.IsGlobal() || done[table.GetTableName()] {
			continue
		}
		done[table.GetTableName()] = true
		err := creator.CreateTableWithDBType(db, companyName, dbType)
		if err != nil {
			return fmt.Errorf("%s: %w", table.GetTableName(), err)
		}
	}
	return nil
}
