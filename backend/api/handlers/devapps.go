package handlers

import (
	"database/sql"
	"errors"
	"net"
	"strconv"

	"github.com/gofiber/fiber/v2"

	apitypes "github.com/hansjlachmann/goeasyerp/backend/api/types"
	blapps "github.com/hansjlachmann/goeasyerp/backend/business-logic/apps"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/appaudit"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/appbundle"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
	"github.com/hansjlachmann/goeasyerp/backend/schemas"
)

// The development endpoint (Extension App development): the GoEasyERP VS Code extension
// publishes an app under development with F5. Registered only on a server started for
// development (GOEASYERP_DEV_ENDPOINT=true, a database that does not look like production —
// business-logic/apps DevelopmentAllowed); elsewhere the routes do not exist.
//
//	POST   /api/dev/apps         the app's package (.zip) as the body → the build requested
//	GET    /api/dev/builds/:no   the build's status and log (from=<byte> for the log after that)
//	DELETE /api/dev/apps/:id     unpublish the app → the build requested
//	GET    /api/dev/schemas      the names of the YAML definitions' JSON schemas (no login)
//	GET    /api/dev/schemas/:name  one schema (table.schema.json …; no login)
//
// The caller logs in as for the pages (POST /api/auth/login, the session cookie); SUPER only.

// DevAppsHandler serves the development endpoint
type DevAppsHandler struct {
	db     *sql.DB
	dbType database.DBType
	remote bool // other machines may publish (GOEASYERP_DEV_ENDPOINT_REMOTE)
}

// NewDevAppsHandler makes the handler
func NewDevAppsHandler(db *sql.DB, dbType database.DBType, remote bool) *DevAppsHandler {
	return &DevAppsHandler{db: db, dbType: dbType, remote: remote}
}

// Register adds the routes to the /api group
func (h *DevAppsHandler) Register(api fiber.Router) {
	dev := api.Group("/dev", h.local)
	dev.Get("/schemas", h.SchemaNames)
	dev.Get("/schemas/:name", h.Schema)
	dev.Post("/apps", h.loggedIn, h.Publish)
	dev.Delete("/apps/:id", h.loggedIn, h.Unpublish)
	dev.Get("/builds/:no", h.loggedIn, h.super, h.Build)
}

// local: from this machine, unless remote publishing is allowed
func (h *DevAppsHandler) local(c *fiber.Ctx) error {
	if !h.remote && !localAddress(c.IP()) {
		language := getLanguageOrDefault(getSession(c))
		return c.Status(fiber.StatusForbidden).JSON(apitypes.NewErrorResponse(i18n.GetInstance().Message("ERR_DEV_ENDPOINT_LOCAL_ONLY", language)))
	}
	return c.Next()
}

// loggedIn: a logged-in user
func (h *DevAppsHandler) loggedIn(c *fiber.Ctx) error {
	sess := getSession(c)
	if sess == nil || sess.GetUserID() == "" {
		language := getLanguageOrDefault(sess)
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message(language)))
	}
	return c.Next()
}

// super: a SUPER user (publishing checks it in the business logic; a build's log is read here)
func (h *DevAppsHandler) super(c *fiber.Ctx) error {
	sess := getSession(c)
	if !sess.IsSuper {
		return c.Status(fiber.StatusForbidden).JSON(apitypes.NewErrorResponse(i18n.GetInstance().Message("ERR_APP_SUPER_ONLY", getLanguageOrDefault(sess))))
	}
	return c.Next()
}

// SchemaNames lists the JSON schemas of the YAML definitions (GET /api/dev/schemas)
func (h *DevAppsHandler) SchemaNames(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"success": true, "data": schemas.Names()})
}

// Schema gives one JSON schema (GET /api/dev/schemas/:name), as application/schema+json
func (h *DevAppsHandler) Schema(c *fiber.Ctx) error {
	data, ok := schemas.Get(c.Params("name"))
	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(apitypes.NewErrorResponse("no schema " + c.Params("name")))
	}
	c.Set(fiber.HeaderContentType, "application/schema+json")
	return c.Send(data)
}

