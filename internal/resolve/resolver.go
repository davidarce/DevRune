// SPDX-License-Identifier: MIT

package resolve

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/parse"
)

// CacheStore is the port interface for content-addressed package storage.
// The canonical implementation is cache.FileCacheStore.
// Defined here to keep the resolve package free of import cycles with cache.
type CacheStore interface {
	// Store writes archive bytes to the cache and returns the extracted directory path.
	Store(key string, data []byte) (dir string, err error)
	// Get returns the extracted directory path for the given "sha256:<hex>" hash.
	Get(hash string) (dir string, ok bool)
	// Has reports whether the given "sha256:<hex>" hash is cached.
	Has(hash string) bool
}

// Resolver is the main orchestrator for Stage 2 of the DevRune pipeline.
// It converts a UserManifest into a Lockfile by:
//  1. Expanding workflows (if any)
//  2. Fetching, hashing, and caching each package archive
//  3. Enumerating and filtering content items
//  4. Fetching and caching MCP and workflow sources
//  5. Computing the manifest hash
type Resolver struct {
	fetcher    Fetcher
	cache      CacheStore
	baseDir    string                // directory containing devrune.yaml
	priorIndex map[string]priorEntry // CacheKey → prior content hash + revision
}

// priorEntry carries the two values we need from a prior lockfile row for
// cache decisions: the content hash (cache dir key) and the commit SHA the
// ref pointed at last time (mutable-ref re-validation).
type priorEntry struct {
	hash     string
	revision string
}

// NewResolver creates a Resolver that uses the given fetcher and cache.
// baseDir is the directory containing devrune.yaml; it is used to resolve
// relative local paths.
func NewResolver(fetcher Fetcher, cache CacheStore, baseDir string) *Resolver {
	return &Resolver{
		fetcher: fetcher,
		cache:   cache,
		baseDir: baseDir,
	}
}

// SetPriorLockfile builds an index from an existing lockfile so that the resolver
// can skip network fetches for packages whose content hash is already cached.
// Local sources are excluded — their content may have changed on disk.
func (r *Resolver) SetPriorLockfile(lf model.Lockfile) {
	idx := make(map[string]priorEntry)
	for _, pkg := range lf.Packages {
		if pkg.Source.Scheme != model.SchemeLocal && pkg.Hash != "" {
			idx[pkg.Source.CacheKey()] = priorEntry{hash: pkg.Hash, revision: pkg.Revision}
		}
	}
	for _, mcp := range lf.MCPs {
		if mcp.Source.Scheme != model.SchemeLocal && mcp.Hash != "" {
			idx[mcp.Source.CacheKey()] = priorEntry{hash: mcp.Hash, revision: mcp.Revision}
		}
	}
	for _, wf := range lf.Workflows {
		if wf.Source.Scheme != model.SchemeLocal && wf.Hash != "" {
			idx[wf.Source.CacheKey()] = priorEntry{hash: wf.Hash, revision: wf.Revision}
		}
	}
	r.priorIndex = idx
}

