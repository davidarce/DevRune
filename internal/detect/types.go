// SPDX-License-Identifier: MIT

package detect

// ProjectProfile holds the tech fingerprint of a project directory.
type ProjectProfile struct {
	Languages    []LanguageInfo   // detected languages with file counts/percentages
	Dependencies []DependencyFile // parsed dependency manifests (package.json, go.mod, pom.xml, etc.)
	ConfigFiles  []string         // detected config files (Dockerfile, .eslintrc, tsconfig.json, etc.)
	Frameworks   []string         // inferred frameworks (e.g. "Spring Boot", "Next.js", "React")
	TotalFiles   int              // total files scanned
	TotalLines   int              // total lines of code
}

// LanguageInfo holds statistics for a single detected language.
type LanguageInfo struct {
	Name       string  // e.g. "Go", "TypeScript", "Java"
	Files      int     // number of files
	Lines      int     // lines of code
	Percentage float64 // percentage of total codebase
}

// DependencyFile represents a parsed dependency manifest file.
type DependencyFile struct {
	Path         string            // e.g. "go.mod", "package.json"
	Type         string            // "gomod" | "npm" | "maven" | "gradle" | "pip" | "cargo"
	Dependencies map[string]string // name → version (top-level deps only)
}
