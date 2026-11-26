package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/appbundle"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/appaudit"
	fapps "github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
)

// Building a bundle: the base source (the builder image's copy of the base release) is copied
// to a work folder, the apps of the build list are put into its apps folder — cloned from their
// Git repository at their tag, or unpacked from their uploaded package —, merged into the base
// as the base's Dockerfile does (extmerge, tablegen), and the server program is built; the
// program, the merged definitions and translations and build.json become the bundle.

// fetcher gets an app of the build list into dest (the app's folder: its application.yaml at
// the top)
type fetcher interface {
	Fetch(ctx context.Context, app fapps.BuildApp, dest string, log io.Writer) error
}

// buildLog collects a build's output (at most logMax; the end is kept when it is longer) and
// takes secrets out of it
type buildLog struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	secrets []string
}

const logMax = 1 << 20

func (l *buildLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf.Write(p)
	if l.buf.Len() > 2*logMax {
		keep := l.buf.Bytes()[l.buf.Len()-logMax:]
		l.buf = *bytes.NewBuffer(append([]byte("…\n"), keep...))
	}
	return len(p), nil
}

func (l *buildLog) Printf(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(l, format+"\n", args...)
}

// Bytes: the log, secrets replaced
func (l *buildLog) Bytes() []byte {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := l.buf.Bytes()
	if l.buf.Len() > logMax {
		out = append([]byte("…\n"), out[l.buf.Len()-logMax:]...)
	}
	text := string(out)
	for _, s := range l.secrets {
		if s != "" {
			text = strings.ReplaceAll(text, s, "***")
		}
	}
	return []byte(text)
}

// auditApp runs the app audit on a fetched app with the build list's allowed modules and the
// exceptions approved for its exact source; the report goes into the build's log
func auditApp(app fapps.BuildApp, dir string, allowedModules []string, log *buildLog) error {
	opts := appaudit.Options{AllowedModules: allowedModules}
	for _, e := range app.AuditExceptions {
		opts.Exceptions = append(opts.Exceptions, appaudit.Exception{Rule: e.Rule, Key: e.Key})
	}
	report, err := appaudit.AuditDir(app.Name+" "+app.Version, dir, opts)
	if err != nil {
		return fmt.Errorf("%s %s: the app audit: %w", app.Name, app.Version, err)
	}
	log.Printf("%s   (source %s; app id %s)", strings.TrimRight(report.Text(), "\n"), app.SourceID(), app.ID)
	if !report.Passed() {
		return fmt.Errorf("%s %s did not pass the app audit: %d finding(s) refused (see the report in the log; a SUPER user can approve a finding for this source in App Audit Exceptions)",
			app.Name, app.Version, len(report.Findings)-report.Excepted())
	}
	return nil
}

// builder builds bundles
type builder struct {
	baseSource string // the base release's source
	buildsDir  string // the builds volume
	workDir    string // where builds are made (temporary folders)
	fetch      fetcher
	env        []string // extra environment of the go commands
}

