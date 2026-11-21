// Command app-builder is the installation's own builder of the extension apps (App Build Setup:
// Builds By = Local Builder). It runs next to the server — a service of the compose file, a
// Deployment on Kubernetes — from the builder image (goeasyerp/builder:<version>: the Go
// toolchain and the base release's source), connects to the server's database (the same DB_*
// environment) and waits for build requests (App Build, Requested by Build Apps). It builds
// each into a bundle in the builds volume (APP_BUILDS_DIR, shared with the server), records the
// result and its log, and the server switches to the bundle by itself. It needs no Docker
// socket, registry or cluster rights: the database, the volume, and network access to the
// apps' Git repositories and the Go module proxy.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"crypto/sha256"
	"encoding/hex"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/appbundle"
	fapps "github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/themes"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// keepBundles: the successful bundles kept (the newest; older ones are Superseded and removed)
const keepBundles = 3

// buildTimeout: the longest a build may take (a Raspberry Pi builds in minutes)
const buildTimeout = 60 * time.Minute

func main() {
	config, ok := database.BuilderConfigFromEnv()
	if !ok {
		log.Fatal("app-builder needs the server's PostgreSQL database (DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME)")
	}
	buildsDir := os.Getenv(appbundle.EnvBuildsDir)
	if buildsDir == "" {
		log.Fatalf("app-builder needs the builds volume (%s)", appbundle.EnvBuildsDir)
	}
	baseSource := env("BASE_SOURCE", "/src")
	base, err := fapps.Load(filepath.Join(baseSource, "backend", "definitions", "application.yaml"))
	if err != nil {
		log.Fatalf("the base source in %s: %v", baseSource, err)
	}
	db, err := sql.Open("postgres", config.ConnectionString())
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	token := os.Getenv("APPS_TOKEN")
	b := &builder{
		baseSource: baseSource,
		buildsDir:  buildsDir,
		workDir:    env("WORK_DIR", os.TempDir()),
		fetch:      &sourceFetcher{db: db, token: token},
	}
	cacheEnv, err := persistentCache(buildsDir, base.Version, goEnvCache(), os.Getenv)
	if err != nil {
		log.Printf("the build cache in %s: %v (builds use the image's cache)", buildsDir, err)
	}
	b.env = append(b.env, cacheEnv...)
	w := &worker{db: db, b: b, version: base.Version, secrets: []string{token}}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	log.Printf("app-builder %s: waiting for build requests (builds in %s)", base.Version, buildsDir)
	w.run(ctx, 10*time.Second)
}

func env(key, def string) string {
	value := os.Getenv(key)
	if value == "" {
		return def
	}
	return value
}

// worker takes the build requests from the database
type worker struct {
	db      *sql.DB
	b       *builder
	version string // the base version this builder builds
	secrets []string
}

func (w *worker) source() ftables.Source {
	return ftables.DBSource(w.db, "", database.DBTypePostgres)
}

// run waits for requests until ctx ends
func (w *worker) run(ctx context.Context, poll time.Duration) {
	w.recover()
	for {
		for {
			no, err := w.claim()
			if err != nil {
				log.Printf("claiming a build request: %v", err)
				break
			}
			if no == 0 {
				break
			}
			w.buildOne(ctx, no)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(poll):
		}
	}
}

// recover: builds left Running by a builder that stopped fail, and unfinished bundles go
func (w *worker) recover() {
	_, err := w.db.Exec(`UPDATE "App_Build" SET status = $1, error_message = $2 WHERE status = $3`,
		int(gtables.AppBuild_Status.Failed), "the builder stopped during the build", int(gtables.AppBuild_Status.Running))
	if err != nil && !strings.Contains(err.Error(), "does not exist") {
		log.Printf("recovering builds: %v", err)
	}
	entries, _ := os.ReadDir(w.b.buildsDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			_ = os.RemoveAll(filepath.Join(w.b.buildsDir, e.Name()))
		}
	}
}

// claim takes the oldest request for this builder's version (several builders never take the
// same one) and marks it Running; 0 when there is none. A request for another version waits for
// a builder of that version: during an upgrade the new server requests its build while the old
// builder still runs, and the old builder must not take it (it cannot build it).
func (w *worker) claim() (int, error) {
	var no int
	err := w.db.QueryRow(`UPDATE "App_Build" SET status = $1 WHERE entry_no = (
		SELECT entry_no FROM "App_Build" WHERE status = $2 AND base_version = $3
		ORDER BY entry_no LIMIT 1 FOR UPDATE SKIP LOCKED)
		RETURNING entry_no`, int(gtables.AppBuild_Status.Running), int(gtables.AppBuild_Status.Requested), w.version).Scan(&no)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil && strings.Contains(err.Error(), "does not exist") {
		return 0, nil // the server has not created the table yet
	}
	return no, err
}

