package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The Go build cache on the builds volume: the builder container's own cache (the image's,
// filled when the image was built) is lost whenever the container starts again, and a build
// after that compiles everything the image did not. The cache in the builds volume stays, so a
// build after a restart reuses what the builds before compiled. When the builder starts with a
// version it has not seen, the image's cache is added to it (Go's cache files are named by
// their content, so files already there are kept). Go trims the cache itself: an entry not
// used for five days goes at the end of a build, so it holds what recent builds used.

// cacheDir: the folder of the cache in the builds volume (the bundles are folders named by
// their build number; the server and the builder find them by number, never by listing)
const cacheDir = ".cache"

// persistentCache prepares the cache in the builds volume and returns the environment the go
// commands of a build take (GOCACHE). A GOCACHE the operator set is kept as it is (none
// returned). imageCache is the image's own cache ("" when it has none).
func persistentCache(buildsDir, version, imageCache string, getenv func(string) string) ([]string, error) {
	if getenv("GOCACHE") != "" {
		return nil, nil
	}
	dir := filepath.Join(buildsDir, cacheDir, "go-build")
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		return nil, err
	}
	marker := filepath.Join(buildsDir, cacheDir, "seeded-"+version)
	_, err = os.Stat(marker)
	if errors.Is(err, os.ErrNotExist) && imageCache != "" && imageCache != dir {
		err = mergeCache(imageCache, dir)
		if err != nil {
			return nil, fmt.Errorf("adding the image's build cache: %w", err)
		}
		olds, _ := filepath.Glob(filepath.Join(buildsDir, cacheDir, "seeded-*"))
		for _, old := range olds {
			_ = os.Remove(old)
		}
		err = os.WriteFile(marker, nil, 0o644)
	}
	if err != nil {
		return nil, err
	}
	return []string{"GOCACHE=" + dir}, nil
}

// mergeCache copies the cache files of from that to does not have (each written under another
// name and renamed, so a build never reads half a file); Go's own trim bookkeeping is left out
func mergeCache(from, to string) error {
	return filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == from && errors.Is(err, os.ErrNotExist) {
				return filepath.SkipAll
			}
			return err
		}
		if !d.Type().IsRegular() || d.Name() == "trim.txt" {
			return nil
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		_, err = os.Stat(target)
		if err == nil {
			return nil
		}
		tmp := fmt.Sprintf("%s.tmp-%d", target, os.Getpid())
		err = copyFile(path, tmp)
		if err != nil {
			return err
		}
		return os.Rename(tmp, target)
	})
}

// goEnvCache: the go command's own cache folder (the image's), "" when it has none
func goEnvCache() string {
	out, err := exec.Command("go", "env", "GOCACHE").Output()
	if err != nil {
		return ""
	}
	dir := strings.TrimSpace(string(out))
	if dir == "off" {
		return ""
	}
	return dir
}
