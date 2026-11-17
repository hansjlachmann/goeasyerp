package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
)

// An app's texts are in translations/<language>/*.yaml of its directory, written like the base
// application's (tables.yaml: tables.<table>.fields.<field>, pages.yaml, common.yaml …). They are
// copied next to the base's as files of their own (<prefix>-<file>.yaml) — the server reads every
// file of a language into one set of keys — and an app only adds texts: a key the base
// application or another app already has is refused, as is a language the base has not.

// mergeAppTranslations copies the base translations and adds the apps' texts. Without apps that
// have translations nothing is needed; an app with translations needs the base translations'
// paths.
func mergeAppTranslations(ordered []*apps.App, dirOf map[string]string, opts options) error {
	var withTexts []*apps.App
	for _, ext := range ordered {
		info, err := os.Stat(filepath.Join(dirOf[ext.ID], "translations"))
		if err == nil && info.IsDir() {
			withTexts = append(withTexts, ext)
		}
	}
	if opts.translations == "" || opts.translationsOutput == "" {
		if len(withTexts) > 0 {
			return fmt.Errorf("%s has translations: give --translations (the base translations) and --translations-output", withTexts[0].Name)
		}
		return nil
	}
	err := copyDir(opts.translations, opts.translationsOutput, opts.verbose)
	if err != nil {
		return fmt.Errorf("failed to copy the base translations: %w", err)
	}
	keys, err := translationKeys(opts.translationsOutput)
	if err != nil {
		return err
	}
	for _, ext := range withTexts {
		err = addAppTranslations(ext, filepath.Join(dirOf[ext.ID], "translations"), opts.translationsOutput, keys)
		if err != nil {
			return fmt.Errorf("%s: %w", ext.Name, err)
		}
	}
	return nil
}

// translationKeys: the keys of every language (language → key → the file that has it)
func translationKeys(dir string) (map[string]map[string]string, error) {
	languages, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	keys := map[string]map[string]string{}
	for _, l := range languages {
		if !l.IsDir() {
			continue
		}
		keys[l.Name()] = map[string]string{}
		files, err := filepath.Glob(filepath.Join(dir, l.Name(), "*.yaml"))
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			flat, err := flatTranslations(f)
			if err != nil {
				return nil, err
			}
			for k := range flat {
				keys[l.Name()][k] = filepath.Base(f)
			}
		}
	}
	return keys, nil
}

// addAppTranslations adds one app's texts: its files as <prefix>-<file>.yaml, refused when a
// key is there already or the language is not the base application's
func addAppTranslations(ext *apps.App, dir, out string, keys map[string]map[string]string) error {
	languages, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, l := range languages {
		if !l.IsDir() {
			continue
		}
		lang := l.Name()
		known, ok := keys[lang]
		if !ok {
			return fmt.Errorf("translations/%s: %s is not a language of the base application", lang, lang)
		}
		files, err := filepath.Glob(filepath.Join(dir, lang, "*.yaml"))
		if err != nil {
			return err
		}
		for _, f := range files {
			flat, err := flatTranslations(f)
			if err != nil {
				return err
			}
			name := ext.Prefix + "-" + filepath.Base(f)
			var taken []string
			for k := range flat {
				if other, used := known[k]; used {
					taken = append(taken, fmt.Sprintf("%s (in %s)", k, other))
				}
			}
			if len(taken) > 0 {
				sort.Strings(taken)
				return fmt.Errorf("translations/%s/%s: an app adds texts, it never changes them — already there: %s", lang, filepath.Base(f), strings.Join(taken, ", "))
			}
			for k := range flat {
				known[k] = name
			}
			err = copyFile(f, filepath.Join(out, lang, name))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// flatTranslations reads a translation file as its keys (a.b.c → text)
func flatTranslations(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tree map[string]interface{}
	err = yaml.Unmarshal(data, &tree)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	flat := map[string]string{}
	var walk func(prefix string, m map[string]interface{})
	walk = func(prefix string, m map[string]interface{}) {
		for k, v := range m {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			sub, isMap := v.(map[string]interface{})
			if isMap {
				walk(key, sub)
			} else {
				flat[key] = fmt.Sprint(v)
			}
		}
	}
	walk("", tree)
	return flat, nil
}
