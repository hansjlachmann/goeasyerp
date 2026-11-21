package themes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Limits of a theme download (security: a repository is untrusted input)
const (
	MaxTreeEntries     = 2000
	MaxTreeBytes       = 4 << 20
	MaxImageBytes      = 6 << 20
	MaxBackgrounds     = 5
	MaxDownloadBytes   = 30 << 20
	requestTimeout     = 15 * time.Second
	InstallTimeout     = 90 * time.Second
	defaultGitHubAPI   = "https://api.github.com"
	defaultGitHubFiles = "https://raw.githubusercontent.com"
)

// Errors of a theme download
var (
	ErrRepoURL       = errors.New("not a GitHub repository URL (https://github.com/owner/repo)")
	ErrRateLimited   = errors.New("GitHub rate limit reached, try again later (or set GITHUB_TOKEN)")
	ErrNotFound      = errors.New("repository, branch or file not found on GitHub")
	ErrTooLarge      = errors.New("theme file too large")
	ErrTreeTooLarge  = errors.New("repository has too many files to be a theme")
	ErrNoThemeColors = errors.New("repository has no colors.toml or alacritty.toml")
)

// Repo is a GitHub repository (and optionally a branch/tag) holding a theme.
type Repo struct {
	Owner, Name, Ref string
}

// URL returns the repository's web address.
func (r Repo) URL() string {
	u := "https://github.com/" + r.Owner + "/" + r.Name
	if r.Ref != "" {
		u += "/tree/" + r.Ref
	}
	return u
}

var (
	repoURLRe = regexp.MustCompile(`^https://github\.com/([A-Za-z0-9](?:[A-Za-z0-9-]{0,38}))/([A-Za-z0-9._-]{1,100}?)(?:\.git)?(?:/tree/([A-Za-z0-9._/-]{1,100}))?/?$`)
	shaRe     = regexp.MustCompile(`^[0-9a-f]{40}$`)
	bgNameRe  = regexp.MustCompile(`^backgrounds/[A-Za-z0-9 _.()+-]{1,100}\.(?i:jpe?g|png|webp)$`)
)

// ParseRepoURL accepts only https://github.com/<owner>/<repo>[.git][/tree/<ref>]. The
// backend builds every request URL itself from owner, repo and ref — the user never
// chooses a host.
func ParseRepoURL(s string) (Repo, error) {
	m := repoURLRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil || m[2] == "." || m[2] == ".." || strings.Contains(m[3], "..") || strings.HasPrefix(m[3], "/") {
		return Repo{}, ErrRepoURL
	}
	return Repo{Owner: m[1], Name: m[2], Ref: strings.TrimSuffix(m[3], "/")}, nil
}

// Download is what a theme repository provides: the theme file, the images (raw, not yet
// checked) and the commit everything was read from.
type Download struct {
	Repo        Repo
	CommitSHA   string
	ColorsName  string // colors.toml or alacritty.toml
	Colors      []byte
	Preview     []byte // preview.png / theme.png, nil when the repository has none
	Backgrounds [][]byte
}

// Fetcher downloads theme repositories from GitHub. API and Files are the base URLs
// (tests point them to a local server); Token is an optional GitHub token, sent only to
// the API host.
type Fetcher struct {
	API, Files string
	Token      string
	Client     *http.Client
}

// NewFetcher returns a fetcher for github.com; GITHUB_TOKEN raises the API rate limit.
func NewFetcher() *Fetcher {
	return newFetcher(defaultGitHubAPI, defaultGitHubFiles, os.Getenv("GITHUB_TOKEN"))
}

// NewFetcherWithBase returns a fetcher for other base URLs (tests: a local fake GitHub).
func NewFetcherWithBase(api, files, token string) *Fetcher { return newFetcher(api, files, token) }

