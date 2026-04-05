// SPDX-License-Identifier: MIT

package recommend

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/davidarce/devrune/internal/detect"
)

// DefaultCacheTTL is the default time-to-live for cached recommendations.
const DefaultCacheTTL = 1 * time.Hour

// CacheDirOverride allows tests to redirect the cache to a temp directory.
// Empty string (default) uses the standard ~/.cache/devrune/recommend/ path.
var CacheDirOverride string

// cacheDir returns the directory for recommendation cache files.
// Uses ~/.cache/devrune/recommend/ (same base as package cache).
func cacheDir() string {
	if CacheDirOverride != "" {
		return CacheDirOverride
	}
	base, err := os.UserCacheDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "devrune", "recommend")
}

// cacheEntry is the on-disk format for a cached recommendation result.
type cacheEntry struct {
	Timestamp       time.Time       `json:"timestamp"`
	Recommendations []Recommendation `json:"recommendations"`
}

// cacheKey computes a SHA256 hash of the project profile + catalog items.
// If either changes, the key changes and the cache misses.
func cacheKey(profile detect.ProjectProfile, catalog []CatalogItem) string {
	type payload struct {
		Profile detect.ProjectProfile `json:"profile"`
		Catalog []CatalogItem         `json:"catalog"`
	}
	data, _ := json.Marshal(payload{Profile: profile, Catalog: catalog})
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// QuickCacheKey computes a cache key from working directory + catalog item names only.
// This avoids running detect.Analyze for the cache lookup — much faster.
// Uses only item names+kinds (sorted) for stability across runs.
func QuickCacheKey(dir string, catalog []CatalogItem) string {
	// Resolve symlinks for consistent paths (macOS /private/var vs /var).
	resolved, err := filepath.EvalSymlinks(dir)
	if err == nil {
		dir = resolved
	}

	// Use sorted name+kind pairs for a deterministic key regardless of order.
	keys := make([]string, len(catalog))
	for i, c := range catalog {
		keys[i] = c.Kind + ":" + c.Name
	}
	sort.Strings(keys)

	type payload struct {
		Dir   string   `json:"dir"`
		Items []string `json:"items"`
	}
	data, _ := json.Marshal(payload{Dir: dir, Items: keys})
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// CheckQuickCache looks up a cached result using the quick key (dir + catalog).
func CheckQuickCache(dir string, catalog []CatalogItem, threshold float64) *RecommendResult {
	key := QuickCacheKey(dir, catalog)
	cached := loadCachedResult(key, DefaultCacheTTL)
	if cached == nil {
		return nil
	}
	if threshold <= 0 {
		threshold = 0.7
	}
	filtered := make([]Recommendation, 0, len(cached.Recommendations))
	for _, rec := range cached.Recommendations {
		if rec.Confidence >= threshold {
			filtered = append(filtered, rec)
		}
	}
	cached.Recommendations = filtered
	return cached
}

// CheckCache looks up a cached recommendation result for the given profile and catalog.
// Returns non-nil result if a valid (non-expired) cached entry exists.
// Applies the given threshold filter to the cached results.
func CheckCache(profile detect.ProjectProfile, catalog []CatalogItem, threshold float64) *RecommendResult {
	key := cacheKey(profile, catalog)
	cached := loadCachedResult(key, DefaultCacheTTL)
	if cached == nil {
		return nil
	}
	if threshold <= 0 {
		threshold = 0.7
	}
	filtered := make([]Recommendation, 0, len(cached.Recommendations))
	for _, rec := range cached.Recommendations {
		if rec.Confidence >= threshold {
			filtered = append(filtered, rec)
		}
	}
	cached.Recommendations = filtered
	return cached
}

// loadCachedResult checks for a valid (non-expired) cached recommendation.
// Returns nil if no cache exists or it has expired.
func loadCachedResult(key string, ttl time.Duration) *RecommendResult {
	path := filepath.Join(cacheDir(), key+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil
	}

	if time.Since(entry.Timestamp) > ttl {
		// Expired — remove stale file.
		_ = os.Remove(path)
		return nil
	}

	return &RecommendResult{Recommendations: entry.Recommendations}
}

// saveCachedResult writes a recommendation result to the cache.
func saveCachedResult(key string, result *RecommendResult) {
	dir := cacheDir()
	_ = os.MkdirAll(dir, 0o755)

	entry := cacheEntry{
		Timestamp:       time.Now(),
		Recommendations: result.Recommendations,
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, key+".json"), data, 0o644)
}
