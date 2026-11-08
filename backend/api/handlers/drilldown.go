package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	apitypes "github.com/hansjlachmann/goeasyerp/backend/api/types"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/flowfilter"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// PageDrilldown gives the page and filters a FlowField's drilldown opens (classic DrillDown:
// the FlowField's source table filtered as the FlowField calculates — the record's values, a
// total account's Totaling, the FlowFilters set). Body: {"field", "record", "flow_filters":
// [{field, expression}]} (FlowFilters in ISO, as the list sends them); answer {"page_id",
// "filters": [{field, expression}]}. Nothing is read: the page opened reads with the user's
// own permissions.
// POST /api/pages/:id/drilldown
func PageDrilldown(c *fiber.Ctx) error {
	sess := getSession(c)
	if sess == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}
	language := sess.GetLanguage()
	pageID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(apperrors.InvalidPageID().Message(language)))
	}
	page, err := pages.GetRegistry().GetPage(pageID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(apitypes.NewErrorResponse(apperrors.PageNotFound(c.Params("id")).Message(language)))
	}
	var body struct {
		Field       string                 `json:"field"`
		Record      map[string]interface{} `json:"record"`
		FlowFilters []struct {
			Field      string `json:"field"`
			Expression string `json:"expression"`
		} `json:"flow_filters"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
	}
	field := pageField(page, body.Field)
	if field == nil || field.Drilldown == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
	}
	def, ok := pages.GetTableMetadata().GetFlowField(page.Page.SourceTable, field.Source)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
	}
	kinds := map[string]flowfilter.Kind{}
	if factory, ok := gtables.GetTableFactory(page.Page.SourceTable); ok {
		for _, ff := range factory().GetFlowFilterFields() {
			kinds[ff.Name] = flowfilter.Kind(ff.Kind)
		}
	}
	flowFilters := map[string]string{}
	for _, f := range body.FlowFilters {
		if _, ok := kinds[f.Field]; ok {
			flowFilters[f.Field] = f.Expression
		}
	}
	filters := pages.FlowFieldDrilldownFilters(def, body.Record, flowFilters, kinds)
	return c.JSON(apitypes.NewSuccessResponse(map[string]interface{}{"page_id": field.Drilldown, "filters": filters}))
}

// pageField finds a field of a list's repeater or a card's sections by its source
func pageField(page *pages.PageDefinition, source string) *pages.Field {
	if page.Page.Layout.Repeater != nil {
		for i := range page.Page.Layout.Repeater.Fields {
			if page.Page.Layout.Repeater.Fields[i].Source == source {
				return &page.Page.Layout.Repeater.Fields[i]
			}
		}
	}
	for i := range page.Page.Layout.Sections {
		for j := range page.Page.Layout.Sections[i].Fields {
			if page.Page.Layout.Sections[i].Fields[j].Source == source {
				return &page.Page.Layout.Sections[i].Fields[j]
			}
		}
	}
	return nil
}
