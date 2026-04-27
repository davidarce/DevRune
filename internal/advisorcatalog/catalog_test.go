// SPDX-License-Identifier: MIT

package advisorcatalog

import (
	"regexp"
	"testing"

	"github.com/davidarce/devrune/internal/model"
)

// ---------------------------------------------------------------------------
// ParseCatalogURL
// ---------------------------------------------------------------------------

func TestParseCatalogURL_HappyPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		url        string
		wantScheme string
		wantBody   string
		wantRef    string
	}{
		{
			name:       "local absolute path",
			url:        "local:/abs/path",
			wantScheme: "local",
			wantBody:   "/abs/path",
			wantRef:    "",
		},
		{
			name:       "local relative path",
			url:        "local:./rel/path",
			wantScheme: "local",
			wantBody:   "./rel/path",
			wantRef:    "",
		},
		{
			name:       "github owner/repo no ref",
			url:        "github:owner/repo",
			wantScheme: "github",
			wantBody:   "owner/repo",
			wantRef:    "",
		},
		{
			name:       "github owner/repo@main",
			url:        "github:owner/repo@main",
			wantScheme: "github",
			wantBody:   "owner/repo",
			wantRef:    "main",
		},
		{
			name:       "github owner/repo@abc123",
			url:        "github:owner/repo@abc123",
			wantScheme: "github",
			wantBody:   "owner/repo",
			wantRef:    "abc123",
		},
		{
			name:       "gitlab group/subgroup/repo no ref",
			url:        "gitlab:group/subgroup/repo",
			wantScheme: "gitlab",
			wantBody:   "group/subgroup/repo",
			wantRef:    "",
		},
		{
			name:       "gitlab owner/repo@v1.2.3",
			url:        "gitlab:owner/repo@v1.2.3",
			wantScheme: "gitlab",
			wantBody:   "owner/repo",
			wantRef:    "v1.2.3",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			scheme, body, ref, err := ParseCatalogURL(tc.url)
			if err != nil {
				t.Fatalf("ParseCatalogURL(%q) returned unexpected error: %v", tc.url, err)
			}
			if scheme != tc.wantScheme {
				t.Errorf("scheme: got %q, want %q", scheme, tc.wantScheme)
			}
			if body != tc.wantBody {
				t.Errorf("body: got %q, want %q", body, tc.wantBody)
			}
			if ref != tc.wantRef {
				t.Errorf("ref: got %q, want %q", ref, tc.wantRef)
			}
		})
	}
}

func TestParseCatalogURL_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
	}{
		{name: "empty string", url: ""},
		{name: "no scheme separator", url: "owner/repo"},
		{name: "http scheme unsupported", url: "http:owner/repo"},
		{name: "bitbucket scheme unsupported", url: "bitbucket:owner/repo"},
		{name: "file scheme unsupported", url: "file:/some/path"},
		{name: "github empty body", url: "github:"},
		{name: "github owner only no slash", url: "github:owner"},
		{name: "github trailing @ empty ref", url: "github:owner/repo@"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, _, _, err := ParseCatalogURL(tc.url)
			if err == nil {
				t.Fatalf("ParseCatalogURL(%q) expected an error but got nil", tc.url)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CacheKey
// ---------------------------------------------------------------------------

var hexPattern = regexp.MustCompile(`^[0-9a-f]+$`)

func TestCacheKey(t *testing.T) {
	t.Parallel()

	t.Run("same URL is deterministic", func(t *testing.T) {
		t.Parallel()

		src := model.CatalogSource{URL: "github:acme/catalog@main"}
		key1 := CacheKey(src)
		key2 := CacheKey(src)
		if key1 != key2 {
			t.Errorf("CacheKey(%q) is not deterministic: first=%q second=%q", src.URL, key1, key2)
		}
	})

	t.Run("different URLs produce different keys", func(t *testing.T) {
		t.Parallel()

		srcA := model.CatalogSource{URL: "github:acme/catalog"}
		srcB := model.CatalogSource{URL: "github:other/repo"}
		keyA := CacheKey(srcA)
		keyB := CacheKey(srcB)
		if keyA == keyB {
			t.Errorf("CacheKey produced the same key %q for different URLs %q and %q", keyA, srcA.URL, srcB.URL)
		}
	})

	t.Run("local path returns empty string", func(t *testing.T) {
		t.Parallel()

		src := model.CatalogSource{URL: "local:./any/path"}
		key := CacheKey(src)
		if key != "" {
			t.Errorf("CacheKey(%q): got %q, want empty string", src.URL, key)
		}
	})

	t.Run("key is exactly 10 hex characters for remote URL", func(t *testing.T) {
		t.Parallel()

		src := model.CatalogSource{URL: "github:owner/repo@main"}
		key := CacheKey(src)
		if len(key) != 10 {
			t.Errorf("CacheKey(%q): got len=%d (%q), want 10", src.URL, len(key), key)
		}
		if !hexPattern.MatchString(key) {
			t.Errorf("CacheKey(%q): got %q, want lowercase hex chars only", src.URL, key)
		}
	})

	t.Run("different refs of same repo produce different keys", func(t *testing.T) {
		t.Parallel()

		srcMain := model.CatalogSource{URL: "github:owner/repo@main"}
		srcV2 := model.CatalogSource{URL: "github:owner/repo@v2"}
		keyMain := CacheKey(srcMain)
		keyV2 := CacheKey(srcV2)
		if keyMain == keyV2 {
			t.Errorf("CacheKey: expected different keys for %q and %q, got same: %q", srcMain.URL, srcV2.URL, keyMain)
		}
	})

	t.Run("malformed URL returns empty string", func(t *testing.T) {
		t.Parallel()

		src := model.CatalogSource{URL: "not-a-valid-url"}
		key := CacheKey(src)
		if key != "" {
			t.Errorf("CacheKey(%q): got %q, want empty string for malformed URL", src.URL, key)
		}
	})
}
