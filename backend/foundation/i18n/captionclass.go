package i18n

import (
	"fmt"
	"maps"
	"strconv"
	"strings"
	"sync"
)

// CaptionClass (classic CaptionClass): a field declares a caption expression in its table YAML
// (caption_class: "<area>,<arguments>"), and its caption is resolved with it — per company and
// language, whenever the caption is translated (page and table captions in the API, the filter
// pane, request pages, report captions, error messages). The areas:
//
//   - 1 dimensions: "1,<kind>,<n>" — kind 1 a dimension code field, 2 a shortcut dimension code
//     field, 3 a dimension filter field, 4 a shortcut dimension filter field; n the dimension. The
//     generic name in the field's caption ("Dimension 1", "Dimensjon 1") becomes the name the
//     company gives the dimension (General Ledger Setup Dimension-1/-2, DimensionLabel); without
//     one the caption stays.
//   - 3 free text: "3,<text>" — the caption is the text (commas allowed).
//
// Area 2 (amounts including/excluding tax by the document's Prices Including Tax) is reserved for
// the documents that show such amounts.
var (
	captionClassMu sync.RWMutex
	captionClasses = map[string]string{} // translation key of a field caption → expression
)

// SetCaptionClass declares the caption expression of a field caption's translation key
// (tables.<table>.fields.<field>; the generated table code registers its fields', the page
// registry its page fields' own)
func SetCaptionClass(key, expr string) {
	captionClassMu.Lock()
	defer captionClassMu.Unlock()
	captionClasses[key] = expr
}

// CaptionClassOf is the caption expression declared for a translation key ("" when none)
func CaptionClassOf(key string) string {
	captionClassMu.RLock()
	defer captionClassMu.RUnlock()
	return captionClasses[key]
}

// FieldKey is the translation key of a table field's caption (tables.<table>.fields.<field>)
func FieldKey(tableName, fieldName string) string {
	return fmt.Sprintf("tables.%s.fields.%s", normalizeTableName(tableName), fieldName)
}

// CheckCaptionClass reports what is wrong with a caption expression ("" when it is valid):
// tablegen and the page registry refuse a field with an invalid one
func CheckCaptionClass(expr string) string {
	parts := strings.Split(expr, ",")
	switch parts[0] {
	case "1":
		if len(parts) != 3 {
			return "a dimension caption class is 1,<kind>,<dimension>"
		}
		kind, err := strconv.Atoi(parts[1])
		if err != nil || kind < 1 || kind > 4 {
			return "the kind of a dimension caption class is 1 (code), 2 (shortcut code), 3 (filter) or 4 (shortcut filter)"
		}
		n, err := strconv.Atoi(parts[2])
		if err != nil || n < 1 || n > 8 {
			return "the dimension of a dimension caption class is 1 to 8"
		}
		return ""
	case "3":
		if len(parts) < 2 || strings.TrimSpace(strings.Join(parts[1:], ",")) == "" {
			return "a free text caption class is 3,<text>"
		}
		return ""
	case "2":
		return "caption class area 2 (amounts including/excluding tax) is not available yet"
	}
	return fmt.Sprintf("unknown caption class area %q (1 dimensions, 3 free text)", parts[0])
}

// ResolveCaptionClass gives the caption a field with the expression has in the language: caption
// is the field's translated caption
func (s *TranslationService) ResolveCaptionClass(expr, caption, language string) string {
	parts := strings.Split(expr, ",")
	switch parts[0] {
	case "1":
		if len(parts) != 3 {
			return caption
		}
		n, err := strconv.Atoi(parts[2])
		if err != nil || DimensionLabel == nil {
			return caption
		}
		label := DimensionLabel(n)
		if label == "" {
			return caption
		}
		// the generic name ("Dimension 1", "Dimensjon 1") in the caption becomes the company's
		generic := s.translate(fmt.Sprintf("tables.dimension_%d.caption", n), language)
		return replaceFold(caption, generic, label)
	case "3":
		return strings.Join(parts[1:], ",")
	}
	return caption
}

// CaptionClasses returns the declared caption expressions by translation key (a copy: tests)
func CaptionClasses() map[string]string {
	captionClassMu.RLock()
	defer captionClassMu.RUnlock()
	return maps.Clone(captionClasses)
}

// KeysWithPrefix returns the translation keys of a language starting with prefix (tests)
func (s *TranslationService) KeysWithPrefix(language, prefix string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var keys []string
	for k := range s.translations[language] {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys
}
