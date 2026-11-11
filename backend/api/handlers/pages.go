package handlers

import (
	"fmt"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	apitypes "github.com/hansjlachmann/goeasyerp/backend/api/types"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
)

// PagesHandler handles page-related API endpoints
type PagesHandler struct{}

// NewPagesHandler creates a new pages handler
func NewPagesHandler() *PagesHandler {
	return &PagesHandler{}
}

// GetPage returns a page definition by ID
// GET /api/pages/:id
func (h *PagesHandler) GetPage(c *fiber.Ctx) error {
	// Get page ID from URL parameter
	pageIDStr := c.Params("id")
	pageID, err := strconv.Atoi(pageIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(apperrors.InvalidPageID().Message("en-US")))
	}

	// Get page definition from registry
	registry := pages.GetRegistry()
	shared, err := registry.GetPage(pageID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(apitypes.NewErrorResponse(apperrors.PageNotFound(pageIDStr).Message("en-US")))
	}
	// The registry's definition is shared by all requests: translate and flag a copy
	pageDef := shared.Clone()

	// Get current session for captions
	sess := getSession(c)
	if sess == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}

	// Get table metadata for primary key info
	tableMeta := pages.GetTableMetadata()
	primaryKeyFields := tableMeta.GetPrimaryKeyFields(pageDef.Page.SourceTable)
	// Send the PK field names so the frontend can identify records even when the
	// primary key is not displayed on the page (e.g. setup tables).
	pageDef.Page.PrimaryKeyFields = primaryKeyFields
	pageDef.Page.HasRecordTriggers = pages.HasRecordTriggers(pageID)
	pageDef.Page.HasNewRecordTriggers = pages.HasNewRecordTriggers(pageID)
	pageDef.Page.HasOpenTrigger = pages.HasOpenTrigger(pageID)
	if factory, ok := gtables.GetTableFactory(pageDef.Page.SourceTable); ok {
		tbl := factory()
		pageDef.Page.FlowFields = tbl.GetFlowFields()
		pageDef.Page.SortableFlowFields = nil
		if ff, ok := tbl.(flowFieldFilterer); ok {
			for _, f := range pageDef.Page.FlowFields {
				if _, sortable := ff.FlowFieldFilterKind(f); sortable {
					pageDef.Page.SortableFlowFields = append(pageDef.Page.SortableFlowFields, f)
				}
			}
		}
		pageDef.Page.FlowFilterFields = nil
		lang := normalizeLanguageCode(sess.GetLanguage())
		if lang == "" {
			lang = "en-US"
		}
		for _, ff := range tbl.GetFlowFilterFields() {
			pageDef.Page.FlowFilterFields = append(pageDef.Page.FlowFilterFields, pages.FlowFilterField{
				Name:    ff.Name,
				Kind:    ff.Kind,
				Caption: i18n.GetInstance().FieldCaption(pageDef.Page.SourceTable, ff.Name, lang),
			})
		}
	}
	// The table's field properties the pages follow: read-only (editable: false) and the
	// decimal places shown; the card title's fields
	tableFields := map[string]ftables.FieldInfo{}
	if factory, ok := gtables.GetTableFactory(pageDef.Page.SourceTable); ok {
		pageDef.Page.SetupTable = factory().IsSetupTable()
		pageDef.Page.AutoKey = pages.GetTableMetadata().IsAutoKey(pageDef.Page.SourceTable)
		pageDef.Page.Posted = pages.GetTableMetadata().IsPosted(pageDef.Page.SourceTable)
		if pageDef.Page.Posted {
			// posted entries are not entered or deleted on a page (the table API refuses it)
			actions := pageDef.Page.Actions[:0]
			for _, action := range pageDef.Page.Actions {
				if action.Name != "New" && action.Name != "Delete" {
					actions = append(actions, action)
				}
			}
			pageDef.Page.Actions = actions
		}
		for _, f := range factory().GetFields() {
			tableFields[f.Name] = f
		}
	}
	applyTableField := func(field *pages.Field) {
		info, ok := tableFields[field.Source]
		if !ok {
			return
		}
		if info.ReadOnly || !tableMeta.IsPostedFieldEditable(pageDef.Page.SourceTable, field.Source) {
			readOnly := false
			field.Editable = &readOnly // a page cannot make a read-only table field editable
		}
		if field.DecimalPlaces == "" {
			field.DecimalPlaces = info.DecimalPlaces
		}
	}
	pageDef.Page.DataCaptionFields = tableMeta.GetDataCaptionFields(pageDef.Page.SourceTable)
	if len(pageDef.Page.DataCaptionFields) == 0 && pageDef.Page.Type == "Card" {
		pageDef.Page.DataCaptionFields = primaryKeyFields // a card without them shows its key
	}

	// Build a set for quick lookup
	pkSet := make(map[string]bool, len(primaryKeyFields))
	for _, pk := range primaryKeyFields {
		pkSet[pk] = true
	}

	// Get field captions using i18n
	ts := i18n.GetInstance()
	lang := normalizeLanguageCode(sess.GetLanguage())
	// Fallback to en-US if language is not set
	if lang == "" {
		lang = "en-US"
	}

	var captions *apitypes.CaptionData
	if pageDef.Page.SourceTable != "" {
		// A matrix's row fields and filters: captions of the rows' table; its cells can be
		// entered only with the page's OnMatrixValidate trigger
		// (filters of a matrix with a card date filter: the card's table)
		matrixCaptions := func(mx *pages.Matrix) {
			for i, fields := range [][]pages.Field{mx.Filters, mx.Fields} {
				table := mx.Table
				if i == 0 && mx.CardDateFilter != "" {
					table = pageDef.Page.SourceTable
				}
				for j := range fields {
					field := &fields[j]
					if field.Caption == "" {
						field.Caption = pageFieldCaption(ts, pageDef.Page.Name, table, field, lang)
					}
				}
			}
			mx.Editable = mx.Editable && pages.HasMatrixValidate(pageDef.Page.ID)
		}
		if pageDef.Page.Layout.Matrix != nil {
			matrixCaptions(pageDef.Page.Layout.Matrix)
		}

		// Build field captions and types maps
		fieldCaptions := make(map[string]string)
		fieldTypes := make(map[string]string)

		// Get captions for card page sections and mark primary key / required fields
		for i := range pageDef.Page.Layout.Sections {
			for j := range pageDef.Page.Layout.Sections[i].Fields {
				field := &pageDef.Page.Layout.Sections[i].Fields[j]
				fieldCaptions[field.Source] = pageFieldCaption(ts, pageDef.Page.Name, pageDef.Page.SourceTable, field, lang)
				if ft := tableMeta.GetFieldType(pageDef.Page.SourceTable, field.Source); ft != "" {
					fieldTypes[field.Source] = ft
				}
				if field.Calculated {
					markCalculated(&pageDef.Page, field, fieldTypes)
				}
				if pkSet[field.Source] {
					field.PrimaryKey = true
				}
				if tableMeta.IsFieldRequired(pageDef.Page.SourceTable, field.Source) {
					field.Required = true
				}
				if tableMeta.IsFieldMasked(pageDef.Page.SourceTable, field.Source) {
					field.Masked = true
				}
				applyTableField(field)
				markFlowFieldDrilldown(tableMeta, pageDef.Page.SourceTable, field)
			}
			// A period grid's filter fields and columns (FlowFields or calculated)
			if grid := pageDef.Page.Layout.Sections[i].PeriodGrid; grid != nil {
				for _, fields := range [][]pages.Field{grid.Filters, grid.Columns} {
					for j := range fields {
						field := &fields[j]
						fieldCaptions[field.Source] = pageFieldCaption(ts, pageDef.Page.Name, pageDef.Page.SourceTable, field, lang)
						if ft := tableMeta.GetFieldType(pageDef.Page.SourceTable, field.Source); ft != "" {
							fieldTypes[field.Source] = ft
						}
						if field.Calculated {
							markCalculated(&pageDef.Page, field, fieldTypes)
						}
						applyTableField(field)
					}
				}
			}
			if mx := pageDef.Page.Layout.Sections[i].Matrix; mx != nil {
				matrixCaptions(mx)
			}
		}

		// Get captions for list page repeater and mark primary key / required fields
		if pageDef.Page.Layout.Repeater != nil {
			for i := range pageDef.Page.Layout.Repeater.Fields {
				field := &pageDef.Page.Layout.Repeater.Fields[i]
				fieldCaptions[field.Source] = pageFieldCaption(ts, pageDef.Page.Name, pageDef.Page.SourceTable, field, lang)
				if ft := tableMeta.GetFieldType(pageDef.Page.SourceTable, field.Source); ft != "" {
					fieldTypes[field.Source] = ft
				}
				if field.Calculated {
					markCalculated(&pageDef.Page, field, fieldTypes)
				}
				if pkSet[field.Source] {
					field.PrimaryKey = true
				}
				if tableMeta.IsFieldRequired(pageDef.Page.SourceTable, field.Source) {
					field.Required = true
				}
				if tableMeta.IsFieldMasked(pageDef.Page.SourceTable, field.Source) {
					field.Masked = true
				}
				applyTableField(field)
				markFlowFieldDrilldown(tableMeta, pageDef.Page.SourceTable, field)
			}
		}

		// The footer's calculated fields (shown under a list for the cursor row) and the header
		// fields above it (their captions: the page's, else the table field's)
		for i := range pageDef.Page.Footer {
			field := &pageDef.Page.Footer[i]
			fieldCaptions[field.Source] = pageFieldCaption(ts, pageDef.Page.Name, pageDef.Page.SourceTable, field, lang)
			if ft := tableMeta.GetFieldType(pageDef.Page.SourceTable, field.Source); ft != "" {
				fieldTypes[field.Source] = ft
			}
			if field.Calculated {
				markCalculated(&pageDef.Page, field, fieldTypes)
			}
			applyTableField(field)
		}
		for i := range pageDef.Page.HeaderFields {
			header := &pageDef.Page.HeaderFields[i]
			header.Caption = pageFieldCaption(ts, pageDef.Page.Name, pageDef.Page.SourceTable, &pages.Field{Source: header.Source}, lang)
			for j := range header.Options {
				option := &header.Options[j]
				key := "pages." + normalizePageName(pageDef.Page.Name) + ".options." + header.Param + "." + i18n.Key(option.Caption)
				caption := ts.Translate(key, lang)
				if caption != key {
					option.Caption = caption
				}
			}
			// the type of the field it stands for (a Code in upper case, a date filter)
			if ft := tableMeta.GetFieldType(pageDef.Page.SourceTable, header.Source); ft != "" {
				fieldTypes[header.Source] = ft
			}
		}

		captions = &apitypes.CaptionData{
			Table:      ts.TableCaption(pageDef.Page.SourceTable, lang),
			Fields:     fieldCaptions,
			FieldTypes: fieldTypes,
		}
	}

	// Translate page caption
	translatedCaption := ts.PageCaption(pageDef.Page.Name, lang)
	// If no translation found (key returned), use original caption
	expectedKey := fmt.Sprintf("pages.%s.caption", normalizePageName(pageDef.Page.Name))
	if translatedCaption != expectedKey {
		pageDef.Page.Caption = translatedCaption
	}

	// Action and section captions: common.actions.<name> / common.sections.<name>, the YAML
	// caption when there is no translation
	translateActions(ts, pageDef.Page.Actions, lang)
	for i := range pageDef.Page.Layout.Sections {
		sec := &pageDef.Page.Layout.Sections[i]
		sec.Caption = commonCaption(ts, "sections", sec.Name, sec.Caption, lang)
		if sec.Matrix != nil {
			translateActions(ts, sec.Matrix.Actions, lang)
		}
	}

	// Build navigation translations for breadcrumbs
	navigation := map[string]string{
		"home": ts.CommonTranslation("navigation.home", lang),
	}

	return c.JSON(apitypes.APIResponse{
		Success:    true,
		Data:       pageDef,
		Captions:   captions,
		Navigation: navigation,
	})
}

