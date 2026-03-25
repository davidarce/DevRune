// Package cache provides content-addressed package storage and fetcher
// implementations for the GitHub, GitLab, and local filesystem schemes.
package cache

// CacheStore is the port interface for content-addressed package storage.
// Implementations store extracted package directories keyed by a content hash.
// The canonical implementation uses SHA256 and stores to ~/.cache/devrune/.
type CacheStore interface {
	// Store writes the archive bytes to the cache and extracts them into a
	// directory. Returns the path to the extracted directory on success.
	// key is a deterministic, human-readable identifier (e.g. the SourceRef
	// CacheKey) used to build the on-disk directory name; the implementation
	// appends the SHA256 hash to make it content-addressable.
	Store(key string, data []byte) (dir string, err error)

	// Get returns the path to the extracted directory for the given SHA256
	// hash (in "sha256:<hex>" format). Returns ("", false) if not cached.
	Get(hash string) (dir string, ok bool)

	// Has reports whether a cached entry exists for the given SHA256 hash.
	Has(hash string) bool
}