// cachedDir decides whether a remote source can reuse its prior-lockfile
// cache entry. Returns the cached directory, content hash, and the commit
// SHA recorded for it (empty when the prior lockfile predates the revision
// field) when the cache is valid. Returns ok=false when the caller must
// re-fetch from the network.
//
// The decision is SHA-driven, not ref-kind-driven. Whenever the fetcher
// can cheaply resolve a ref to a commit SHA (via RevisionResolver), we do
// the check for EVERY remote ref — empty, HEAD, branch names, tags, full
// commit SHAs. The ref string itself never has to be classified as
// mutable or immutable because the SHA is the ground truth: if it matches
// the prior-lockfile revision the underlying commit hasn't moved and the
// cached archive is still valid.
//
// Decision matrix:
//
//  1. No prior index, or local scheme → always re-fetch.
//  2. No prior entry for this CacheKey → always re-fetch (first resolve).
//  3. Fetcher implements RevisionResolver:
//     a. Prior revision recorded → cheap GET, compare SHAs. Match reuses
//        the cache; mismatch/error/missing-cache-dir all fall through to
//        re-fetch.
//     b. No prior revision (old lockfile schema) → re-fetch so the next
//        lockfile captures the SHA.
//  4. Fetcher does NOT implement RevisionResolver (legacy backend; today
//     only the local scheme but kept defensively for future backends):
//     a. Mutable ref (HEAD / empty) → re-fetch. This preserves the safety
//        guarantee of the HEAD-bypass fix (6bed877) — without an SHA
//        check, trusting the cache would silently hide upstream moves.
//     b. Immutable-looking ref (everything else) → reuse cache by hash.
func (r *Resolver) cachedDir(ctx context.Context, sourceRef model.SourceRef) (dir, hash, revision string, ok bool) {
	if r.priorIndex == nil {
		return "", "", "", false
	}
	// Local sources always re-fetch (content may change on disk).
	if sourceRef.Scheme == model.SchemeLocal {
		return "", "", "", false
	}
	prior, exists := r.priorIndex[sourceRef.CacheKey()]
	if !exists {
		return "", "", "", false
	}

	// Preferred path: the fetcher can cheaply resolve the ref to a commit
	// SHA, so trust the SHA instead of the ref's name to decide cache
	// validity. This is the uniform path regardless of ref kind.
	if rr, canResolve := r.fetcher.(RevisionResolver); canResolve {
		if prior.revision == "" {
			// Old lockfile without the revision field — can't compare, so
			// refetch to populate it. Next sync hits the fast path.
			return "", "", "", false
		}
		currentSHA, err := rr.ResolveRevision(ctx, sourceRef)
		if err != nil {
			// Network hiccup, missing permissions, or ErrRevisionUnsupported.
			// Prefer a tarball download over serving potentially stale
			// content — correctness beats cache reuse on error paths.
			return "", "", "", false
		}
		if currentSHA != prior.revision {
			// Upstream moved — let the caller refetch and record the new SHA.
			return "", "", "", false
		}
		if d, cached := r.cache.Get(prior.hash); cached {
			return d, prior.hash, prior.revision, true
		}
		// SHA matched but the cache dir was pruned — re-fetch to repopulate.
		return "", "", "", false
	}

	// Fallback path: fetcher can't resolve revisions. Preserve the post-
	// 6bed877 safety for mutable refs (always refetch) and the original
	// hash-based cache reuse for immutable refs. In practice today this
	// branch is only reached by the local scheme, which we already short-
	// circuited above, so this exists purely to keep future RevisionResolver-
	// less backends safe.
	if isMutableRef(sourceRef.Ref) {
		return "", "", "", false
	}
	if d, cached := r.cache.Get(prior.hash); cached {
		return d, prior.hash, prior.revision, true
	}
	return "", "", "", false
}

// isMutableRef reports whether a SourceRef's ref component names a moving
// target (empty or literal HEAD) rather than a pinned-looking revision.
//
// This is a name-based heuristic and is ONLY consulted in the fallback
// path of cachedDir — when the fetcher can't resolve revisions. In the
// preferred path we don't care about ref kinds at all because the commit
// SHA is the source of truth. The function stays here for backends that
// don't implement RevisionResolver yet.
func isMutableRef(ref string) bool {
	return ref == "" || ref == "HEAD"
}

// revisionForFetch captures the commit SHA for a freshly-fetched remote
// source so the next resolve can cheap-check it and reuse the cache when
// upstream hasn't moved. Called for every remote ref — empty, HEAD, branch,
// tag, or commit SHA — because the SHA check is now the uniform path.
// Returns "" silently for local sources, fetchers without RevisionResolver,
// and any ResolveRevision error.
func (r *Resolver) revisionForFetch(ctx context.Context, sourceRef model.SourceRef) string {
	if sourceRef.Scheme == model.SchemeLocal {
		return ""
	}
	rr, ok := r.fetcher.(RevisionResolver)
	if !ok {
		return ""
	}
	sha, err := rr.ResolveRevision(ctx, sourceRef)
	if err != nil {
		return ""
	}
	return sha
}

