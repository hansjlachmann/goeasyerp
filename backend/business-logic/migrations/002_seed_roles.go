package migrations

import (
	"fmt"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration002SeedRoles{})
}

// Migration002SeedRoles seeds default User_Role and Permission records
type Migration002SeedRoles struct{}

func (m *Migration002SeedRoles) Version() int {
	return 2
}

func (m *Migration002SeedRoles) Name() string {
	return "seed_default_roles"
}

func (m *Migration002SeedRoles) Description() string {
	return "Seed SUPER and READER default roles with permissions"
}

func (m *Migration002SeedRoles) Up(ctx *fmigrations.Context) error {
	// Ensure tables exist (migrations run before table sync)
	err := m.ensureTables(ctx)
	if err != nil {
		return err
	}

	// Insert SUPER role (bypasses all permission checks via IsSuper flag)
	err = m.insertRole(ctx, "SUPER", "Full access - bypasses all permission checks")
	if err != nil {
		return fmt.Errorf("failed to insert SUPER role: %w", err)
	}

	// Insert READER role (read-only access to all tables)
	err = m.insertRole(ctx, "READER", "Read-only access to all tables")
	if err != nil {
		return fmt.Errorf("failed to insert READER role: %w", err)
	}

	// READER gets read permission on all business tables
	readerTables := []string{
		"Customer",
		"Customer_ledger_entry",
		"Payment_terms",
		"Job_Queue",
		"Job_Queue_Entry",
		"User",
		"User_Role",
		"User_Member",
		"Permission",
		"Language",
		"Menu",
		"Company",
	}

	for _, table := range readerTables {
		err := m.insertPermission(ctx, "READER", table, true, false, false, false)
		if err != nil {
			return fmt.Errorf("failed to insert READER permission for %s: %w", table, err)
		}
	}

	return nil
}

func (m *Migration002SeedRoles) insertRole(ctx *fmigrations.Context, code, description string) error {
	var query string
	if ctx.DBType == database.DBTypePostgres {
		query = `INSERT INTO "User_Role" (code, description) VALUES ($1, $2) ON CONFLICT (code) DO NOTHING`
	} else {
		query = `INSERT OR IGNORE INTO "User_Role" (code, description) VALUES (?, ?)`
	}
	return ctx.ExecuteSQL(query, code, description)
}

func (m *Migration002SeedRoles) insertPermission(ctx *fmigrations.Context, roleID, tableName string, canRead, canInsert, canModify, canDelete bool) error {
	// Uppercase Code fields to match types.Code behavior (prevents case-sensitive mismatch in PostgreSQL)
	roleID = strings.ToUpper(roleID)
	tableName = strings.ToUpper(tableName)
	var query string
	if ctx.DBType == database.DBTypePostgres {
		query = `INSERT INTO "Permission" (role_id, table_name, can_read, can_insert, can_modify, can_delete) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (role_id, table_name) DO NOTHING`
	} else {
		query = `INSERT OR IGNORE INTO "Permission" (role_id, table_name, can_read, can_insert, can_modify, can_delete) VALUES (?, ?, ?, ?, ?, ?)`
	}
	return ctx.ExecuteSQL(query, roleID, tableName, canRead, canInsert, canModify, canDelete)
}

func (m *Migration002SeedRoles) ensureTables(ctx *fmigrations.Context) error {
	userRoleSQL := `CREATE TABLE IF NOT EXISTS "User_Role" (
		code VARCHAR(20) NOT NULL DEFAULT '',
		description VARCHAR(50) NOT NULL DEFAULT '',
		PRIMARY KEY (code)
	)`
	err := ctx.ExecuteSQL(userRoleSQL)
	if err != nil {
		return fmt.Errorf("failed to ensure User_Role table: %w", err)
	}

	userMemberSQL := `CREATE TABLE IF NOT EXISTS "User_Member" (
		user_id VARCHAR(50) NOT NULL DEFAULT '',
		role_id VARCHAR(20) NOT NULL DEFAULT '',
		company VARCHAR(100) NOT NULL DEFAULT '',
		PRIMARY KEY (user_id, role_id, company)
	)`
	err = ctx.ExecuteSQL(userMemberSQL)
	if err != nil {
		return fmt.Errorf("failed to ensure User_Member table: %w", err)
	}

	permissionSQL := `CREATE TABLE IF NOT EXISTS "Permission" (
		role_id VARCHAR(20) NOT NULL DEFAULT '',
		table_name VARCHAR(100) NOT NULL DEFAULT '',
		can_read BOOLEAN NOT NULL DEFAULT FALSE,
		can_insert BOOLEAN NOT NULL DEFAULT FALSE,
		can_modify BOOLEAN NOT NULL DEFAULT FALSE,
		can_delete BOOLEAN NOT NULL DEFAULT FALSE,
		PRIMARY KEY (role_id, table_name)
	)`
	err = ctx.ExecuteSQL(permissionSQL)
	if err != nil {
		return fmt.Errorf("failed to ensure Permission table: %w", err)
	}

	return nil
}
