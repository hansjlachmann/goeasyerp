package session

import (
	"fmt"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
)

// LoadPermissions loads and merges a user's permissions in a company: the roles of the user's
// memberships (User_Member, the company or all companies), then the Permission rows of those
// roles, merged with OR across roles. It returns (hasRoles, isSuper, permissions): no roles means
// the permission system is not configured for the user; SUPER needs no rows.
func LoadPermissions(db database.Executor, dbType database.DBType, userID, company string) (bool, bool, map[string]TablePermission) {
	permissions := make(map[string]TablePermission)

	query := `SELECT role_id FROM "User_Member" WHERE user_id = ? AND (company = ? OR company = '')`
	if dbType == database.DBTypePostgres {
		query = `SELECT role_id FROM "User_Member" WHERE user_id = $1 AND (company = $2 OR company = '')`
	}
	rows, err := db.Query(query, userID, company)
	if err != nil {
		fmt.Printf("Warning: Failed to load user roles: %v\n", err)
		return false, false, permissions
	}
	var roleIDs []string
	isSuper := false
	for rows.Next() {
		var roleID string
		if rows.Scan(&roleID) != nil {
			continue
		}
		roleIDs = append(roleIDs, roleID)
		if strings.ToUpper(roleID) == "SUPER" {
			isSuper = true
		}
	}
	_ = rows.Close()

	if len(roleIDs) == 0 {
		return false, false, permissions
	}
	if isSuper {
		return true, true, nil
	}

	placeholders := make([]string, len(roleIDs))
	args := make([]interface{}, len(roleIDs))
	for i, id := range roleIDs {
		placeholders[i] = "?"
		if dbType == database.DBTypePostgres {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}
		args[i] = id
	}
	// can_execute is NULL on rows from before execute permissions existed
	permRows, err := db.Query(fmt.Sprintf(
		`SELECT table_name, can_read, can_insert, can_modify, can_delete, COALESCE(can_execute, %s) FROM "Permission" WHERE role_id IN (%s)`,
		falseLiteral(dbType), strings.Join(placeholders, ", ")), args...)
	if err != nil {
		fmt.Printf("Warning: Failed to load permissions: %v\n", err)
		return true, false, permissions
	}
	defer func() { _ = permRows.Close() }()
	for permRows.Next() {
		var key string
		var canRead, canInsert, canModify, canDelete, canExecute bool
		if permRows.Scan(&key, &canRead, &canInsert, &canModify, &canDelete, &canExecute) != nil {
			continue
		}
		key = strings.ToUpper(key)
		p := permissions[key]
		p.Read = p.Read || canRead
		p.Insert = p.Insert || canInsert
		p.Modify = p.Modify || canModify
		p.Delete = p.Delete || canDelete
		p.Execute = p.Execute || canExecute
		permissions[key] = p
	}
	return true, false, permissions
}

func falseLiteral(dbType database.DBType) string {
	if dbType == database.DBTypePostgres {
		return "false"
	}
	return "0"
}
