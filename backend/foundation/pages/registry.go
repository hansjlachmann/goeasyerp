package pages

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Registry manages page definitions
type Registry struct {
	pages map[int]*PageDefinition
	menu  *MenuDefinition
	mu    sync.RWMutex
}

var (
	instance *Registry
	once     sync.Once
)

// GetRegistry returns the singleton page registry
func GetRegistry() *Registry {
	once.Do(func() {
		instance = &Registry{
			pages: make(map[int]*PageDefinition),
		}
		// Load page definitions automatically
		if err := instance.LoadPages(); err != nil {
			fmt.Printf("Warning: Failed to load page definitions: %v\n", err)
		}
		// Load default menu (optional — per-user menus are loaded from pages/menus/ folder)
		_ = instance.LoadMenu()
	})
	return instance
}

// LoadPages loads all page definitions from YAML files
func (r *Registry) LoadPages() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Get project root
	rootPath, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	pagesPath := filepath.Join(rootPath, "backend", "definitions", "pages")

	// Check if pages directory exists
	if _, err := os.Stat(pagesPath); os.IsNotExist(err) {
		return fmt.Errorf("pages directory not found: %s", pagesPath)
	}

	// Find all YAML files
	files, err := filepath.Glob(filepath.Join(pagesPath, "*.yaml"))
	if err != nil {
		return fmt.Errorf("failed to read pages directory: %w", err)
	}

	loadedCount := 0
	for _, file := range files {
		if err := r.loadPageFile(file); err != nil {
			fmt.Printf("Warning: Failed to load %s: %v\n", filepath.Base(file), err)
		} else {
			loadedCount++
		}
	}

	if loadedCount == 0 {
		return fmt.Errorf("no page definitions loaded")
	}

	fmt.Printf("✓ Loaded %d page definition(s)\n", loadedCount)
	return nil
}

// loadPageFile loads a single page definition file
func (r *Registry) loadPageFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var pageDef PageDefinition
	if err := yaml.Unmarshal(data, &pageDef); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// A subpage section has no fields: send an empty list, not null
	for i := range pageDef.Page.Layout.Sections {
		if pageDef.Page.Layout.Sections[i].Fields == nil {
			pageDef.Page.Layout.Sections[i].Fields = []Field{}
		}
	}

	// Store by page ID
	r.pages[pageDef.Page.ID] = &pageDef

	return nil
}

// LoadMenu loads the menu definition
func (r *Registry) LoadMenu() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Get project root
	rootPath, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	menuPath := filepath.Join(rootPath, "backend", "definitions", "menu.yaml")

	// Check if menu file exists
	if _, err := os.Stat(menuPath); os.IsNotExist(err) {
		return fmt.Errorf("menu file not found: %s", menuPath)
	}

	data, err := os.ReadFile(menuPath)
	if err != nil {
		return fmt.Errorf("failed to read menu file: %w", err)
	}

	var menuDef MenuDefinition
	if err := yaml.Unmarshal(data, &menuDef); err != nil {
		return fmt.Errorf("failed to parse menu YAML: %w", err)
	}

	r.menu = &menuDef

	fmt.Printf("✓ Loaded menu with %d group(s)\n", len(menuDef.Menu))
	return nil
}

// GetPage returns a page definition by ID
func (r *Registry) GetPage(pageID int) (*PageDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	page, ok := r.pages[pageID]
	if !ok {
		return nil, fmt.Errorf("page %d not found", pageID)
	}

	return page, nil
}

// GetMenu returns the default menu definition
func (r *Registry) GetMenu() *MenuDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.menu
}