// buildOne builds a claimed request and records the result
func (w *worker) buildOne(ctx context.Context, no int) {
	rec := gtables.NewAppBuild(w.source())
	if !rec.Get(no) {
		log.Printf("build %d: not found", no)
		return
	}
	rec.Started_at = types.NewDateTimeFromTime(time.Now())
	_ = rec.Modify(false)
	blog := &buildLog{secrets: w.secrets}
	sha, err := w.buildRequest(ctx, no, rec, blog)
	rec.Finished_at = types.NewDateTimeFromTime(time.Now())
	if err != nil {
		blog.Printf("== Failed: %v", err)
		rec.Status = gtables.AppBuild_Status.Failed
		rec.Error_message = types.NewText(truncate(redact(err.Error(), w.secrets), 250))
		log.Printf("build %d failed: %v", no, redact(err.Error(), w.secrets))
	} else {
		rec.Status = gtables.AppBuild_Status.Succeeded
		rec.Binary_sha256 = types.NewText(sha)
		rec.Bundle_path = types.NewText(fmt.Sprint(no))
		log.Printf("build %d succeeded", no)
	}
	rec.Log = blog.Bytes()
	err = rec.ModifyOrError(false)
	if err != nil {
		log.Printf("build %d: recording the result: %v", no, err)
		return
	}
	if rec.Status == gtables.AppBuild_Status.Succeeded {
		w.prune()
	}
}

