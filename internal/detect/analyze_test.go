// SPDX-License-Identifier: MIT

package detect_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/davidarce/devrune/internal/detect"
)

// makeGoProject writes a minimal Go project into dir and returns the dir path.
func makeGoProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	gomod := `module github.com/example/myapp

go 1.21

require (
	github.com/spf13/cobra v1.8.0
	github.com/gin-gonic/gin v1.9.1
)
`
	mainGo := `package main

import "fmt"

func main() {
	fmt.Println("hello world")
}
`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainGo), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	return dir
}

// makeReactProject writes a minimal React/TypeScript project into dir and returns the dir path.
func makeReactProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	pkgJSON := `{
  "name": "my-react-app",
  "version": "1.0.0",
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0"
  },
  "devDependencies": {
    "typescript": "^5.0.0",
    "@types/react": "^18.2.0"
  }
}
`
	tsconfig := `{
  "compilerOptions": {
    "target": "ES2020",
    "jsx": "react-jsx",
    "strict": true
  },
  "include": ["src"]
}
`
	appTSX := `import React from "react";

function App() {
  return <div>Hello World</div>;
}

export default App;
`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsconfig), 0o644); err != nil {
		t.Fatalf("write tsconfig.json: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "App.tsx"), []byte(appTSX), 0o644); err != nil {
		t.Fatalf("write App.tsx: %v", err)
	}
	return dir
}

func TestAnalyze_GoProject(t *testing.T) {
	dir := makeGoProject(t)

	profile, err := detect.Analyze(dir)
	if err != nil {
		t.Fatalf("Analyze(go-project): unexpected error: %v", err)
	}

	// Language detection: Go should be present.
	foundGo := false
	for _, l := range profile.Languages {
		if l.Name == "Go" {
			foundGo = true
			if l.Files < 1 {
				t.Errorf("expected at least 1 Go file, got %d", l.Files)
			}
		}
	}
	if !foundGo {
		t.Errorf("expected Go to be detected as a language; got languages: %+v", profile.Languages)
	}

	// Dependency parsing: go.mod should be parsed.
	foundGoMod := false
	for _, dep := range profile.Dependencies {
		if dep.Type == "gomod" {
			foundGoMod = true
			if _, ok := dep.Dependencies["github.com/spf13/cobra"]; !ok {
				t.Errorf("expected cobra in go.mod deps; got: %v", dep.Dependencies)
			}
			if _, ok := dep.Dependencies["github.com/gin-gonic/gin"]; !ok {
				t.Errorf("expected gin in go.mod deps; got: %v", dep.Dependencies)
			}
		}
	}
	if !foundGoMod {
		t.Errorf("expected go.mod to be parsed as gomod dependency")
	}

	// Framework inference: Cobra and Gin should be detected.
	frameworks := make(map[string]bool)
	for _, f := range profile.Frameworks {
		frameworks[f] = true
	}
	if !frameworks["Cobra CLI"] {
		t.Errorf("expected Cobra CLI framework to be inferred; got: %v", profile.Frameworks)
	}
	if !frameworks["Gin"] {
		t.Errorf("expected Gin framework to be inferred; got: %v", profile.Frameworks)
	}

	// Total files must be > 0.
	if profile.TotalFiles < 1 {
		t.Errorf("expected TotalFiles > 0, got %d", profile.TotalFiles)
	}
}

func TestAnalyze_ReactProject(t *testing.T) {
	dir := makeReactProject(t)

	profile, err := detect.Analyze(dir)
	if err != nil {
		t.Fatalf("Analyze(react-project): unexpected error: %v", err)
	}

	// Language detection: TypeScript or TSX should be present.
	foundTS := false
	for _, l := range profile.Languages {
		if l.Name == "TypeScript" || l.Name == "TSX" {
			foundTS = true
		}
	}
	if !foundTS {
		t.Errorf("expected TypeScript/TSX language; got: %+v", profile.Languages)
	}

	// Dependency parsing: package.json should be parsed.
	foundNPM := false
	for _, dep := range profile.Dependencies {
		if dep.Type == "npm" {
			foundNPM = true
			if _, ok := dep.Dependencies["react"]; !ok {
				t.Errorf("expected react in package.json deps; got: %v", dep.Dependencies)
			}
		}
	}
	if !foundNPM {
		t.Errorf("expected package.json to be parsed as npm dependency")
	}

	// Framework inference: React should be detected.
	frameworks := make(map[string]bool)
	for _, f := range profile.Frameworks {
		frameworks[f] = true
	}
	if !frameworks["React"] {
		t.Errorf("expected React framework to be inferred; got: %v", profile.Frameworks)
	}

	// Config files: tsconfig.json should be detected.
	foundTSConfig := false
	for _, cf := range profile.ConfigFiles {
		if cf == "tsconfig.json" {
			foundTSConfig = true
		}
	}
	if !foundTSConfig {
		t.Errorf("expected tsconfig.json in ConfigFiles; got: %v", profile.ConfigFiles)
	}
}

func TestAnalyze_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	profile, err := detect.Analyze(dir)
	if err != nil {
		t.Fatalf("Analyze(empty dir): unexpected error: %v", err)
	}

	if len(profile.Languages) != 0 {
		t.Errorf("expected no languages for empty dir, got: %v", profile.Languages)
	}
	if len(profile.Dependencies) != 0 {
		t.Errorf("expected no dependencies for empty dir, got: %v", profile.Dependencies)
	}
	if len(profile.Frameworks) != 0 {
		t.Errorf("expected no frameworks for empty dir, got: %v", profile.Frameworks)
	}
	if profile.TotalFiles != 0 {
		t.Errorf("expected TotalFiles=0 for empty dir, got %d", profile.TotalFiles)
	}
}

func TestAnalyze_NonExistentDir(t *testing.T) {
	_, err := detect.Analyze("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error for non-existent directory, got nil")
	}
}
