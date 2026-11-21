package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	blapps "github.com/hansjlachmann/goeasyerp/backend/business-logic/apps"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/appbundle"
	fapps "github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
)

// The installation's own builds of the extension apps (App Build Setup: Builds By = Local
// Builder; APP_BUILDS_DIR: the builds volume shared with the builder): the standard program
// starts the newest bundle built for its version (launchAppBundle), waits in maintenance while
// the apps of a new version are built, and a running server starts a newer bundle as soon as it
// is built (watchAppBuilds).

// launchAppBundle runs before anything else: it replaces this process with the bundle when
// there is one, or waits in maintenance until there is; it returns when the standard program
// is to run
func launchAppBundle(starting *appbundle.Starting) *appbundle.Starting {
	buildsDir := os.Getenv(appbundle.EnvBuildsDir)
	if buildsDir == "" || appbundle.Current() != 0 {
		return starting
	}
	config, ok := database.PostgresConfigFromEnv()
	if !ok {
		return starting
	}
	db, err := sql.Open("postgres", config.ConnectionString())
	if err != nil {
		log.Printf("app builds: %v", err)
		return starting
	}
	defer func() { _ = db.Close() }()
	err = db.Ping()
	if err != nil {
		log.Printf("app builds: %v", err)
		return starting
	}
	base, err := fapps.LoadBase()
	if err != nil {
		log.Printf("app builds: %v", err)
		return starting
	}
	home, err := os.Getwd()
	if err != nil {
		log.Printf("app builds: %v", err)
		return starting
	}
	d, err := appbundle.Decide(db, database.DBTypePostgres, buildsDir, base.Version, base.ID)
	if err != nil {
		log.Printf("app builds: %v", err)
		return starting
	}
	if d.Reason != "" {
		log.Printf("app builds: %s", d.Reason)
	}
	if d.Maintenance {
		log.Printf("app builds: the extension apps are not built for version %s yet — requesting their build and waiting in maintenance", base.Version)
		build, err := blapps.RequestUpgradeBuild(db, database.DBTypePostgres)
		if err != nil {
			log.Printf("app builds: requesting the build: %v", err)
		} else if build != nil {
			log.Printf("app builds: build %d requested", build.Entry_no)
		}
		ln, err := appbundle.Listen(starting, ":"+getEnv("PORT", "8080"))
		if err != nil {
			log.Fatalf("app builds: maintenance: %v", err)
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		d, err = appbundle.ServeMaintenance(ctx, db, ln, buildsDir, base.Version, base.ID, 10*time.Second)
		stop()
		if err != nil {
			log.Printf("app builds: maintenance stopped: %v", err)
			os.Exit(0)
		}
		// maintenance handed its socket over: the notice "starting" answers on it from here on
		starting = appbundle.ServeStarting()
	}
	if d.Bundle == nil {
		return starting
	}
	log.Printf("app builds: starting %s", d)
	err = starting.HandOver()
	if err != nil {
		log.Printf("app builds: handing the socket over: %v", err)
	}
	err = appbundle.Exec(d.Dir, d.Bundle.No, home)
	log.Fatalf("app builds: starting build %d: %v", d.Bundle.No, err)
	return nil
}

// watchAppBuilds checks every 30 s for a newer bundle of this version; when there is a verified
// one it signals switchTo (once)
func watchAppBuilds(db *sql.DB, switchTo chan<- struct{}) {
	buildsDir := os.Getenv(appbundle.EnvBuildsDir)
	if buildsDir == "" {
		return
	}
	base, err := fapps.LoadBase()
	if err != nil {
		log.Printf("app builds: %v", err)
		return
	}
	current := appbundle.Current()
	for range time.Tick(30 * time.Second) {
		local, err := appbundle.LocalBuilder(db, database.DBTypePostgres)
		if err != nil || !local {
			continue
		}
		b, err := appbundle.Latest(db, database.DBTypePostgres, base.Version)
		if err != nil || b == nil || b.No == current {
			continue
		}
		err = appbundle.Verify(appbundle.Dir(buildsDir, b.No), base.Version, b.SHA256)
		if err != nil {
			log.Printf("app builds: build %d: %v", b.No, err)
			continue
		}
		log.Printf("app builds: build %d is ready — restarting with it", b.No)
		switchTo <- struct{}{}
		return
	}
}