func (w *worker) buildRequest(ctx context.Context, no int, rec *gtables.AppBuild, blog *buildLog) (string, error) {
	if rec.Base_version.String() != w.version {
		return "", fmt.Errorf("the request is for version %s, this builder builds version %s (its image must have the server's version)", rec.Base_version, w.version)
	}
	var list fapps.BuildList
	err := json.Unmarshal(rec.Build_list, &list)
	if err != nil {
		return "", fmt.Errorf("the build list: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, buildTimeout)
	defer cancel()
	return w.b.build(ctx, no, list, blog)
}

// prune keeps the newest successful bundles; older ones are Superseded and their folders go
func (w *worker) prune() {
	rec := gtables.NewAppBuild(w.source())
	rec.SetRange("status", gtables.AppBuild_Status.Succeeded)
	var nos []int
	for found := rec.FindSet(); found; found = rec.Next() {
		nos = append(nos, rec.Entry_no)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(nos)))
	for i, no := range nos {
		if i < keepBundles {
			continue
		}
		old := gtables.NewAppBuild(w.source())
		if !old.Get(no) {
			continue
		}
		old.Status = gtables.AppBuild_Status.Superseded
		err := old.ModifyOrError(false)
		if err != nil {
			log.Printf("build %d: %v", no, err)
			continue
		}
		_ = os.RemoveAll(appbundle.Dir(w.b.buildsDir, no))
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func redact(s string, secrets []string) string {
	for _, secret := range secrets {
		if secret != "" {
			s = strings.ReplaceAll(s, secret, "***")
		}
	}
	return s
}

// sourceFetcher gets apps from their Git repositories (git clone of the tag) and from the
// uploaded packages in the database
type sourceFetcher struct {
	db    *sql.DB
	token string // APPS_TOKEN: read access to private app repositories on GitHub
}

func (f *sourceFetcher) Fetch(ctx context.Context, app fapps.BuildApp, dest string, log io.Writer) error {
	switch app.Source {
	case fapps.SourcePackage:
		return f.fetchPackage(app, dest)
	case fapps.SourceGit, "":
		return f.fetchGit(ctx, app, dest, log)
	}
	return fmt.Errorf("unknown source %q", app.Source)
}

// fetchPackage unpacks the uploaded package (its SHA-256 the build list's)
func (f *sourceFetcher) fetchPackage(app fapps.BuildApp, dest string) error {
	rec := gtables.NewAppPackage(ftables.DBSource(f.db, "", database.DBTypePostgres))
	if !rec.Get(map[string]interface{}{"app_id": app.ID, "version": app.Version}) {
		return errors.New("its uploaded package is not there any more")
	}
	return unpack(rec.Package, app.PackageSHA256, dest)
}

// unpack checks a package against its SHA-256 and writes its files into dest
func unpack(data []byte, want, dest string) error {
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != want {
		return errors.New("its package is not the one the build was requested with")
	}
	pkg, err := fapps.ReadPackage(data)
	if err != nil {
		return err
	}
	return pkg.Extract(dest)
}

// fetchGit takes the app's repository at its commit (no history) and copies its folder: the
// commit recorded when the app was installed, after checking that its tag still points to it — a
// moved tag would bring other code into the program unseen. An app installed before commits were
// recorded is cloned at its tag, with a warning.
func (f *sourceFetcher) fetchGit(ctx context.Context, app fapps.BuildApp, dest string, log io.Writer) error {
	repo, err := url.Parse(app.Repository)
	if err != nil || repo.Scheme != "https" || repo.Host == "" || repo.User != nil {
		return fmt.Errorf("%q is not an https repository address", app.Repository)
	}
	if !themes.TagRe.MatchString(app.Ref) {
		return fmt.Errorf("%q is not a version tag", app.Ref)
	}
	cloneURL := repo.String()
	if f.token != "" && repo.Host == "github.com" {
		withToken := *repo
		withToken.User = url.UserPassword("x-access-token", f.token)
		cloneURL = withToken.String()
	}
	git := func(out io.Writer, args ...string) error {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		cmd.Stdout = out
		cmd.Stderr = log
		return cmd.Run()
	}
	clone := dest + ".clone"
	defer func() { _ = os.RemoveAll(clone) }()
	if app.Commit == "" {
		_, _ = fmt.Fprintf(log, "   WARNING: %s has no recorded commit (installed before commits were recorded): built at the tag %s as it is now — install it again (Install App, the same version) to pin it\n", app.Name, app.Ref)
		err = git(log, "-c", "advice.detachedHead=false", "clone", "--quiet", "--depth", "1", "--branch", app.Ref, "--", cloneURL, clone)
		if err != nil {
			return fmt.Errorf("git clone %s %s: %w", app.Repository, app.Ref, err)
		}
	} else {
		if !commitRe.MatchString(app.Commit) {
			return fmt.Errorf("%q is not a commit", app.Commit)
		}
		var refs strings.Builder
		tagRef := "refs/tags/" + app.Ref
		err = git(&refs, "ls-remote", "--", cloneURL, tagRef, tagRef+"^{}")
		if err != nil {
			return fmt.Errorf("git ls-remote %s: %w", app.Repository, err)
		}
		now := tagCommit(refs.String(), tagRef)
		switch {
		case now == "":
			return fmt.Errorf("the tag %s is not in %s any more", app.Ref, app.Repository)
		case now != app.Commit:
			return fmt.Errorf("the tag %s of %s now points to commit %s, not to %s, which was installed — refused: other code under the same version (the publisher must release a new version)", app.Ref, app.Repository, now, app.Commit)
		}
		_, _ = fmt.Fprintf(log, "   commit %s (tag %s)\n", app.Commit, app.Ref)
		for _, args := range [][]string{
			{"init", "--quiet", clone},
			{"-C", clone, "fetch", "--quiet", "--depth", "1", "--", cloneURL, app.Commit},
			{"-C", clone, "-c", "advice.detachedHead=false", "checkout", "--quiet", "FETCH_HEAD"},
		} {
			err = git(log, args...)
			if err != nil {
				return fmt.Errorf("git %s %s %s: %w", args[len(args)-1], app.Repository, app.Commit, err)
			}
		}
		var head strings.Builder
		err = git(&head, "-C", clone, "rev-parse", "HEAD")
		if err != nil || strings.TrimSpace(head.String()) != app.Commit {
			return fmt.Errorf("%s: fetched %q, not the commit %s", app.Repository, strings.TrimSpace(head.String()), app.Commit)
		}
	}
	folder := filepath.Clean(filepath.FromSlash(app.Path))
	if filepath.IsAbs(folder) || folder == ".." || strings.HasPrefix(folder, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%q is not a folder of the repository", app.Path)
	}
	return copyTree(filepath.Join(clone, folder), dest, func(rel string, d os.DirEntry) bool { return d.Name() == ".git" })
}

// commitRe: a full commit id
var commitRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// tagCommit is the commit a tag points to in git ls-remote's output for tagRef and tagRef^{}:
// an annotated tag's peeled line (^{}: its commit), else the lightweight tag's own; "" when the
// tag is not there
func tagCommit(lsRemote, tagRef string) string {
	plain, peeled := "", ""
	for _, line := range strings.Split(lsRemote, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		switch fields[1] {
		case tagRef:
			plain = fields[0]
		case tagRef + "^{}":
			peeled = fields[0]
		}
	}
	if peeled != "" {
		return peeled
	}
	return plain
}
