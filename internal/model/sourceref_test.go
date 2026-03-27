// SPDX-License-Identifier: MIT

package model

import (
	"testing"
)

// TestParseSourceRef_GitHub tests parsing of github: scheme source refs.
func TestParseSourceRef_GitHub(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		baseDir string
		want    SourceRef
		wantErr bool
	}{
		{
			name: "full ref with version and subpath",
			raw:  "github:owner/repo@v1.0.0//packages/shared",
			want: SourceRef{
				Scheme:  SchemeGitHub,
				Owner:   "owner",
				Repo:    "repo",
				Ref:     "v1.0.0",
				Subpath: "packages/shared",
			},
		},
		{
			name: "ref with version, no subpath",
			raw:  "github:owner/repo@v2.3.1",
			want: SourceRef{
				Scheme: SchemeGitHub,
				Owner:  "owner",
				Repo:   "repo",
				Ref:    "v2.3.1",
			},
		},
		{
			name: "ref with sha, no subpath",
			raw:  "github:owner/repo@abc1234def5678",
			want: SourceRef{
				Scheme: SchemeGitHub,
				Owner:  "owner",
				Repo:   "repo",
				Ref:    "abc1234def5678",
			},
		},
		{
			name: "ref with branch name",
			raw:  "github:owner/repo@main",
			want: SourceRef{
				Scheme: SchemeGitHub,
				Owner:  "owner",
				Repo:   "repo",
				Ref:    "main",
			},
		},
		{
			name: "ref without version or subpath",
			raw:  "github:owner/repo",
			want: SourceRef{
				Scheme: SchemeGitHub,
				Owner:  "owner",
				Repo:   "repo",
			},
		},
		{
			name: "ref with deep subpath",
			raw:  "github:davidarce/devrune-catalog@v0.1.0//skills/git-commit",
			want: SourceRef{
				Scheme:  SchemeGitHub,
				Owner:   "davidarce",
				Repo:    "devrune-catalog",
				Ref:     "v0.1.0",
				Subpath: "skills/git-commit",
			},
		},
		{
			name:    "missing owner",
			raw:     "github:repo@v1.0.0",
			wantErr: true,
		},
		{
			name:    "empty owner in owner/repo",
			raw:     "github:/repo@v1.0.0",
			wantErr: true,
		},
		{
			name:    "empty repo in owner/repo",
			raw:     "github:owner/@v1.0.0",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSourceRef(tt.raw, tt.baseDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSourceRef(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.Scheme != tt.want.Scheme {
				t.Errorf("Scheme = %q, want %q", got.Scheme, tt.want.Scheme)
			}
			if got.Owner != tt.want.Owner {
				t.Errorf("Owner = %q, want %q", got.Owner, tt.want.Owner)
			}
			if got.Repo != tt.want.Repo {
				t.Errorf("Repo = %q, want %q", got.Repo, tt.want.Repo)
			}
			if got.Ref != tt.want.Ref {
				t.Errorf("Ref = %q, want %q", got.Ref, tt.want.Ref)
			}
			if got.Subpath != tt.want.Subpath {
				t.Errorf("Subpath = %q, want %q", got.Subpath, tt.want.Subpath)
			}
		})
	}
}