// GetMenuByName loads and returns a menu by name from the menus folder
// Falls back to default menu.yaml if named menu not found
func (r *Registry) GetMenuByName(menuName string) *MenuDefinition {
	// If no menu name specified, return default
	if menuName == "" {
		return r.GetMenu()
	}

	// Get project root
	rootPath, err := findProjectRoot()
	if err != nil {
		fmt.Printf("Warning: Failed to find project root for menu: %v\n", err)
		return r.GetMenu()
	}

	// Try to load from menus folder (e.g., menus/admin.yaml)
	// Strip .yaml extension if already present (filename may be stored with or without extension)
	// Convert to lowercase for case-insensitive file matching on Linux
	cleanName := strings.ToLower(strings.TrimSuffix(menuName, ".yaml"))
	menuPath := filepath.Join(rootPath, "backend", "definitions", "menus", cleanName+".yaml")

	// Check if menu file exists
	if _, err := os.Stat(menuPath); os.IsNotExist(err) {
		fmt.Printf("Warning: Menu file not found: %s, falling back to default\n", menuPath)
		return r.GetMenu()
	}

	data, err := os.ReadFile(menuPath)
	if err != nil {
		fmt.Printf("Warning: Failed to read menu file %s: %v\n", menuPath, err)
		return r.GetMenu()
	}

	var menuDef MenuDefinition
	if err := yaml.Unmarshal(data, &menuDef); err != nil {
		fmt.Printf("Warning: Failed to parse menu YAML %s: %v\n", menuPath, err)
		return r.GetMenu()
	}

	return &menuDef
}

// GetAllPages returns all loaded page definitions
func (r *Registry) GetAllPages() map[int]*PageDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy to prevent external modification
	pages := make(map[int]*PageDefinition, len(r.pages))
	for id, page := range r.pages {
		pages[id] = page
	}

	return pages
}

// findProjectRoot finds the project root directory
func findProjectRoot() (string, error) {
	// Start from current working directory
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up until we find go.mod
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding go.mod
			return "", fmt.Errorf("project root not found (no go.mod)")
		}
		dir = parent
	}
}

// LookupPage returns the list page that shows a table's records — the page a lookup's "New"
// opens: the table's lookup_page_id, else its list page with the lowest number; 0 for none
func (r *Registry) LookupPage(tableName string) int {
	if id := GetTableMetadata().GetLookupPageID(tableName); id != 0 {
		return id
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	best := 0
	for id, page := range r.pages {
		if page.Page.Type == "List" && metadataKey(page.Page.SourceTable) == metadataKey(tableName) && (best == 0 || id < best) {
			best = id
		}
	}
	return best
}

// NewRecordPage is the page a lookup's New opens to enter a record of a table: its lookup page —
// none (0) when that page enters nothing (read-only, editable: false, without a card page) or
// the table is written by the system or by posting only (posted: true: the Object table, ledger
// entries)
func (r *Registry) NewRecordPage(tableName string) int {
	if GetTableMetadata().IsPosted(tableName) {
		return 0
	}
	id := r.LookupPage(tableName)
	if id == 0 {
		return 0
	}
	r.mu.RLock()
	page, ok := r.pages[id]
	r.mu.RUnlock()
	// a read-only list without a card enters nothing (the Objects page); a read-only list with a
	// card enters records on its card (the chart of accounts)
	if ok && page.Page.Editable != nil && !*page.Page.Editable && page.Page.CardPageID == 0 {
		return 0
	}
	return id
}

// CalculatedFields returns the calculated fields (calculated: true) of the pages showing a
// table: page values a record sent from such a page carries, which are not table fields
func (r *Registry) CalculatedFields(tableName string) map[string]bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	fields := map[string]bool{}
	for _, page := range r.pages {
		if metadataKey(page.Page.SourceTable) != metadataKey(tableName) {
			continue
		}
		all := []Field{}
		if page.Page.Layout.Repeater != nil {
			all = append(all, page.Page.Layout.Repeater.Fields...)
		}
		for _, s := range page.Page.Layout.Sections {
			all = append(all, s.Fields...)
		}
		all = append(all, page.Page.Footer...)
		for _, f := range all {
			if f.Calculated {
				fields[strings.ToLower(f.Source)] = true
			}
		}
	}
	return fields
}
