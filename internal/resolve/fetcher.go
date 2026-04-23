// SPDX-License-Identifier: MIT

// Package resolve implements Stage 2 of the DevRune pipeline: converting a
// UserManifest into a Lockfile by fetching, hashing, and enumerating packages.
// All network and filesystem I/O is injected via the Fetcher interface.
package resolve

import (
	"context"
	"errors"

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

// RevisionResolver is an optional capability for Fetchers that can cheaply
// resolve a ref (possibly mutable like HEAD or a branch name) to a stable
// commit SHA without downloading the full archive.
//
// Implementations typically issue a single lightweight API call — e.g.
// GitHub `/repos/{owner}/{repo}/commits/{ref}` or GitLab `/projects/{id}/
// repository/commits/{ref}` — that responds in ~100ms with a few hundred
// bytes. The resolver uses this to re-validate cached mutable refs on
// subsequent syncs without re-downloading multi-MB tarballs when upstream
// hasn't moved.
//
// Fetchers that do not support revision resolution (local:, where there is
// no commit concept; or backends that simply haven't implemented it yet)
// should not implement this interface — callers fall back to the always-
// refetch path established by the HEAD bypass fix.
type RevisionResolver interface {
	// ResolveRevision returns the commit SHA the ref currently points at.
	// An empty ref is treated as HEAD of the default branch.
	// Returns ErrRevisionUnsupported when the source has no stable revision
	// identity (local paths); other errors indicate network/permission
	// failures and should be treated as "cannot determine revision" by the
	// caller, which falls back to re-fetching the archive.
	ResolveRevision(ctx context.Context, ref model.SourceRef) (sha string, err error)
}

// ErrRevisionUnsupported is returned by RevisionResolver implementations that
// cannot meaningfully report a commit SHA for a source (most notably the
// local: scheme).
var ErrRevisionUnsupported = errors.New("resolve: revision lookup not supported for this source")