// translateActions translates the captions of actions and their menu items
func translateActions(ts *i18n.TranslationService, actions []pages.Action, lang string) {
	for i := range actions {
		a := &actions[i]
		a.Caption = commonCaption(ts, "actions", a.Name, a.Caption, lang)
		translateActions(ts, a.Items, lang)
	}
}

// commonCaption translates an action or section name ("Send Test E-mail") with the key
// common.<group>.<name as snake case> (send_test_e_mail); fallback when there is none.
func commonCaption(ts *i18n.TranslationService, group, name, fallback, lang string) string {
	key := group + "." + captionKey(name)
	if translated := ts.CommonTranslation(key, lang); translated != "common."+key {
		return translated
	}
	if fallback == "" {
		return name
	}
	return fallback
}

// captionKey turns a name into a translation key: lower case, other characters "_"
// ("Send Test E-mail" → "send_test_e_mail", "Back to List" → "back_to_list")
func captionKey(name string) string {
	return i18n.Key(name)
}

// optionKeys are the translation keys of an option field's values (captionKey); a value whose
// key an earlier one has gets a number ("Qty. Disc. + Cust./Item Disc." and "Qty. Disc. *
// Cust./Item Disc." → qty_disc_cust_item_disc, qty_disc_cust_item_disc_2)
func optionKeys(values []string) []string {
	keys := make([]string, len(values))
	seen := map[string]int{}
	for i, value := range values {
		key := captionKey(value)
		if key != "" {
			if seen[key]++; seen[key] > 1 {
				key = fmt.Sprintf("%s_%d", key, seen[key])
			}
		}
		keys[i] = key
	}
	return keys
}

