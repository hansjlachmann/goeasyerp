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
	err = copyTree(b.baseSource, src, skipSource)
	if err != nil {
		return "", fmt.Errorf("copying the base source: %w", err)
	}

	if len(list.Apps) > 0 {
		appsDir := filepath.Join(src, "apps")
		for _, app := range list.Apps {
			log.Printf("== %s %s (%s)", app.Name, app.Version, app.Source)
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
		}
		log.Printf("== Merging the apps into the base")
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
		log.Printf("== Generating the table code")
		err = b.run(ctx, src, log, "go", "generate", "./backend/generated/tables")
		if err != nil {
			return "", err
		}
	}

	log.Printf("== Building the server program")
	ldflags := "-X github.com/hansjlachmann/goeasyerp/backend/api.Version=" + list.Base.Version
	err = b.run(ctx, src, log, "go", "build", "-trimpath", "-ldflags="+ldflags, "-o", appbundle.BinaryName, "./cmd/api-server")
	if err != nil {
		return "", err
	}

	// the bundle: written next to its place, then renamed into it
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
	log.Printf("== Built in %s: %s", time.Since(start).Round(time.Second), sha)
	return sha, nil
}

// run runs a command in dir, its output into the log
func (b *builder) run(ctx context.Context, dir string, log *buildLog, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), "CGO_ENABLED=0"), b.env...)
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