// localAddress: the request comes from this machine — loopback, or a private address (the
// container network: the developer's localhost reaches the container through Docker's gateway;
// the development compose file publishes the port on 127.0.0.1 only)
func localAddress(address string) bool {
	ip := net.ParseIP(address)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate())
}

// Publish takes the app's package and requests its build (POST /api/dev/apps)
func (h *DevAppsHandler) Publish(c *fiber.Ctx) error {
	language := getLanguageOrDefault(getSession(c))
	data := c.Body()
	if len(data) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(i18n.GetInstance().Message("ERR_DEV_PACKAGE_EMPTY", language)))
	}
	pkg := append([]byte(nil), data...) // Fiber reuses the body's buffer
	return h.inTransaction(c, language, func(src *gtables.InstalledApp) (*blapps.DevPublish, error) {
		return blapps.PublishDevelopment(src, pkg)
	})
}

// Unpublish takes a published app out again (DELETE /api/dev/apps/:id)
func (h *DevAppsHandler) Unpublish(c *fiber.Ctx) error {
	language := getLanguageOrDefault(getSession(c))
	appID := c.Params("id")
	return h.inTransaction(c, language, func(src *gtables.InstalledApp) (*blapps.DevPublish, error) {
		return blapps.UnpublishDevelopment(src, appID)
	})
}

// inTransaction runs a publish in one transaction: what it records is committed together, or
// nothing is; an app audit refusal answers 422 with the findings
func (h *DevAppsHandler) inTransaction(c *fiber.Ctx, language string, run func(*gtables.InstalledApp) (*blapps.DevPublish, error)) error {
	tx, err := ftables.BeginTransaction(h.db)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(apitypes.NewErrorResponse(err.Error()))
	}
	result, err := run(gtables.NewInstalledApp(ftables.DBSource(tx, "", h.dbType)))
	if err != nil {
		_ = ftables.Rollback(tx)
		var audit *blapps.AuditError
		if errors.As(err, &audit) {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"success": false, "error": audit.Error(),
				"data": fiber.Map{"sha256": audit.SHA256, "findings": refusedFindings(audit.Report)},
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
	}
	err = ftables.Commit(tx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
	}
	return c.JSON(fiber.Map{"success": true, "data": result})
}

// refusedFindings: the findings of a report as VS Code shows them (file, line, rule …)
func refusedFindings(r *appaudit.Report) []appaudit.Finding {
	if r == nil {
		return nil
	}
	return r.Findings
}

// devLogMax: the most of a build's log one answer gives (the end; "from" asks for the rest)
const devLogMax = 256 << 10

// Build gives a build's status and its log (GET /api/dev/builds/:no?from=<byte>)
func (h *DevAppsHandler) Build(c *fiber.Ctx) error {
	language := getLanguageOrDefault(getSession(c))
	no, err := strconv.Atoi(c.Params("no"))
	if err != nil || no <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
	}
	build := gtables.NewAppBuild(ftables.DBSource(h.db, "", h.dbType))
	if !build.Get(no) {
		return c.Status(fiber.StatusNotFound).JSON(apitypes.NewErrorResponse(apperrors.TableRecordNotFound(gtables.AppBuildTableName, strconv.Itoa(no)).Message(language)))
	}
	from := c.QueryInt("from", 0)
	logText := build.Log
	if from < 0 || from > len(logText) {
		from = len(logText)
	}
	part := logText[from:]
	if len(part) > devLogMax {
		part = part[len(part)-devLogMax:]
	}
	done := build.Status == gtables.AppBuild_Status.Succeeded || build.Status == gtables.AppBuild_Status.Failed ||
		build.Status == gtables.AppBuild_Status.Superseded
	return c.JSON(fiber.Map{"success": true, "data": fiber.Map{
		"build_no":      build.Entry_no,
		"status":        build.Status.String(),
		"done":          done,
		"error_message": build.Error_message.String(),
		"log":           string(part),
		"log_size":      len(logText),
		// the build the server runs now (0: the program without apps): live once it is this one
		"running_build": appbundle.Current(),
		"live":          appbundle.Current() == build.Entry_no,
	}})
}