// normalizePageName converts "Customer Card" to "customer_card"
// markCalculated makes a calculated page field (filled by the page's triggers) read-only, gives
// it its type, and lists it with the computed fields (not searched or sorted on the server)
// pageFieldCaption is a page field's caption: the page's own (pages.<page>.fields.<source> — a
// calculated field, or a caption that differs from the table's), else the table field's —
// resolved with the page field's caption expression, else the table field's (CaptionClass)
func pageFieldCaption(ts *i18n.TranslationService, pageName, table string, field *pages.Field, lang string) string {
	key := "pages." + normalizePageName(pageName) + ".fields." + field.Source
	caption := ts.Translate(key, lang)
	if caption == key {
		if field.CaptionClass == "" {
			return ts.FieldCaption(table, field.Source, lang)
		}
		caption = ts.FieldCaption(table, field.Source, lang)
	}
	// the page field's caption expression (CaptionClass), else the table field's
	expr := field.CaptionClass
	if expr == "" {
		expr = i18n.CaptionClassOf(i18n.FieldKey(table, field.Source))
	}
	if expr == "" {
		return caption
	}
	return ts.ResolveCaptionClass(expr, caption, lang)
}

func markCalculated(page *pages.PageMetadata, field *pages.Field, fieldTypes map[string]string) {
	readOnly := false
	field.Editable = &readOnly
	if field.Type != "" {
		fieldTypes[field.Source] = field.Type
	}
	page.FlowFields = append(page.FlowFields, field.Source)
}

