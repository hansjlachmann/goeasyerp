package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The cache goes into the builds volume; the image's cache is added once per version, files
// already there kept, Go's trim file left out; an operator's GOCACHE wins
func TestPersistentCache(t *testing.T) {
	builds, image := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(image, "ab", "ab12-a"), "image")
	writeFile(t, filepath.Join(image, "cd", "cd34-d"), "image")
	writeFile(t, filepath.Join(image, "trim.txt"), "1")
	cache := filepath.Join(builds, cacheDir, "go-build")
	writeFile(t, filepath.Join(cache, "cd", "cd34-d"), "volume")
	noEnv := func(string) string { return "" }

	env, err := persistentCache(builds, "1.0.0", image, noEnv)
	if err != nil {
		t.Fatal(err)
	}
	if len(env) != 1 || env[0] != "GOCACHE="+cache {
		t.Fatalf("env = %v", env)
	}
	read := func(rel string) string {
		data, _ := os.ReadFile(filepath.Join(cache, rel))
		return string(data)
	}
	if read("ab/ab12-a") != "image" || read("cd/cd34-d") != "volume" || read("trim.txt") != "" {
		t.Errorf("merged cache: %q %q %q", read("ab/ab12-a"), read("cd/cd34-d"), read("trim.txt"))
	}

	// the same version again: the image's cache is not walked again
	writeFile(t, filepath.Join(image, "ef", "ef56-a"), "image")
	if _, err := persistentCache(builds, "1.0.0", image, noEnv); err != nil {
		t.Fatal(err)
	}
	if read("ef/ef56-a") != "" {
		t.Error("the image's cache was added again for the same version")
	}
	// a new version adds it, and only the new version's marker stays
	if _, err := persistentCache(builds, "1.1.0", image, noEnv); err != nil {
		t.Fatal(err)
	}
	if read("ef/ef56-a") != "image" {
		t.Error("the new version's image cache was not added")
	}
	markers, _ := filepath.Glob(filepath.Join(builds, cacheDir, "seeded-*"))
	if len(markers) != 1 || filepath.Base(markers[0]) != "seeded-1.1.0" {
		t.Errorf("markers = %v", markers)
	}
	// no image cache at all
	if _, err := persistentCache(t.TempDir(), "1.0.0", filepath.Join(image, "missing"), noEnv); err != nil {
		t.Errorf("missing image cache: %v", err)
	}

	env, err = persistentCache(builds, "1.1.0", image, func(k string) string {
		if k == "GOCACHE" {
			return "/elsewhere"
		}
		return ""
	})
	if err != nil || env != nil {
		t.Errorf("operator's GOCACHE: %v %v", env, err)
	}
}

// Each step's time goes into the log, all of them into the summary
func TestStepTimer(t *testing.T) {
	log := &buildLog{}
	steps := &stepTimer{log: log}
	steps.start("copy", "Copying")
	steps.start("compile", "Building")
	steps.stop()
	steps.stop()
	text := string(log.Bytes())
	if !strings.Contains(text, "== Copying") || !strings.Contains(text, "(copy took ") || !strings.Contains(text, "(compile took ") {
		t.Errorf("log:\n%s", text)
	}
	if got := steps.summary(); !strings.HasPrefix(got, "copy ") || !strings.Contains(got, ", compile ") {
		t.Errorf("summary = %q", got)
	}
	if roundDuration(1234*time.Millisecond) != 1200*time.Millisecond || roundDuration(83400*time.Millisecond) != 83*time.Second {
		t.Error("roundDuration")
	}
}

// The builder image fills its cache with the flags the builder compiles with; with others
// nothing it filled would be used
func TestDockerfileFillsTheBuildersCache(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	args, _ := lowMemoryBuild(func(string) string { return "" })
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, "go build") || !strings.Contains(line, "./cmd/api-server") || !strings.Contains(line, "/dev/null") {
			continue
		}
		for _, flag := range append([]string{"-trimpath"}, args...) {
			if strings.HasPrefix(flag, "-p=") {
				continue // parallel compiles do not change what is cached
			}
			if !strings.Contains(line, flag) {
				t.Errorf("the Dockerfile fills the cache without %s: %s", flag, strings.TrimSpace(line))
			}
		}
		return
	}
	t.Fatal("the Dockerfile's app-builder stage does not fill the cache with ./cmd/api-server")
}