// Resolve processes a UserManifest and produces a Lockfile.
// The manifest must pass Validate() before calling this method.
func (r *Resolver) Resolve(ctx context.Context, manifest model.UserManifest) (model.Lockfile, error) {
	// Step 1: Expand workflows.
	expanded, err := ExpandWorkflows(ctx, manifest, r.fetcher, r.baseDir)
	if err != nil {
		return model.Lockfile{}, fmt.Errorf("resolve: expand workflows: %w", err)
	}

	// Step 2: Resolve packages.
	lockedPkgs, err := r.resolvePackages(ctx, expanded.Packages)
	if err != nil {
		return model.Lockfile{}, err
	}

	// Step 3: Resolve MCPs.
	lockedMCPs, err := r.resolveMCPs(ctx, expanded.MCPs)
	if err != nil {
		return model.Lockfile{}, err
	}

	// Step 4: Resolve workflows.
	lockedWorkflows, err := r.resolveWorkflows(ctx, workflowSources(expanded.Workflows))
	if err != nil {
		return model.Lockfile{}, err
	}

	// Step 5: Compute manifest hash over the original (unexpanded) manifest YAML.
	manifestHash, err := computeManifestHash(manifest)
	if err != nil {
		return model.Lockfile{}, fmt.Errorf("resolve: compute manifest hash: %w", err)
	}

	return model.Lockfile{
		SchemaVersion: parse.LockfileSchemaVersion,
		ManifestHash:  manifestHash,
		Packages:      lockedPkgs,
		MCPs:          lockedMCPs,
		Workflows:     lockedWorkflows,
	}, nil
}

// maxConcurrentFetches limits parallel network requests during resolution.
const maxConcurrentFetches = 6

// resolvePackages fetches, hashes, caches, and enumerates each package reference.
// Remote packages are resolved in parallel (bounded by maxConcurrentFetches).
func (r *Resolver) resolvePackages(ctx context.Context, refs []model.PackageRef) ([]model.LockedPackage, error) {
	result := make([]model.LockedPackage, len(refs))
	errs := make([]error, len(refs))

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentFetches)

	for i, ref := range refs {
		wg.Add(1)
		go func(i int, ref model.PackageRef) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			result[i], errs[i] = r.resolvePackage(ctx, ref)
		}(i, ref)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// resolvePackage resolves a single PackageRef into a LockedPackage.
func (r *Resolver) resolvePackage(ctx context.Context, pkg model.PackageRef) (model.LockedPackage, error) {
	sourceRef, err := model.ParseSourceRef(pkg.Source, r.baseDir)
	if err != nil {
		return model.LockedPackage{}, fmt.Errorf("resolve: package %q: parse source ref: %w", pkg.Source, err)
	}

	var dir, hash, revision string

	// Cache-first: check if the prior lockfile has a cached hash for this source.
	if d, h, rev, ok := r.cachedDir(ctx, sourceRef); ok {
		dir, hash, revision = d, h, rev
	} else {
		data, err := r.fetcher.Fetch(ctx, sourceRef)
		if err != nil {
			return model.LockedPackage{}, fmt.Errorf("resolve: package %q: fetch: %w", pkg.Source, err)
		}
		hash = HashBytes(data)
		dir, err = r.cache.Store(sourceRef.CacheKey(), data)
		if err != nil {
			return model.LockedPackage{}, fmt.Errorf("resolve: package %q: cache: %w", pkg.Source, err)
		}
		revision = r.revisionForFetch(ctx, sourceRef)
	}

	allItems, err := EnumerateContents(dir)
	if err != nil {
		return model.LockedPackage{}, fmt.Errorf("resolve: package %q: enumerate: %w", pkg.Source, err)
	}

	filtered := ApplyFilter(allItems, pkg.Select)

	return model.LockedPackage{
		Source:   sourceRef,
		Hash:     hash,
		Revision: revision,
		Contents: filtered,
	}, nil
}

// resolveMCPs fetches and hashes each MCP reference.
// MCP YAML files are small; we store their content in the cache just like packages.
func (r *Resolver) resolveMCPs(ctx context.Context, refs []model.MCPRef) ([]model.LockedMCP, error) {
	result := make([]model.LockedMCP, len(refs))
	errs := make([]error, len(refs))

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentFetches)

	for i, ref := range refs {
		wg.Add(1)
		go func(i int, ref model.MCPRef) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			result[i], errs[i] = r.resolveMCP(ctx, ref)
		}(i, ref)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// resolveMCP resolves a single MCPRef into a LockedMCP.
