package system

import (
	"errors"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
)

// Message returns the result of a codeunit that shows an information dialog; the text is
// filled in with StrSubstNo. Pass a translated text (i18n Message), never English literals.
func Message(text string, values ...any) codeunits.Result {
	return codeunits.Message(StrSubstNo(text, values...))
}

// Error returns an error with the text filled in with StrSubstNo: returned from a codeunit's
// Run it stops the codeunit and is shown to the user. Pass a translated text (i18n Message);
// table triggers return apperrors errors instead, which name the field.
func Error(text string, values ...any) error {
	return errors.New(StrSubstNo(text, values...))
}
