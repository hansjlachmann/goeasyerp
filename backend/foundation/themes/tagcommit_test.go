package themes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A tag resolves to its commit: a lightweight tag directly, an annotated one through its tag
// object; a branch name or a missing tag is not found, a bad name is refused before asking
func TestTagCommit(t *testing.T) {
	const commit = "1111111111111111111111111111111111111111"
	const tagObject = "2222222222222222222222222222222222222222"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/loyalty/git/ref/tags/v1.0.0":
			_, _ = w.Write([]byte(`{"ref":"refs/tags/v1.0.0","object":{"type":"commit","sha":"` + commit + `"}}`))
		case "/repos/acme/loyalty/git/ref/tags/v2.0.0":
			_, _ = w.Write([]byte(`{"ref":"refs/tags/v2.0.0","object":{"type":"tag","sha":"` + tagObject + `"}}`))
		case "/repos/acme/loyalty/git/tags/" + tagObject:
			_, _ = w.Write([]byte(`{"tag":"v2.0.0","object":{"type":"commit","sha":"` + commit + `"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	f := NewFetcherWithBase(srv.URL, srv.URL, "")
	repo, err := ParseRepoURL("https://github.com/acme/loyalty")
	if err != nil {
		t.Fatal(err)
	}
	for _, tag := range []string{"v1.0.0", "v2.0.0"} {
		got, err := f.TagCommit(context.Background(), repo, tag)
		if err != nil || got != commit {
			t.Errorf("%s: %q, %v", tag, got, err)
		}
	}
	if _, err := f.TagCommit(context.Background(), repo, "main"); err != ErrNotFound {
		t.Errorf("a branch: %v, want ErrNotFound", err)
	}
	for _, bad := range []string{"", "../x", "a/b", "-x"} {
		if _, err := f.TagCommit(context.Background(), repo, bad); err == nil || err == ErrNotFound {
			t.Errorf("tag %q: %v", bad, err)
		}
	}
}