func (r *Resolver) resolveMCP(ctx context.Context, mcp model.MCPRef) (model.LockedMCP, error) {
	sourceRef, err := model.ParseSourceRef(mcp.Source, r.baseDir)
	if err != nil {
		return model.LockedMCP{}, fmt.Errorf("resolve: mcp %q: parse source ref: %w", mcp.Source, err)
	}

	var hash, revision string

	if d, h, rev, ok := r.cachedDir(ctx, sourceRef); ok {
		_ = d // MCP doesn't use dir directly
		hash = h
		revision = rev
	} else {
		data, err := r.fetcher.Fetch(ctx, sourceRef)
		if err != nil {
			return model.LockedMCP{}, fmt.Errorf("resolve: mcp %q: fetch: %w", mcp.Source, err)
		}
		if _, err := r.cache.Store(sourceRef.CacheKey(), data); err != nil {
			return model.LockedMCP{}, fmt.Errorf("resolve: mcp %q: cache: %w", mcp.Source, err)
		}
		hash = HashBytes(data)
		revision = r.revisionForFetch(ctx, sourceRef)
	}

	name := mcpName(sourceRef)
	dir := mcpDir(sourceRef)

	return model.LockedMCP{
		Source:   sourceRef,
		Hash:     hash,
		Revision: revision,
		Name:     name,
		Dir:      dir,
	}, nil
}

// workflowSources extracts the source ref strings from a Workflows map.
// The order is non-deterministic (map iteration), but that is acceptable for resolution.
func workflowSources(workflows map[string]model.WorkflowEntry) []string {
	sources := make([]string, 0, len(workflows))
	for _, entry := range workflows {
		sources = append(sources, entry.Source)
	}
	return sources
}

