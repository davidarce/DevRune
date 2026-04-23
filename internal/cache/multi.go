// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/davidarce/devrune/internal/model"
)

// fetcher is a local interface alias matching resolve.Fetcher.
// Defined here to avoid a circular import between the cache and resolve packages.
// cache.MultiFetcher satisfies resolve.Fetcher by structural compatibility.
type fetcher interface {
	Fetch(ctx context.Context, ref model.SourceRef) ([]byte, error)
	Supports(scheme model.Scheme) bool
}

// revisionResolver mirrors resolve.RevisionResolver locally for the same
// reason as `fetcher` above — no import cycle with the resolve package.
type revisionResolver interface {
	ResolveRevision(ctx context.Context, ref model.SourceRef) (string, error)
}

// ErrRevisionUnsupported mirrors resolve.ErrRevisionUnsupported so this
// package can signal "no SHA check available" without depending on the
// resolve package. Callers typically errors.Is against either sentinel.
var ErrRevisionUnsupported = errors.New("cache: revision lookup not supported for this source")

// MultiFetcher dispatches Fetch calls to the appropriate scheme-specific Fetcher.
// It satisfies the resolve.Fetcher interface (structurally compatible) and acts
// as a composite root for all supported source schemes.
type MultiFetcher struct {
	fetchers map[model.Scheme]fetcher
}

// NewMultiFetcher creates a MultiFetcher that routes to github, gitlab, and local.
// Any of the three arguments may be nil; attempting to Fetch with a nil fetcher's
// scheme returns an error.
func NewMultiFetcher(github, gitlab, local fetcher) *MultiFetcher {
	m := &MultiFetcher{
		fetchers: make(map[model.Scheme]fetcher, 3),
	}
	if github != nil {
		m.fetchers[model.SchemeGitHub] = github
	}
	if gitlab != nil {
		m.fetchers[model.SchemeGitLab] = gitlab
	}
	if local != nil {
		m.fetchers[model.SchemeLocal] = local
	}
	return m
}

// Supports reports whether any registered fetcher handles the given scheme.
func (m *MultiFetcher) Supports(scheme model.Scheme) bool {
	_, ok := m.fetchers[scheme]
	return ok
}

// Fetch delegates to the scheme-specific fetcher.
// Returns an error if no fetcher is registered for the scheme.
func (m *MultiFetcher) Fetch(ctx context.Context, ref model.SourceRef) ([]byte, error) {
	f, ok := m.fetchers[ref.Scheme]
	if !ok {
		return nil, fmt.Errorf("multi fetcher: no fetcher registered for scheme %q", ref.Scheme)
	}
	return f.Fetch(ctx, ref)
}

// ResolveRevision delegates to the scheme-specific fetcher when that fetcher
// implements revisionResolver (GitHub, GitLab). Otherwise returns
// ErrRevisionUnsupported so the caller knows to fall back to always-refetch
// for mutable refs (LocalFetcher takes this path — no commit concept).
//
// This method also makes MultiFetcher structurally satisfy
// resolve.RevisionResolver so the resolver can type-assert it.
func (m *MultiFetcher) ResolveRevision(ctx context.Context, ref model.SourceRef) (string, error) {
	f, ok := m.fetchers[ref.Scheme]
	if !ok {
		return "", fmt.Errorf("multi fetcher: no fetcher registered for scheme %q", ref.Scheme)
	}
	rr, ok := f.(revisionResolver)
	if !ok {
		return "", ErrRevisionUnsupported
	}
	return rr.ResolveRevision(ctx, ref)
}