// build builds the bundle of build no from the build list and returns the program's SHA-256
func (b *builder) build(ctx context.Context, no int, list fapps.BuildList, log *buildLog) (string, error) {
	start := time.Now()
	work, err := os.MkdirTemp(b.workDir, fmt.Sprintf("app-build-%d-", no))
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(work) }()
	src := filepath.Join(work, "base")
	log.Printf("== Build %d: base %s, %d app(s)", no, list.Base.Version, len(list.Apps))
	steps := &stepTimer{log: log}
	steps.start("copy", "Copying the base source")
	err = copyTree(b.baseSource, src, skipSource)
	if err != nil {
		return "", fmt.Errorf("copying the base source: %w", err)
	}

	if len(list.Apps) > 0 {
		appsDir := filepath.Join(src, "apps")
		steps.start("fetch", "Fetching the apps")
		for _, app := range list.Apps {
			log.Printf("   %s %s (%s)", app.Name, app.Version, app.Source)
			dest := filepath.Join(appsDir, app.ID)
			err = b.fetch.Fetch(ctx, app, dest, log)
			if err != nil {
				return "", fmt.Errorf("%s %s: %w", app.Name, app.Version, err)
			}
			got, err := fapps.Load(filepath.Join(dest, "application.yaml"))
			if err != nil {
				return "", fmt.Errorf("%s %s: %w", app.Name, app.Version, err)
			}
			if got.ID != app.ID || got.Version != app.Version {
				return "", fmt.Errorf("%s: the source holds %s %s, not version %s of this app", app.Name, got.ID, got.Version, app.Version)
			}
			// the app audit (S55): nothing of an app that fails it is merged or compiled
			err = auditApp(app, dest, list.AllowedGoModules, log)
			if err != nil {
				return "", err
			}
		}
		steps.start("merge", "Merging the apps into the base")
		err = b.run(ctx, src, log, "go", "run", "./tools/extmerge", "--core", "backend/definitions", "--apps", "apps",
			"--output", "merged", "--translations", "translations", "--translations-output", "merged-translations", "--go-root", ".")
		if err != nil {
			return "", err
		}
		for _, swap := range [][2]string{{"merged", "backend/definitions"}, {"merged-translations", "translations"}} {
			err = os.RemoveAll(filepath.Join(src, swap[1]))
			if err != nil {
				return "", err
			}
			err = os.Rename(filepath.Join(src, swap[0]), filepath.Join(src, swap[1]))
			if err != nil {
				return "", err
			}
		}
		steps.start("generate", "Generating the table code")
		err = b.run(ctx, src, log, "go", "generate", "./backend/generated/tables")
		if err != nil {
			return "", err
		}
	}

	steps.start("compile", "Building the server program")
	ldflags := "-X github.com/hansjlachmann/goeasyerp/backend/api.Version=" + list.Base.Version
	args := []string{"build", "-trimpath", "-ldflags=" + ldflags}
	memArgs, memEnv := lowMemoryBuild(os.Getenv)
	args = append(args, memArgs...)
	args = append(args, "-o", appbundle.BinaryName, "./cmd/api-server")
	err = b.runEnv(ctx, src, log, memEnv, "go", args...)
	if err != nil {
		return "", err
	}

	// the bundle: written next to its place, then renamed into it
	steps.start("bundle", "Writing the bundle")
	final := appbundle.Dir(b.buildsDir, no)
	tmp := final + ".tmp"
	_ = os.RemoveAll(tmp)
	err = os.MkdirAll(tmp, 0o755)
	if err != nil {
		return "", err
	}
	for _, f := range []string{appbundle.BinaryName, "go.mod", "LICENSE", "THIRD_PARTY_NOTICES"} {
		err = copyFile(filepath.Join(src, f), filepath.Join(tmp, f))
		// the licence files are optional; the program and the go.mod marker are not
		optional := f != appbundle.BinaryName && f != "go.mod"
		if err != nil && (!errors.Is(err, os.ErrNotExist) || !optional) {
			return "", err
		}
	}
	for _, d := range []string{"backend/definitions", "translations"} {
		err = copyTree(filepath.Join(src, d), filepath.Join(tmp, d), nil)
		if err != nil {
			return "", err
		}
	}
	sha, err := appbundle.FileSHA256(filepath.Join(tmp, appbundle.BinaryName))
	if err != nil {
		return "", err
	}
	err = appbundle.WriteManifest(tmp, &appbundle.Manifest{BuildNo: no, BaseVersion: list.Base.Version, Apps: list.Apps,
		BinarySHA256: sha, BuiltAt: time.Now().UTC().Format(time.RFC3339)})
	if err != nil {
		return "", err
	}
	_ = os.RemoveAll(final)
	err = os.Rename(tmp, final)
	if err != nil {
		return "", err
	}
	steps.stop()
	log.Printf("== Built in %s (%s): %s", time.Since(start).Round(time.Second), steps.summary(), sha)
	return sha, nil
}