func normalizePageName(name string) string {
	return i18n.Key(name)
}

// normalizeLanguageCode converts language codes to proper BCP-47 format
// e.g., "NB-NO" -> "nb-NO", "EN-US" -> "en-US"
func normalizeLanguageCode(code string) string {
	if code == "" {
		return ""
	}
	parts := strings.Split(code, "-")
	if len(parts) == 2 {
		return strings.ToLower(parts[0]) + "-" + strings.ToUpper(parts[1])
	}
	return strings.ToLower(code)
}

// GetMenu returns the menu structure based on user's assigned menu
// GET /api/menu
func (h *PagesHandler) GetMenu(c *fiber.Ctx) error {
	// Get current session to determine user's menu
	sess := getSession(c)
	menuName := ""
	if sess != nil {
		menuName = sess.GetMenu()
	}

	// Default to admin if no menu assigned
	if menuName == "" {
		menuName = "admin"
	}

	// Get menu from registry by name
	registry := pages.GetRegistry()
	menuDef := registry.GetMenuByName(menuName)
	if menuDef == nil {
		return c.Status(fiber.StatusNotFound).JSON(apitypes.NewErrorResponse(apperrors.MenuNotFound().Message("en-US")))
	}

	// Translate menu item names based on user's language
	language := "en-US"
	if sess != nil {
		lang := normalizeLanguageCode(sess.GetLanguage())
		if lang != "" {
			language = lang
		}
	}
	ts := i18n.GetInstance()

	// Build a translated copy so we don't mutate the cached definition
	translated := pages.MenuDefinition{
		Menu: make([]pages.MenuGroup, len(menuDef.Menu)),
	}
	for i, group := range menuDef.Menu {
		translated.Menu[i] = group
		translated.Menu[i].Name = ts.MenuItemCaption(group.Name, language)
		translated.Menu[i].Items = menuEntries(group.Items, ts, language)
	}

	return c.JSON(apitypes.NewSuccessResponse(&translated))
}