// TestParseSourceRef_GitLab tests parsing of gitlab: scheme source refs.
func TestParseSourceRef_GitLab(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		baseDir string
		want    SourceRef
		wantErr bool
	}{
		{
			name: "full ref with version and subpath (default host)",
			raw:  "gitlab:owner/repo@v1.0.0//packages/shared",
			want: SourceRef{
				Scheme:  SchemeGitLab,
				Owner:   "owner",
				Repo:    "repo",
				Ref:     "v1.0.0",
				Subpath: "packages/shared",
				Host:    "gitlab.com",
			},
		},
		{
			name: "ref with version, no subpath (default host)",
			raw:  "gitlab:owner/repo@v2.3.1",
			want: SourceRef{
				Scheme: SchemeGitLab,
				Owner:  "owner",
				Repo:   "repo",
				Ref:    "v2.3.1",
				Host:   "gitlab.com",
			},
		},
		{
			name: "ref with custom host",
			raw:  "gitlab:owner/repo@v1.0.0//subpath?host=gitlab.example.com",
			want: SourceRef{
				Scheme:  SchemeGitLab,
				Owner:   "owner",
				Repo:    "repo",
				Ref:     "v1.0.0",
				Subpath: "subpath",
				Host:    "gitlab.example.com",
			},
		},
		{
			name: "ref with custom host, no subpath",
			raw:  "gitlab:myorg/myrepo@main?host=gitlab.example.com",
			want: SourceRef{
				Scheme: SchemeGitLab,
				Owner:  "myorg",
				Repo:   "myrepo",
				Ref:    "main",
				Host:   "gitlab.example.com",
			},
		},
		{
			name: "ref without version or subpath (default host)",
			raw:  "gitlab:owner/repo",
			want: SourceRef{
				Scheme: SchemeGitLab,
				Owner:  "owner",
				Repo:   "repo",
				Host:   "gitlab.com",
			},
		},
		{
			name: "ref with host param set to gitlab.com (redundant but valid)",
			raw:  "gitlab:owner/repo@v1.0.0?host=gitlab.com",
			want: SourceRef{
				Scheme: SchemeGitLab,
				Owner:  "owner",
				Repo:   "repo",
				Ref:    "v1.0.0",
				Host:   "gitlab.com",
			},
		},
		{
			name:    "missing owner (no slash)",
			raw:     "gitlab:repo@v1.0.0",
			wantErr: true,
		},
		{
			name:    "empty owner",
			raw:     "gitlab:/repo@v1.0.0",
			wantErr: true,
		},
		{
			name:    "empty repo",
			raw:     "gitlab:owner/@v1.0.0",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSourceRef(tt.raw, tt.baseDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSourceRef(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.Scheme != tt.want.Scheme {
				t.Errorf("Scheme = %q, want %q", got.Scheme, tt.want.Scheme)
			}
			if got.Owner != tt.want.Owner {
				t.Errorf("Owner = %q, want %q", got.Owner, tt.want.Owner)
			}
			if got.Repo != tt.want.Repo {
				t.Errorf("Repo = %q, want %q", got.Repo, tt.want.Repo)
			}
			if got.Ref != tt.want.Ref {
				t.Errorf("Ref = %q, want %q", got.Ref, tt.want.Ref)
			}
			if got.Subpath != tt.want.Subpath {
				t.Errorf("Subpath = %q, want %q", got.Subpath, tt.want.Subpath)
			}
			if got.Host != tt.want.Host {
				t.Errorf("Host = %q, want %q", got.Host, tt.want.Host)
			}
		})
	}
}

// TestParseSourceRef_Local tests parsing of local: scheme source refs.
func TestParseSourceRef_Local(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		baseDir string
		want    SourceRef
		wantErr bool
	}{
		{
			name: "relative path with ./",
			raw:  "local:./path/to/dir",
			want: SourceRef{
				Scheme: SchemeLocal,
				Path:   "./path/to/dir",
			},
		},
		{
			name: "relative parent traversal",
			raw:  "local:../relative/path",
			want: SourceRef{
				Scheme: SchemeLocal,
				Path:   "../relative/path",
			},
		},
		{
			name: "absolute path",
			raw:  "local:/absolute/path/to/dir",
			want: SourceRef{
				Scheme: SchemeLocal,
				Path:   "/absolute/path/to/dir",
			},
		},
		{
			name:    "empty local path",
			raw:     "local:",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSourceRef(tt.raw, tt.baseDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSourceRef(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.Scheme != tt.want.Scheme {
				t.Errorf("Scheme = %q, want %q", got.Scheme, tt.want.Scheme)
			}
			if got.Path != tt.want.Path {
				t.Errorf("Path = %q, want %q", got.Path, tt.want.Path)
			}
		})
	}
}