// tablesPackage: the generated table code, the one package that needs much memory to compile
const tablesPackage = "github.com/hansjlachmann/goeasyerp/backend/generated/tables"

// lowMemoryBuild: the go build flags and environment that keep the compile of the generated
// table code within a small server's memory (one package of some 40 MB source: about 8 GB with
// Go's defaults, about 4.6 GB with one compile thread and GOGC=25). What the operator sets in
// the builder's environment wins: GOFLAGS with -gcflags or -p, and GOGC.
func lowMemoryBuild(getenv func(string) string) (args, env []string) {
	goflags := getenv("GOFLAGS")
	if !strings.Contains(goflags, "-gcflags") {
		args = append(args, "-gcflags="+tablesPackage+"=-c=1")
	}
	if !strings.Contains(goflags, "-p=") {
		args = append(args, fmt.Sprintf("-p=%d", parallelCompiles))
	}
	if getenv("GOGC") == "" {
		env = append(env, "GOGC=25")
	}
	return args, env
}

// run runs a command in dir, its output into the log
func (b *builder) run(ctx context.Context, dir string, log *buildLog, name string, args ...string) error {
	return b.runEnv(ctx, dir, log, nil, name, args...)
}

// runEnv runs a command in dir with extra environment, its output into the log
func (b *builder) runEnv(ctx context.Context, dir string, log *buildLog, env []string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(append(append(os.Environ(), "CGO_ENABLED=0"), b.env...), env...)
	cmd.Stdout = log
	cmd.Stderr = log
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args[:min(len(args), 2)], " "), err)
	}
	return nil
}

// skipSource: what of the base source a build does not need
func skipSource(rel string, d os.DirEntry) bool {
	name := d.Name()
	if d.IsDir() {
		return name == ".git" || name == "node_modules" || rel == "frontend" || rel == "apps"
	}
	return strings.HasSuffix(name, ".db") || name == "go.work" || name == "go.work.sum" || name == appbundle.BinaryName && rel == name
}

// copyTree copies a folder tree — its folders and regular files, without what skip says
func copyTree(from, to string, skip func(rel string, d os.DirEntry) bool) error {
	return filepath.WalkDir(from, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		if rel != "." && skip != nil && skip(filepath.ToSlash(rel), d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		return copyFile(path, target)
	})
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Dir(to), 0o755)
	if err != nil {
		return err
	}
	out, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}

// parallelCompiles: the packages go build compiles at a time (Go's default is one per CPU);
// the other packages of the server need far less than the table code, but four at once on top
// of it would not fit a small server
const parallelCompiles = 2

// stepTimer times the steps of a build in its log: each step's time when it ends, and all of
// them in the last line, so the log tells where a build spends its time
type stepTimer struct {
	log     *buildLog
	name    string
	started time.Time
	done    []string
}

// start ends the step running and starts the next
func (s *stepTimer) start(name, heading string) {
	s.stop()
	s.log.Printf("== %s", heading)
	s.name, s.started = name, time.Now()
}

// stop ends the step running
func (s *stepTimer) stop() {
	if s.name == "" {
		return
	}
	took := roundDuration(time.Since(s.started))
	s.log.Printf("   (%s took %s)", s.name, took)
	s.done = append(s.done, s.name+" "+took.String())
	s.name = ""
}

// summary: the steps ended and their times ("copy 2s, merge 9s, …")
func (s *stepTimer) summary() string {
	return strings.Join(s.done, ", ")
}

// roundDuration: whole seconds, tenths below ten seconds
func roundDuration(d time.Duration) time.Duration {
	if d < 10*time.Second {
		return d.Round(100 * time.Millisecond)
	}
	return d.Round(time.Second)
}
