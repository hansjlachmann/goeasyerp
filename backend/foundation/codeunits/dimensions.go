package codeunits

import (
	"sync"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
)

// DimensionLabelsLoader reads the names a company gives its two dimensions (General Ledger
// Setup Dimension-1 / Dimension-2); set at startup (the API server), nil in tests and tools —
// then the texts show the generic names "Dimension 1" / "Dimension 2"
var DimensionLabelsLoader func(company string) [2]string

// the names per company, read again after dimensionLabelsTTL (or ForgetDimensionLabels)
var dimensionLabels sync.Map // company → cachedDimensionLabels

const dimensionLabelsTTL = 30 * time.Second

type cachedDimensionLabels struct {
	labels [2]string
	read   time.Time
}

func init() {
	i18n.DimensionLabel = CurrentDimensionLabel
	i18n.UserLanguage = CurrentLanguage
}

// loadDimensionLabels: the company's dimension names (cached), when a context starts — not
// later, so a transaction in progress never waits for them
func loadDimensionLabels(company string) [2]string {
	if DimensionLabelsLoader == nil || company == "" {
		return [2]string{}
	}
	if v, ok := dimensionLabels.Load(company); ok {
		cached := v.(cachedDimensionLabels)
		if time.Since(cached.read) < dimensionLabelsTTL {
			return cached.labels
		}
	}
	labels := DimensionLabelsLoader(company)
	dimensionLabels.Store(company, cachedDimensionLabels{labels: labels, read: time.Now()})
	return labels
}

// ForgetDimensionLabels: the company's dimension names changed (the setup was modified)
func ForgetDimensionLabels(company string) {
	dimensionLabels.Delete(company)
}

// CurrentDimensionLabel returns the name the current goroutine's company gives dimension n
// (1 or 2); "" without a context or a name
func CurrentDimensionLabel(n int) string {
	ctx := GetCurrentContext()
	if ctx == nil || n < 1 || n > 2 {
		return ""
	}
	return ctx.DimensionLabels[n-1]
}
