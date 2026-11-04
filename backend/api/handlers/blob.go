package handlers

import (
	"bytes"
	"errors"

	"github.com/gofiber/fiber/v2"

	apitypes "github.com/hansjlachmann/goeasyerp/backend/api/types"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/themes"
)

// Pictures (BLOB fields such as Item.Picture): never sent in the table API's records (PublicMap
// gives Yes/No) and never written by insert/modify; these endpoints serve, store and remove one
// record's picture.
//
//	GET    /api/tables/:table/blob/:field/:id  the picture (read permission)
//	PUT    /api/tables/:table/blob/:field/:id  store an uploaded image (modify permission)
//	DELETE /api/tables/:table/blob/:field/:id  remove it (modify permission)
//
// An upload is untrusted: only PNG, JPEG and WebP of a limited size are accepted, decoded and
// re-encoded (themes.ProcessImage) — the uploaded bytes are never stored or served.

// PictureWidth is the width a stored picture is scaled down to
const PictureWidth = 1600

type blobRecord interface {
	FieldValue(no int) (any, bool)
	SetFieldValue(no int, value any) bool
}

// blobField finds the table's BLOB field by name
func blobField(table ftables.Table, name string) (ftables.FieldInfo, bool) {
	for _, f := range table.GetFields() {
		if f.Name == name && f.Type == ftables.FieldTypeBlob && !f.Sensitive {
			return f, true
		}
	}
	return ftables.FieldInfo{}, false
}

// loadBlobRecord reads the record of the URL and its BLOB field
func (h *TablesHandler) loadBlobRecord(c *fiber.Ctx) (ftables.Table, blobRecord, ftables.FieldInfo, string, error) {
	sess := getSession(c)
	if sess == nil {
		return nil, nil, ftables.FieldInfo{}, "", c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}
	tableName := c.Params("table")
	language := sess.GetLanguage()
	table, err := h.getTable(tableName, sess.GetCompany())
	if err != nil {
		return nil, nil, ftables.FieldInfo{}, language, c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.TableNotFound(tableName).Message(language)))
	}
	field, ok := blobField(table, c.Params("field"))
	rec, okRec := table.(blobRecord)
	if !ok || !okRec {
		return nil, nil, ftables.FieldInfo{}, language, c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.PictureNoField(c.Params("field")).Message(language)))
	}
	id := c.Params("id")
	if !table.Get(parseRecordKey(id, table)) {
		tableCaption := i18n.GetInstance().TableCaption(tableName, language)
		return nil, nil, ftables.FieldInfo{}, language, c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.RecordNotFound(tableCaption, id).Message(language)))
	}
	return table, rec, field, language, nil
}

// GetBlob serves a record's picture (the re-encoded image stored by PutBlob)
func (h *TablesHandler) GetBlob(c *fiber.Ctx) error {
	_, rec, field, _, err := h.loadBlobRecord(c)
	if rec == nil {
		return err
	}
	value, _ := rec.FieldValue(field.No)
	data, _ := value.([]byte)
	if len(data) == 0 {
		return c.SendStatus(fiber.StatusNotFound)
	}
	contentType := "image/jpeg"
	if bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		contentType = "image/png"
	}
	c.Set(fiber.HeaderContentType, contentType)
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Send(data)
}

// PutBlob stores an uploaded image as the record's picture (the request body is the file)
func (h *TablesHandler) PutBlob(c *fiber.Ctx) error {
	table, rec, field, language, err := h.loadBlobRecord(c)
	if rec == nil {
		return err
	}
	img, err := themes.ProcessImage(c.Body(), PictureWidth)
	if err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.PictureInvalid().Message(language)))
	}
	rec.SetFieldValue(field.No, img.Data)
	return h.modifyBlob(c, table, language)
}

// DeleteBlob removes the record's picture
func (h *TablesHandler) DeleteBlob(c *fiber.Ctx) error {
	table, rec, field, language, err := h.loadBlobRecord(c)
	if rec == nil {
		return err
	}
	rec.SetFieldValue(field.No, []byte(nil))
	return h.modifyBlob(c, table, language)
}

// modifyBlob saves the record (with its OnModify trigger)
func (h *TablesHandler) modifyBlob(c *fiber.Ctx, table ftables.Table, language string) error {
	if !table.Modify(true) {
		err := table.TriggerError()
		if err == nil {
			err = errors.New("modify failed")
		}
		return c.Status(400).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
	}
	return c.JSON(apitypes.NewSuccessResponse(map[string]interface{}{"ok": true}))
}