func newFetcher(api, files, token string) *Fetcher {
	f := &Fetcher{API: api, Files: files, Token: token}
	allowed := map[string]bool{hostOf(api): true, hostOf(files): true}
	f.Client = &http.Client{
		Timeout: requestTimeout,
		// Redirects only between the two GitHub hosts, never to another site (SSRF)
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 || !allowed[req.URL.Host] {
				return fmt.Errorf("redirect to %s refused", req.URL.Host)
			}
			if req.URL.Host != hostOf(f.API) {
				req.Header.Del("Authorization")
			}
			return nil
		},
		// A proxy only as the administrator configured it (HTTPS_PROXY); hosts come from
		// the fetcher, never from the user
		Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, ResponseHeaderTimeout: requestTimeout, MaxResponseHeaderBytes: 64 << 10},
	}
	return f
}

func hostOf(base string) string {
	u, err := url.Parse(base)
	if err != nil {
		return ""
	}
	return u.Host
}

// Fetch downloads the theme files of repo, all pinned to one commit.
func (f *Fetcher) Fetch(ctx context.Context, repo Repo) (*Download, error) {
	ctx, cancel := context.WithTimeout(ctx, InstallTimeout)
	defer cancel()

	ref := repo.Ref
	if ref == "" {
		ref = "HEAD"
	}
	sha, err := f.get(ctx, f.API, []string{"repos", repo.Owner, repo.Name, "commits", ref}, "application/vnd.github.sha", 64)
	if err != nil {
		return nil, err
	}
	commit := strings.TrimSpace(string(sha))
	if !shaRe.MatchString(commit) {
		return nil, fmt.Errorf("unexpected commit id from GitHub")
	}

	raw, err := f.get(ctx, f.API, []string{"repos", repo.Owner, repo.Name, "git", "trees", commit}, "application/vnd.github+json", MaxTreeBytes, "recursive=1")
	if err != nil {
		return nil, err
	}
	var tree struct {
		Truncated bool `json:"truncated"`
		Tree      []struct {
			Path string `json:"path"`
			Type string `json:"type"`
			Size int64  `json:"size"`
		} `json:"tree"`
	}
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, fmt.Errorf("unexpected file list from GitHub")
	}
	if tree.Truncated || len(tree.Tree) > MaxTreeEntries {
		return nil, ErrTreeTooLarge
	}

	// Only these files are read — never scripts, configs or anything else in the repository
	files := map[string]int64{}
	var backgrounds []string
	for _, e := range tree.Tree {
		if e.Type != "blob" {
			continue
		}
		switch {
		case e.Path == "colors.toml" || e.Path == "alacritty.toml" || e.Path == "preview.png" || e.Path == "theme.png":
			files[e.Path] = e.Size
		case bgNameRe.MatchString(e.Path) && !strings.Contains(e.Path, ".."):
			files[e.Path] = e.Size
			backgrounds = append(backgrounds, e.Path)
		}
	}
	sort.Strings(backgrounds)
	if len(backgrounds) > MaxBackgrounds {
		backgrounds = backgrounds[:MaxBackgrounds]
	}

	d := &Download{Repo: repo, CommitSHA: commit}
	total := int64(0)
	download := func(p string, max int64) ([]byte, error) {
		if files[p] > max {
			return nil, fmt.Errorf("%s: %w", p, ErrTooLarge)
		}
		if total+files[p] > MaxDownloadBytes {
			return nil, fmt.Errorf("theme: %w", ErrTooLarge)
		}
		segs := append([]string{repo.Owner, repo.Name, commit}, strings.Split(p, "/")...)
		b, err := f.get(ctx, f.Files, segs, "", max)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		total += int64(len(b))
		return b, nil
	}

	for _, name := range []string{"colors.toml", "alacritty.toml"} {
		if _, ok := files[name]; ok {
			if d.Colors, err = download(name, MaxThemeFileSize); err != nil {
				return nil, err
			}
			d.ColorsName = name
			break
		}
	}
	if d.Colors == nil {
		return nil, ErrNoThemeColors
	}
	for _, name := range []string{"preview.png", "theme.png"} {
		if _, ok := files[name]; ok {
			if d.Preview, err = download(name, MaxImageBytes); err != nil {
				return nil, err
			}
			break
		}
	}
	for _, p := range backgrounds {
		b, err := download(p, MaxImageBytes)
		if err != nil {
			return nil, err
		}
		d.Backgrounds = append(d.Backgrounds, b)
	}
	return d, nil
}