// TestSourceRef_Roundtrip verifies that ParseSourceRef(ref.String()) reproduces
// an equivalent SourceRef for all three schemes.
func TestSourceRef_Roundtrip(t *testing.T) {
	tests := []struct {
		name string
		ref  SourceRef
	}{
		{
			name: "github with version and subpath",
			ref: SourceRef{
				Scheme:  SchemeGitHub,
				Owner:   "davidarce",
				Repo:    "devrune-catalog",
				Ref:     "v0.1.0",
				Subpath: "packages/shared",
			},
		},
		{
			name: "github with version, no subpath",
			ref: SourceRef{
				Scheme: SchemeGitHub,
				Owner:  "owner",
				Repo:   "repo",
				Ref:    "main",
			},
		},
		{
			name: "github no version no subpath",
			ref: SourceRef{
				Scheme: SchemeGitHub,
				Owner:  "owner",
				Repo:   "repo",
			},
		},
		{
			name: "gitlab default host with subpath",
			ref: SourceRef{
				Scheme:  SchemeGitLab,
				Owner:   "myorg",
				Repo:    "myrepo",
				Ref:     "v1.2.3",
				Subpath: "workflows/sdd",
				Host:    "gitlab.com",
			},
		},
		{
			name: "gitlab custom host",
			ref: SourceRef{
				Scheme: SchemeGitLab,
				Owner:  "myorg",
				Repo:   "myrepo",
				Ref:    "main",
				Host:   "gitlab.example.com",
			},
		},
		{
			name: "local relative path",
			ref: SourceRef{
				Scheme: SchemeLocal,
				Path:   "./my-local-workflow",
			},
		},
		{
			name: "local parent traversal",
			ref: SourceRef{
				Scheme: SchemeLocal,
				Path:   "../shared/workflows",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serialized := tt.ref.String()
			reparsed, err := ParseSourceRef(serialized, "")
			if err != nil {
				t.Fatalf("ParseSourceRef(%q) roundtrip error = %v", serialized, err)
			}
			// Compare all relevant fields
			if reparsed.Scheme != tt.ref.Scheme {
				t.Errorf("Scheme: got %q, want %q", reparsed.Scheme, tt.ref.Scheme)
			}
			if reparsed.Owner != tt.ref.Owner {
				t.Errorf("Owner: got %q, want %q", reparsed.Owner, tt.ref.Owner)
			}
			if reparsed.Repo != tt.ref.Repo {
				t.Errorf("Repo: got %q, want %q", reparsed.Repo, tt.ref.Repo)
			}
			if reparsed.Ref != tt.ref.Ref {
				t.Errorf("Ref: got %q, want %q", reparsed.Ref, tt.ref.Ref)
			}
			if reparsed.Subpath != tt.ref.Subpath {
				t.Errorf("Subpath: got %q, want %q", reparsed.Subpath, tt.ref.Subpath)
			}
			if reparsed.Path != tt.ref.Path {
				t.Errorf("Path: got %q, want %q", reparsed.Path, tt.ref.Path)
			}
			// For GitLab: host should be preserved (or default to gitlab.com)
			if tt.ref.Scheme == SchemeGitLab {
				wantHost := tt.ref.Host
				if wantHost == "" {
					wantHost = "gitlab.com"
				}
				gotHost := reparsed.Host
				if gotHost == "" {
					gotHost = "gitlab.com"
				}
				if gotHost != wantHost {
					t.Errorf("Host: got %q, want %q", gotHost, wantHost)
				}
			}
		})
	}
}

