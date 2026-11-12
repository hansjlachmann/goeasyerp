package handlers

import (
	"database/sql"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"

	"github.com/gofiber/fiber/v2"
	apitypes "github.com/hansjlachmann/goeasyerp/backend/api/types"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/codeunits"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
)

// CodeunitsHandler handles codeunit-related API requests
type CodeunitsHandler struct {
	db     *sql.DB
	dbType database.DBType
}

// NewCodeunitsHandler creates a new codeunits handler
func NewCodeunitsHandler(db *sql.DB) *CodeunitsHandler {
	return NewCodeunitsHandlerWithDBType(db, database.DBTypeSQLite)
}

// NewCodeunitsHandlerWithDBType creates a new codeunits handler with explicit database type
func NewCodeunitsHandlerWithDBType(db *sql.DB, dbType database.DBType) *CodeunitsHandler {
	return &CodeunitsHandler{db: db, dbType: dbType}
}

// RunCodeunit executes a codeunit by ID with the provided record
// POST /api/codeunits/run
func (h *CodeunitsHandler) RunCodeunit(c *fiber.Ctx) error {
	sess := getSession(c)

	if sess == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}

	company := sess.GetCompany()
	language := sess.GetLanguage()

	// Set goroutine context for sync codeunit execution
	fcodeunits.SetCurrentContext(sess.GetUserID(), sess.GetUserName(), company, language)
	fcodeunits.SetCurrentAccess(sess)
	defer fcodeunits.ClearCurrentContext()

	// Parse request body
	var req struct {
		CodeunitID int                    `json:"codeunit_id"`
		Record     map[string]interface{} `json:"record"`
		// The records selected on the list (rec.SetSelectionFilter in the codeunit)
		Selection *listSelection `json:"selection"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
	}

	if req.CodeunitID == 0 {
		return c.Status(400).JSON(apitypes.NewErrorResponse("Routine ID is required"))
	}

	// Get codeunit factory from registry
	factory, ok := codeunits.Get(req.CodeunitID)
	if !ok {
		return c.Status(404).JSON(apitypes.NewErrorResponse("Routine not found"))
	}
	// Execute permission on the codeunit (classic object permissions)
	if !sess.CanExecute(session.ObjectCodeunit, req.CodeunitID) {
		return c.Status(fiber.StatusForbidden).JSON(apitypes.NewErrorResponse(
			apperrors.NoPermissionCodeunit(req.CodeunitID, codeunits.List()[req.CodeunitID]).Message(language)))
	}

	// Create codeunit instance
	codeunit := factory(h.db, company, h.dbType)

	// The source table's record from the request (typed); an object without a source table
	// (a report without data items) runs without a record
	tableName := codeunit.SourceTable()
	var table ftables.Table
	if tableName != "" {
		tableFactory, ok := gtables.GetTableFactory(tableName)
		if !ok {
			return c.Status(500).JSON(apitypes.NewErrorResponse("Source table not found: " + tableName))
		}

		// Create table instance and populate from request
		table = tableFactory()
		table.InitWithDBType(h.db, company, h.dbType)

		// Populate table fields from the record map
		table.FromMap(req.Record)
		newTable := func() ftables.Table {
			t := tableFactory()
			t.InitWithDBType(h.db, company, h.dbType)
			return t
		}
		if err := applySelection(c, table, req.Selection, newTable, language); err != nil {
			return c.Status(400).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
		}
	}

	// Run the codeunit with the typed record
	result, err := codeunit.Run(table)
	if err != nil {
		return c.Status(500).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
	}

	response := map[string]interface{}{
		"success": result.Success,
		"message": result.Message,
		"data":    result.Data,
	}
	if result.Dialog != nil {
		response["dialog"] = result.Dialog
	}
	return c.JSON(apitypes.NewSuccessResponse(response))
}