// get requests base/<escaped segments>[?query] and reads at most max bytes (more is an
// error, whatever Content-Length claims).
func (f *Fetcher) get(ctx context.Context, base string, segs []string, accept string, max int64, query ...string) ([]byte, error) {
	esc := make([]string, len(segs))
	for i, s := range segs {
		esc[i] = url.PathEscape(s)
	}
	u := strings.TrimSuffix(base, "/") + "/" + path.Join(esc...)
	if len(query) > 0 {
		u += "?" + strings.Join(query, "&")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "openerp-themes")
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if f.Token != "" && base == f.API {
		req.Header.Set("Authorization", "Bearer "+f.Token)
	}
	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrNotFound
	case resp.StatusCode == http.StatusTooManyRequests,
		resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0":
		return nil, ErrRateLimited
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("GitHub answered %d", resp.StatusCode)
	}
	if resp.ContentLength > max {
		return nil, ErrTooLarge
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, fmt.Errorf("GitHub download failed")
	}
	if int64(len(b)) > max {
		return nil, ErrTooLarge
	}
	return b, nil
}

// filePathRe: a file path inside a repository (folders and a file name, no ..)
var filePathRe = regexp.MustCompile(`^[A-Za-z0-9._-]+(?:/[A-Za-z0-9._-]+)*$`)

// File downloads one file of a repository at its Ref (a tag, branch or commit) through the
// GitHub API's contents endpoint — the token goes along, so a private repository the token may
// read works too; at most max bytes. Used for an extension app's application.yaml.
func (f *Fetcher) File(ctx context.Context, repo Repo, filePath string, max int64) ([]byte, error) {
	if !filePathRe.MatchString(filePath) || strings.Contains(filePath, "..") {
		return nil, fmt.Errorf("invalid file path %q", filePath)
	}
	segs := append([]string{"repos", repo.Owner, repo.Name, "contents"}, strings.Split(filePath, "/")...)
	var query []string
	if repo.Ref != "" {
		query = append(query, "ref="+url.QueryEscape(repo.Ref))
	}
	return f.get(ctx, f.API, segs, "application/vnd.github.raw", max, query...)
}

// TagRe: a version tag name an app may be installed at (letters, digits, . _ -; no path)
var TagRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

// TagCommit is the commit a tag of repo points to — a lightweight tag's own, an annotated tag
// followed to its commit. ErrNotFound when the repository has no such tag (a branch is not a
// tag: an app is installed at a tag, and pinned to the tag's commit).
func (f *Fetcher) TagCommit(ctx context.Context, repo Repo, tag string) (string, error) {
	if !TagRe.MatchString(tag) {
		return "", fmt.Errorf("invalid tag %q", tag)
	}
	type object struct {
		Object struct {
			Type string `json:"type"`
			SHA  string `json:"sha"`
		} `json:"object"`
	}
	raw, err := f.get(ctx, f.API, []string{"repos", repo.Owner, repo.Name, "git", "ref", "tags", tag}, "application/vnd.github+json", 64<<10)
	if err != nil {
		return "", err
	}
	// an annotated tag is a tag object pointing to the commit (or, rarely, to another tag)
	for i := 0; i < 4; i++ {
		var o object
		err = json.Unmarshal(raw, &o)
		if err != nil || !shaRe.MatchString(o.Object.SHA) {
			return "", fmt.Errorf("unexpected tag from GitHub")
		}
		switch o.Object.Type {
		case "commit":
			return o.Object.SHA, nil
		case "tag":
			raw, err = f.get(ctx, f.API, []string{"repos", repo.Owner, repo.Name, "git", "tags", o.Object.SHA}, "application/vnd.github+json", 64<<10)
			if err != nil {
				return "", err
			}
		default:
			return "", fmt.Errorf("tag %s does not point to a commit", tag)
		}
	}
	return "", fmt.Errorf("tag %s does not point to a commit", tag)
}
