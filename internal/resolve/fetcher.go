// Package resolve implements Stage 2 of the DevRune pipeline: converting a
// UserManifest into a Lockfile by fetching, hashing, and enumerating packages.
// All network and filesystem I/O is injected via the Fetcher interface.
package resolve

import (
	"context"

	"github.com/davidarce/devrune/internal/model"
)

// Fetcher is the port interface for downloading a package archive.
// Implementations exist for GitHub, GitLab, and local filesystem.
// The returned bytes are a gzip-compressed tar archive of the package contents.
type Fetcher interface {
	// Fetch downloads the archive identified by ref and returns its raw bytes.
	// The caller is responsible for decompressing and extracting the archive.
	// Returns an error if the scheme is unsupported, the network fails, or
	// the server returns a non-2xx status.
	Fetch(ctx context.Context, ref model.SourceRef) ([]byte, error)

	// Supports reports whether this Fetcher handles the given scheme.
	Supports(scheme model.Scheme) bool
}