// menuEntries translates a menu's entries (and their sub-menus) and marks those whose page,
// report or codeunit exists as enabled — a sub-menu when one of its entries is; the others are
// shown greyed out (not built yet)
func menuEntries(items []pages.MenuItem, ts *i18n.TranslationService, language string) []pages.MenuItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]pages.MenuItem, len(items))
	for i, item := range items {
		out[i] = item
		if item.Separator {
			continue
		}
		out[i].Name = ts.MenuItemCaption(item.Name, language)
		out[i].Items = menuEntries(item.Items, ts, language)
		switch {
		case len(item.Items) > 0:
			for _, sub := range out[i].Items {
				if sub.Enabled {
					out[i].Enabled = true
					break
				}
			}
		case item.PageID > 0:
			_, err := pages.GetRegistry().GetPage(item.PageID)
			out[i].Enabled = err == nil
		case item.RunObject != "":
			out[i].Enabled = runObjectExists(item.RunObject)
		}
	}
	return out
}

// runObjectExists reports whether a menu entry's report or codeunit ("report:93",
// "codeunit:6") is registered
func runObjectExists(runObject string) bool {
	kind, no, ok := strings.Cut(runObject, ":")
	if !ok {
		return false
	}
	id, err := strconv.Atoi(no)
	if err != nil {
		return false
	}
	switch kind {
	case "report":
		_, found := reports.Get(id)
		return found
	case "codeunit":
		_, found := codeunits.Get(id)
		return found
	}
	return false
}

// GetAllPages returns all loaded page definitions
// GET /api/pages
func (h *PagesHandler) GetAllPages(c *fiber.Ctx) error {
	registry := pages.GetRegistry()
	allPages := registry.GetAllPages()

	// Convert map to slice for easier consumption
	pageList := make([]*pages.PageDefinition, 0, len(allPages))
	for _, page := range allPages {
		pageList = append(pageList, page)
	}

	return c.JSON(apitypes.NewSuccessResponse(pageList))
}

// markFlowFieldDrilldown: a FlowField's drilldown without filters of its own opens its page
// with the FlowField's filters (field.DrilldownFlowFilters; POST /api/pages/:id/drilldown)
func markFlowFieldDrilldown(tableMeta *pages.TableMetadata, table string, field *pages.Field) {
	if field.Drilldown == 0 || field.DrilldownFilterField != "" || field.DrilldownPageField != "" {
		return
	}
	if _, ok := tableMeta.GetFlowField(table, field.Source); ok {
		field.DrilldownFlowFilters = true
	}
}