// resolveWorkflows fetches and hashes each workflow source.
// Workflow directories are stored in the cache; their name comes from workflow.yaml.
func (r *Resolver) resolveWorkflows(ctx context.Context, sources []string) ([]model.LockedWorkflow, error) {
	result := make([]model.LockedWorkflow, len(sources))
	errs := make([]error, len(sources))

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentFetches)

	for i, wfSource := range sources {
		wg.Add(1)
		go func(i int, wfSource string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			result[i], errs[i] = r.resolveWorkflow(ctx, wfSource)
		}(i, wfSource)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// resolveWorkflow resolves a single workflow source string into a LockedWorkflow.
func (r *Resolver) resolveWorkflow(ctx context.Context, wfSource string) (model.LockedWorkflow, error) {
	sourceRef, err := model.ParseSourceRef(wfSource, r.baseDir)
	if err != nil {
		return model.LockedWorkflow{}, fmt.Errorf("resolve: workflow %q: parse source ref: %w", wfSource, err)
	}

	var data []byte
	var hash, revision string

	if d, h, rev, ok := r.cachedDir(ctx, sourceRef); ok {
		_ = d
		hash = h
		revision = rev
		// Still need data to parse workflow.yaml — read from cache dir.
		// Re-fetch from cache store is not possible since we only store extracted dirs.
		// Fall through to fetch path to get raw data for parsing.
		// Actually, we need the raw archive to call extractAndParseWorkflow.
		// For workflows with cache hit, read the workflow.yaml from the cached dir.
	}

	if hash == "" {
		// No cache hit — fetch from network.
		data, err = r.fetcher.Fetch(ctx, sourceRef)
		if err != nil {
			return model.LockedWorkflow{}, fmt.Errorf("resolve: workflow %q: fetch: %w", wfSource, err)
		}
		hash = HashBytes(data)
		if _, err := r.cache.Store(sourceRef.CacheKey(), data); err != nil {
			return model.LockedWorkflow{}, fmt.Errorf("resolve: workflow %q: cache: %w", wfSource, err)
		}
		revision = r.revisionForFetch(ctx, sourceRef)
	}

	// Parse workflow.yaml — try from cache dir first (faster), fall back to archive data.
	var wfManifest model.WorkflowManifest
	var wfDir string

	if dir, ok := r.cache.Get(hash); ok {
		// If the source ref has a subpath (e.g. "//workflows/sdd"), resolve
		// workflow.yaml relative to that subdirectory within the cached archive
		// root rather than searching the entire package tree.
		lookupDir := dir
		if sourceRef.Subpath != "" {
			lookupDir = filepath.Join(dir, filepath.FromSlash(sourceRef.Subpath))
		}
		wfManifest, wfDir, err = parseWorkflowFromDir(lookupDir)
		if err != nil {
			return model.LockedWorkflow{}, fmt.Errorf("resolve: workflow %q: %w", wfSource, err)
		}
		// wfDir is relative to lookupDir; reconstruct it relative to the cache
		// root so the materializer can locate the workflow directory correctly.
		if sourceRef.Subpath != "" {
			if wfDir == "" {
				wfDir = filepath.ToSlash(sourceRef.Subpath)
			} else {
				wfDir = filepath.ToSlash(filepath.Join(sourceRef.Subpath, wfDir))
			}
		}
	} else if data != nil {
		wfManifest, wfDir, err = extractAndParseWorkflow(data, wfSource)
		if err != nil {
			return model.LockedWorkflow{}, fmt.Errorf("resolve: workflow %q: %w", wfSource, err)
		}
	} else {
		return model.LockedWorkflow{}, fmt.Errorf("resolve: workflow %q: cached but cannot read workflow.yaml", wfSource)
	}

	return model.LockedWorkflow{
		Source:   sourceRef,
		Hash:     hash,
		Revision: revision,
		Name:     wfManifest.Metadata.Name,
		Dir:      wfDir,
	}, nil
}

// computeManifestHash serialises manifest to YAML and returns "sha256:<hex>".
func computeManifestHash(manifest model.UserManifest) (string, error) {
	data, err := parse.SerializeManifest(manifest)
	if err != nil {
		return "", fmt.Errorf("serialize manifest: %w", err)
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", sum), nil
}

// mcpName derives a human-readable name for an MCP from its SourceRef.
// When a Subpath is present (e.g. "mcps/atlassian"), the basename is used ("atlassian").
// This ensures catalog-hosted MCPs get their individual name, not the repo name.
func mcpName(ref model.SourceRef) string {
	switch ref.Scheme {
	case model.SchemeGitHub, model.SchemeGitLab:
		// Prefer subpath basename when available (e.g. "mcps/atlassian" → "atlassian").
		if ref.Subpath != "" {
			return pathBasename(ref.Subpath)
		}
		if ref.Repo != "" {
			return ref.Repo
		}
		return ref.Owner
	case model.SchemeLocal:
		if ref.Path != "" {
			// Use the last path component, stripping any file extension.
			base := ref.Path
			for len(base) > 0 && (base[len(base)-1] == '/' || base[len(base)-1] == '\\') {
				base = base[:len(base)-1]
			}
			name := base
			for i := len(base) - 1; i >= 0; i-- {
				if base[i] == '/' || base[i] == '\\' {
					name = base[i+1:]
					break
				}
			}
			// Strip common config file extensions.
			name = strings.TrimSuffix(name, ".yaml")
			name = strings.TrimSuffix(name, ".yml")
			name = strings.TrimSuffix(name, ".json")
			return name
		}
	}
	return string(ref.Scheme)
}

// mcpDir returns the relative directory path within the cached archive where the
// MCP definition file lives. For catalog-hosted MCPs this equals the Subpath
// (e.g. "mcps/atlassian"). For standalone MCP sources with no subpath, it is "".
func mcpDir(ref model.SourceRef) string {
	switch ref.Scheme {
	case model.SchemeGitHub, model.SchemeGitLab:
		return ref.Subpath // may be "" for standalone MCP repos
	case model.SchemeLocal:
		return "" // local MCPs point directly at the definition file/dir
	}
	return ""
}

// pathBasename returns the last slash-separated component of p.
// Trailing slashes are stripped before extracting the component.
func pathBasename(p string) string {
	p = strings.TrimRight(p, "/")
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return p
	}
	return p[idx+1:]
}