// TestSourceRef_CacheKey verifies CacheKey produces stable, deterministic output.
func TestSourceRef_CacheKey(t *testing.T) {
	tests := []struct {
		name    string
		ref     SourceRef
		wantKey string
	}{
		{
			name: "github with ref and subpath",
			ref: SourceRef{
				Scheme:  SchemeGitHub,
				Owner:   "owner",
				Repo:    "repo",
				Ref:     "v1.0.0",
				Subpath: "packages/shared",
			},
			wantKey: "github:owner/repo@v1.0.0//packages/shared",
		},
		{
			name: "github with ref, no subpath",
			ref: SourceRef{
				Scheme: SchemeGitHub,
				Owner:  "owner",
				Repo:   "repo",
				Ref:    "main",
			},
			wantKey: "github:owner/repo@main",
		},
		{
			name: "gitlab default host",
			ref: SourceRef{
				Scheme: SchemeGitLab,
				Owner:  "owner",
				Repo:   "repo",
				Ref:    "v1.0.0",
				Host:   "gitlab.com",
			},
			wantKey: "gitlab:owner/repo@v1.0.0?host=gitlab.com",
		},
		{
			name: "gitlab custom host with subpath",
			ref: SourceRef{
				Scheme:  SchemeGitLab,
				Owner:   "myorg",
				Repo:    "myrepo",
				Ref:     "main",
				Subpath: "workflows/sdd",
				Host:    "gitlab.example.com",
			},
			wantKey: "gitlab:myorg/myrepo@main?host=gitlab.example.com//workflows/sdd",
		},
		{
			name: "local path",
			ref: SourceRef{
				Scheme: SchemeLocal,
				Path:   "./my-workflow",
			},
			wantKey: "local:./my-workflow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ref.CacheKey()
			if got != tt.wantKey {
				t.Errorf("CacheKey() = %q, want %q", got, tt.wantKey)
			}
		})
	}
}

// TestSourceRef_Validate tests the Validate method on SourceRef.
func TestSourceRef_Validate(t *testing.T) {
	tests := []struct {
		name    string
		ref     SourceRef
		wantErr bool
	}{
		{
			name:    "valid github",
			ref:     SourceRef{Scheme: SchemeGitHub, Owner: "owner", Repo: "repo", Ref: "main"},
			wantErr: false,
		},
		{
			name:    "valid gitlab",
			ref:     SourceRef{Scheme: SchemeGitLab, Owner: "owner", Repo: "repo", Host: "gitlab.com"},
			wantErr: false,
		},
		{
			name:    "valid local",
			ref:     SourceRef{Scheme: SchemeLocal, Path: "./path"},
			wantErr: false,
		},
		{
			name:    "empty scheme",
			ref:     SourceRef{Owner: "owner", Repo: "repo"},
			wantErr: true,
		},
		{
			name:    "unknown scheme",
			ref:     SourceRef{Scheme: "s3", Owner: "owner", Repo: "repo"},
			wantErr: true,
		},
		{
			name:    "github missing owner",
			ref:     SourceRef{Scheme: SchemeGitHub, Repo: "repo"},
			wantErr: true,
		},
		{
			name:    "github missing repo",
			ref:     SourceRef{Scheme: SchemeGitHub, Owner: "owner"},
			wantErr: true,
		},
		{
			name:    "gitlab missing owner",
			ref:     SourceRef{Scheme: SchemeGitLab, Repo: "repo"},
			wantErr: true,
		},
		{
			name:    "local missing path",
			ref:     SourceRef{Scheme: SchemeLocal},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ref.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestParseSourceRef_Invalid tests that invalid inputs return errors.
func TestParseSourceRef_Invalid(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "empty string", raw: ""},
		{name: "no scheme separator", raw: "owner/repo@v1.0.0"},
		{name: "unknown scheme", raw: "s3:bucket/path"},
		{name: "unknown scheme bitbucket", raw: "bitbucket:owner/repo@main"},
		{name: "scheme only no rest", raw: "github:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSourceRef(tt.raw, "")
			if err == nil {
				t.Errorf("ParseSourceRef(%q) expected error but got nil", tt.raw)
			}
		})
	}
}
