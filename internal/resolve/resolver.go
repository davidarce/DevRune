package resolve

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

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
	fetcher Fetcher
	cache   CacheStore
	baseDir string // directory containing devrune.yaml
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
	lockedWorkflows, err := r.resolveWorkflows(ctx, expanded.Workflows)
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

// resolvePackages fetches, hashes, caches, and enumerates each package reference.
func (r *Resolver) resolvePackages(ctx context.Context, refs []model.PackageRef) ([]model.LockedPackage, error) {
	result := make([]model.LockedPackage, 0, len(refs))

	for _, ref := range refs {
		locked, err := r.resolvePackage(ctx, ref)
		if err != nil {
			return nil, err
		}
		result = append(result, locked)
	}
	return result, nil
}

// resolvePackage resolves a single PackageRef into a LockedPackage.
func (r *Resolver) resolvePackage(ctx context.Context, pkg model.PackageRef) (model.LockedPackage, error) {
	sourceRef, err := model.ParseSourceRef(pkg.Source, r.baseDir)
	if err != nil {
		return model.LockedPackage{}, fmt.Errorf("resolve: package %q: parse source ref: %w", pkg.Source, err)
	}

	data, err := r.fetcher.Fetch(ctx, sourceRef)
	if err != nil {
		return model.LockedPackage{}, fmt.Errorf("resolve: package %q: fetch: %w", pkg.Source, err)
	}

	hash := HashBytes(data)

	dir, err := r.cache.Store(sourceRef.CacheKey(), data)
	if err != nil {
		return model.LockedPackage{}, fmt.Errorf("resolve: package %q: cache: %w", pkg.Source, err)
	}

	allItems, err := EnumerateContents(dir)
	if err != nil {
		return model.LockedPackage{}, fmt.Errorf("resolve: package %q: enumerate: %w", pkg.Source, err)
	}

	filtered := ApplyFilter(allItems, pkg.Select)

	return model.LockedPackage{
		Source:   sourceRef,
		Hash:     hash,
		Contents: filtered,
	}, nil
}

// resolveMCPs fetches and hashes each MCP reference.
// MCP YAML files are small; we store their content in the cache just like packages.
func (r *Resolver) resolveMCPs(ctx context.Context, refs []model.MCPRef) ([]model.LockedMCP, error) {
	result := make([]model.LockedMCP, 0, len(refs))

	for _, ref := range refs {
		locked, err := r.resolveMCP(ctx, ref)
		if err != nil {
			return nil, err
		}
		result = append(result, locked)
	}
	return result, nil
}

// resolveMCP resolves a single MCPRef into a LockedMCP.
func (r *Resolver) resolveMCP(ctx context.Context, mcp model.MCPRef) (model.LockedMCP, error) {
	sourceRef, err := model.ParseSourceRef(mcp.Source, r.baseDir)
	if err != nil {
		return model.LockedMCP{}, fmt.Errorf("resolve: mcp %q: parse source ref: %w", mcp.Source, err)
	}

	data, err := r.fetcher.Fetch(ctx, sourceRef)
	if err != nil {
		return model.LockedMCP{}, fmt.Errorf("resolve: mcp %q: fetch: %w", mcp.Source, err)
	}

	// Store in cache so the materializer can read MCP definitions.
	if _, err := r.cache.Store(sourceRef.CacheKey(), data); err != nil {
		return model.LockedMCP{}, fmt.Errorf("resolve: mcp %q: cache: %w", mcp.Source, err)
	}

	hash := HashBytes(data)

	// Derive an MCP name from the source ref (owner/repo or local path basename).
	name := mcpName(sourceRef)

	return model.LockedMCP{
		Source: sourceRef,
		Hash:   hash,
		Name:   name,
	}, nil
}

// resolveWorkflows fetches and hashes each workflow source.
// Workflow directories are stored in the cache; their name comes from workflow.yaml.
func (r *Resolver) resolveWorkflows(ctx context.Context, sources []string) ([]model.LockedWorkflow, error) {
	result := make([]model.LockedWorkflow, 0, len(sources))

	for _, wfSource := range sources {
		locked, err := r.resolveWorkflow(ctx, wfSource)
		if err != nil {
			return nil, err
		}
		result = append(result, locked)
	}
	return result, nil
}

// resolveWorkflow resolves a single workflow source string into a LockedWorkflow.
func (r *Resolver) resolveWorkflow(ctx context.Context, wfSource string) (model.LockedWorkflow, error) {
	sourceRef, err := model.ParseSourceRef(wfSource, r.baseDir)
	if err != nil {
		return model.LockedWorkflow{}, fmt.Errorf("resolve: workflow %q: parse source ref: %w", wfSource, err)
	}

	data, err := r.fetcher.Fetch(ctx, sourceRef)
	if err != nil {
		return model.LockedWorkflow{}, fmt.Errorf("resolve: workflow %q: fetch: %w", wfSource, err)
	}

	hash := HashBytes(data)

	// Store the workflow archive in the cache so that the install step can find it.
	if _, err := r.cache.Store(sourceRef.CacheKey(), data); err != nil {
		return model.LockedWorkflow{}, fmt.Errorf("resolve: workflow %q: cache: %w", wfSource, err)
	}

	// Parse workflow.yaml to extract the workflow name.
	wfManifest, wfDir, err := extractAndParseWorkflow(data, wfSource)
	if err != nil {
		return model.LockedWorkflow{}, fmt.Errorf("resolve: workflow %q: %w", wfSource, err)
	}

	return model.LockedWorkflow{
		Source: sourceRef,
		Hash:   hash,
		Name:   wfManifest.Metadata.Name,
		Dir:    wfDir,
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
func mcpName(ref model.SourceRef) string {
	switch ref.Scheme {
	case model.SchemeGitHub, model.SchemeGitLab:
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
