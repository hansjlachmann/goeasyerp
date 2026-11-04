package handlers

import (
	"github.com/gofiber/fiber/v2"
	apitypes "github.com/hansjlachmann/goeasyerp/backend/api/types"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
)

// TranslationsHandler handles translation-related API requests
type TranslationsHandler struct{}

// NewTranslationsHandler creates a new translations handler
func NewTranslationsHandler() *TranslationsHandler {
	return &TranslationsHandler{}
}

// GetTranslations returns all message translations for the requested language
// GET /api/translations?lang=nb-NO
// Uses ?lang query param if provided, otherwise falls back to session language
func (h *TranslationsHandler) GetTranslations(c *fiber.Ctx) error {
	language := "en-US"

	// Prefer explicit language from query parameter (frontend sends user's translation_key)
	if lang := c.Query("lang"); lang != "" {
		language = lang
	} else {
		// Fall back to session language
		sess := getSession(c)
		if sess != nil {
			lang := normalizeLanguageCode(sess.GetLanguage())
			if lang != "" {
				language = lang
			}
		}
	}

	messages := i18n.GetInstance().GetAllMessages(language)

	response := apitypes.NewSuccessResponse(messages)
	return c.JSON(response)
}
