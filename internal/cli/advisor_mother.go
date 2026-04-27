// SPDX-License-Identifier: MIT

// Package cli provides the Object Mother fluent builders for advisor-related
// test fixtures. These builders are used by advisors_inventory_test.go,
// advisors_sync_test.go, and any other test that needs well-formed
// model.AdvisorDef, model.CatalogSource, or model.UserManifest values.
//
// None of these functions are part of the production CLI surface; they live
// here (rather than in a _test.go file) so that both internal (package cli)
// and external (package cli_test) test packages can import them through the
// shared internal path.

package cli

import (
	"github.com/davidarce/devrune/internal/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// advisorDefBuilder — fluent builder for model.AdvisorDef
// ─────────────────────────────────────────────────────────────────────────────

// advisorDefBuilder is the fluent builder for model.AdvisorDef.
// Construct via AnAdvisorDef().
type advisorDefBuilder struct {
	def model.AdvisorDef
}

// AnAdvisorDef returns a new advisorDefBuilder pre-populated with safe defaults:
//   - Name:        "test-advisor"
//   - Description: "A test advisor"
//   - SkillSource: "./testdata/test-advisor"
//   - Scope:       nil (universal — applies to every project)
//   - Origin:      model.AdvisorOriginLocal
func AnAdvisorDef() *advisorDefBuilder {
	return &advisorDefBuilder{
		def: model.AdvisorDef{
			Name:        "test-advisor",
			Description: "A test advisor",
			SkillSource: "./testdata/test-advisor",
			Origin:      model.AdvisorOriginLocal,
		},
	}
}

// Named overrides the Name field.
func (b *advisorDefBuilder) Named(name string) *advisorDefBuilder {
	b.def.Name = name
	return b
}

// WithScope overrides the Scope field with one or more scope tags.
// Accepted vocabulary: "frontend", "backend", "testing", "architecture",
// "api", "security", "performance", "accessibility".
// Passing no arguments sets Scope to nil (universal — applies to every project).
// Example: WithScope("backend", "testing")
func (b *advisorDefBuilder) WithScope(scope ...string) *advisorDefBuilder {
	if len(scope) == 0 {
		b.def.Scope = nil
		return b
	}
	b.def.Scope = scope
	return b
}

// WithSkillSource overrides the SkillSource field.
func (b *advisorDefBuilder) WithSkillSource(p string) *advisorDefBuilder {
	b.def.SkillSource = p
	return b
}

// WithOrigin overrides the Origin field.
func (b *advisorDefBuilder) WithOrigin(o model.AdvisorOrigin) *advisorDefBuilder {
	b.def.Origin = o
	return b
}

// WithDescription overrides the Description field.
func (b *advisorDefBuilder) WithDescription(desc string) *advisorDefBuilder {
	b.def.Description = desc
	return b
}

// Build returns the fully constructed model.AdvisorDef.
func (b *advisorDefBuilder) Build() model.AdvisorDef {
	return b.def
}

// ─────────────────────────────────────────────────────────────────────────────
// catalogSourceBuilder — fluent builder for model.CatalogSource
// ─────────────────────────────────────────────────────────────────────────────

// catalogSourceBuilder is the fluent builder for model.CatalogSource.
// Construct via ACatalogSource().
type catalogSourceBuilder struct {
	src model.CatalogSource
}

// ACatalogSource returns a new catalogSourceBuilder pre-populated with safe defaults:
//   - URL:         "github:acme/advisor-catalog@main"
//   - Name:        "Acme advisors"
//   - LastFetched: ""
func ACatalogSource() *catalogSourceBuilder {
	return &catalogSourceBuilder{
		src: model.CatalogSource{
			URL:  "github:acme/advisor-catalog@main",
			Name: "Acme advisors",
		},
	}
}

// WithURL overrides the URL field.
func (b *catalogSourceBuilder) WithURL(url string) *catalogSourceBuilder {
	b.src.URL = url
	return b
}

// WithName overrides the Name field.
func (b *catalogSourceBuilder) WithName(name string) *catalogSourceBuilder {
	b.src.Name = name
	return b
}

// WithLastFetched overrides the LastFetched field (RFC3339 string).
func (b *catalogSourceBuilder) WithLastFetched(ts string) *catalogSourceBuilder {
	b.src.LastFetched = ts
	return b
}

// Build returns the fully constructed model.CatalogSource.
func (b *catalogSourceBuilder) Build() model.CatalogSource {
	return b.src
}

// ─────────────────────────────────────────────────────────────────────────────
// userManifestBuilder — fluent builder for model.UserManifest
// ─────────────────────────────────────────────────────────────────────────────

// userManifestBuilder is the fluent builder for model.UserManifest.
// Construct via AUserManifest().
type userManifestBuilder struct {
	manifest model.UserManifest
}

// AUserManifest returns a new userManifestBuilder pre-populated with safe defaults:
//   - SchemaVersion: "devrune/v1"
//   - Agents:        [{Name: "claude"}]
//   - Packages, CustomAdvisors, AdvisorCatalogs: empty slices
func AUserManifest() *userManifestBuilder {
	return &userManifestBuilder{
		manifest: model.UserManifest{
			SchemaVersion: "devrune/v1",
			Agents:        []model.AgentRef{{Name: "claude"}},
		},
	}
}

// WithPackage adds a PackageRef with the given source and an optional list of
// skill names. If no skills are provided the SelectFilter is set but empty.
func (b *userManifestBuilder) WithPackage(source string, skills ...string) *userManifestBuilder {
	pkg := model.PackageRef{
		Source: source,
		Select: &model.SelectFilter{
			Skills: skills,
		},
	}
	b.manifest.Packages = append(b.manifest.Packages, pkg)
	return b
}

// WithCustom appends the given AdvisorDef entries to CustomAdvisors.
func (b *userManifestBuilder) WithCustom(defs ...model.AdvisorDef) *userManifestBuilder {
	b.manifest.CustomAdvisors = append(b.manifest.CustomAdvisors, defs...)
	return b
}

// WithCatalog appends the given CatalogSource entries to AdvisorCatalogs.
func (b *userManifestBuilder) WithCatalog(sources ...model.CatalogSource) *userManifestBuilder {
	b.manifest.AdvisorCatalogs = append(b.manifest.AdvisorCatalogs, sources...)
	return b
}

// WithAgent replaces the Agents slice with a single AgentRef of the given name.
// Call multiple times or pass the desired agent name (e.g. "copilot") to
// override the default "claude" agent.
func (b *userManifestBuilder) WithAgent(name string) *userManifestBuilder {
	b.manifest.Agents = []model.AgentRef{{Name: name}}
	return b
}

// Build returns the fully constructed model.UserManifest.
func (b *userManifestBuilder) Build() model.UserManifest {
	return b.manifest
}
