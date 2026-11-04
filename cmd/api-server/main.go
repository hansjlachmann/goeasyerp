package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/hansjlachmann/openerp/backend/api"
	blapps "github.com/hansjlachmann/openerp/backend/business-logic/apps"
	blmigrations "github.com/hansjlachmann/openerp/backend/business-logic/migrations"
	"github.com/hansjlachmann/openerp/backend/business-logic/refusedposting"
	"github.com/hansjlachmann/openerp/backend/business-logic/scheduler"
	// The business logic of the tables (triggers), registered by its init functions
	_ "github.com/hansjlachmann/openerp/backend/business-logic/pages"
	_ "github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/business-logic/themerepo"
	fapps "github.com/hansjlachmann/openerp/backend/foundation/apps"
	"github.com/hansjlachmann/openerp/backend/foundation/company"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	"github.com/hansjlachmann/openerp/backend/foundation/objects"
	"github.com/hansjlachmann/openerp/backend/foundation/secrets"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	fmt.Println("=== OpenERP API Server ===")

	// Initialize object registry
	registry := objects.NewObjectRegistry()

	// Register tables (every table definition, generated)
	records := gtables.NewRecords()
	ids := make([]int, 0, len(records))
	for id := range records {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		if err := registry.RegisterTable(id, records[id]); err != nil {
			log.Printf("Warning: Failed to register table %d: %v\n", id, err)
		}
	}

	// The installed apps (application.yaml): the base application, later extension apps.
	// Every table, page and codeunit number must be in the series of one of them.
	baseApp, err := fapps.LoadBase()
	if err != nil {
		log.Fatalf("Failed to read the base application: %v", err)
	}
	installedApps := []*fapps.App{baseApp}
	if err := blapps.CheckObjectNumbers(installedApps); err != nil {
		log.Fatalf("%v", err)
	}
	fmt.Printf("✓ %s %s\n", baseApp.Name, baseApp.Version)

	var db *database.Database
	var companyName string
	var port string

	// Check if we're in Docker/production mode (env vars set)
	dbHost := os.Getenv("DB_HOST")
	if dbHost != "" {
		// PostgreSQL mode (Docker/production)
		config := &database.PostgresConfig{
			Host:     dbHost,
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "openerp"),
			Password: getEnv("DB_PASSWORD", "openerp"),
			DBName:   getEnv("DB_NAME", "openerp"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		}

		fmt.Printf("Connecting to PostgreSQL at %s:%s...\n", config.Host, config.Port)

		db, err = database.CreateOrOpenPostgresDatabase(config)
		if err != nil {
			log.Fatalf("Failed to connect to PostgreSQL: %v", err)
		}

		companyName = getEnv("COMPANY_NAME", "cronus")
		port = getEnv("PORT", "8080")

		fmt.Printf("✓ Connected to PostgreSQL database: %s\n", config.DBName)
	} else {
		// SQLite mode (local development)
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Print("Enter database path (or press Enter for 'test.db'): ")
		scanner.Scan()
		dbPath := strings.TrimSpace(scanner.Text())
		if dbPath == "" {
			dbPath = "test.db"
		}

		db, err = database.OpenDatabase(dbPath)
		if err != nil {
			log.Fatalf("Failed to open database: %v", err)
		}

		fmt.Printf("✓ Database opened: %s\n", dbPath)

		fmt.Print("Enter company name (or press Enter for 'cronus'): ")
		scanner.Scan()
		companyName = strings.TrimSpace(scanner.Text())
		if companyName == "" {
			companyName = "cronus"
		}

		port = getEnv("PORT", "8080")
	}

	defer func() { _ = db.CloseDatabase() }()

	// Enter company (auto-create if it doesn't exist in Docker mode)
	// Use NewManagerWithMigrations to enable versioned migrations
	companyMgr := company.NewManagerWithMigrations(db, registry, blmigrations.GetAll())
	if err := companyMgr.EnterCompany(companyName); err != nil {
		// In Docker mode, auto-create the company if it doesn't exist — only on an empty
		// database: when other companies exist, COMPANY_NAME was renamed or deleted on
		// purpose, so enter an existing one instead of creating an empty company again
		existing, _ := companyMgr.ListCompanies()
		if dbHost != "" && strings.Contains(err.Error(), "does not exist") && len(existing) > 0 {
			fmt.Printf("Company '%s' not found; using existing company '%s'\n", companyName, existing[0])
			companyName = existing[0]
			if enterErr := companyMgr.EnterCompany(companyName); enterErr != nil {
				log.Fatalf("Failed to enter company: %v", enterErr)
			}
		} else if dbHost != "" && strings.Contains(err.Error(), "does not exist") {
			fmt.Printf("Company '%s' not found, creating...\n", companyName)
			if createErr := companyMgr.CreateCompany(companyName); createErr != nil {
				log.Fatalf("Failed to create company: %v", createErr)
			}
			fmt.Printf("✓ Company created: %s\n", companyName)

			// Now enter the newly created company
			if enterErr := companyMgr.EnterCompany(companyName); enterErr != nil {
				log.Fatalf("Failed to enter company: %v", enterErr)
			}
		} else {
			log.Fatalf("Failed to enter company: %v", err)
		}
	}

	fmt.Printf("✓ Company entered: %s\n", companyName)

	fmt.Printf("✓ Per-request JWT sessions enabled\n")

	// Fields with encrypted: true (the SMTP password) are stored encrypted with a key from
	// the environment; values saved before that are encrypted now.
	secretsDBType := database.DBTypeSQLite
	if dbHost != "" {
		secretsDBType = database.DBTypePostgres
	}
	if src := secrets.Source(); src != "" {
		fmt.Printf("✓ Stored secrets encrypted (key from %s)\n", src)
		companies, err := companyMgr.ListCompanies()
		if err != nil {
			log.Printf("WARNING: encrypt stored secrets: %v", err)
		}
		if n, err := gtables.EncryptStoredSecrets(db.GetConnection(), secretsDBType, companies); err != nil {
			log.Printf("WARNING: %v", err)
		} else if n > 0 {
			fmt.Printf("✓ %d stored secret(s) encrypted\n", n)
		}
	} else {
		log.Println("WARNING: No OPENERP_ENCRYPTION_KEY or JWT_SECRET set - the SMTP password is stored unencrypted")
	}
	// Installed themes are per user (User_Theme); themes installed before that belong to the
	// user who installed them
	if n, err := themerepo.LinkInstallers(db.GetConnection(), secretsDBType); err != nil {
		log.Printf("WARNING: link installed themes to their users: %v", err)
	} else if n > 0 {
		fmt.Printf("✓ %d installed theme(s) added to the themes of the user who installed them\n", n)
	}
	if err := blapps.SyncInstalledApps(db.GetConnection(), secretsDBType, installedApps); err != nil {
		log.Printf("WARNING: installed apps: %v", err)
	}
	if err := blapps.SyncObjects(db.GetConnection(), secretsDBType); err != nil {
		log.Printf("WARNING: objects: %v", err)
	}
	fmt.Println()

	// Create and setup API server
	var server *api.Server
	if dbHost != "" {
		// PostgreSQL mode
		server = api.NewServerFull(db.GetConnection(), database.DBTypePostgres, registry)
	} else {
		// SQLite mode
		server = api.NewServerFull(db.GetConnection(), database.DBTypeSQLite, registry)
	}
	server.Setup()
	// Postings the balance check refuses are logged for an administrator (table 904)
	refusedposting.Register(db.GetConnection(), db.GetDBType())

	// Start server in a goroutine
	go func() {
		if err := server.StartOnPort(port); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Start the Job Queue scheduler (in-process poller for automatic job runs)
	schedDBType := database.DBTypeSQLite
	if dbHost != "" {
		schedDBType = database.DBTypePostgres
	}
	sched := scheduler.New(db.GetConnection(), schedDBType, companyMgr.ListCompanies)
	sched.Start()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	fmt.Println("\n✅ API Server is running!")
	fmt.Printf("📡 Listening on port %s\n", port)
	fmt.Println("📝 Press Ctrl+C to stop the server")
	fmt.Println("Available endpoints:")
	fmt.Println("  GET    /health")
	fmt.Println("  GET    /api/session")
	fmt.Println("  GET    /api/tables/Customer/list")
	fmt.Println("  GET    /api/tables/Customer/card/:id")
	fmt.Println("  POST   /api/tables/Customer/insert")
	fmt.Println("  PUT    /api/tables/Customer/modify/:id")
	fmt.Println("  DELETE /api/tables/Customer/delete/:id")
	fmt.Println("  POST   /api/tables/Customer/validate")
	fmt.Println("\n(Same endpoints available for Payment_terms and Customer_ledger_entry)")

	<-quit
	fmt.Println("\n🛑 Shutting down server...")

	sched.Stop()

	if err := server.Shutdown(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	fmt.Println("✓ Server stopped")
}
