// SPDX-License-Identifier: MIT

package detect

import "strings"

// depFrameworks maps dependency name substrings to framework names.
// Keys are matched as exact or prefix/contains matches against dependency names.
var depFrameworks = []struct {
	match     string
	framework string
	exact     bool // if true, match exact dependency name; otherwise substring
}{
	// JavaScript / TypeScript
	{"react-dom", "React", true},
	{"react", "React", true},
	{"next", "Next.js", true},
	{"vue", "Vue.js", true},
	{"@angular/core", "Angular", true},
	{"svelte", "Svelte", true},
	{"express", "Express.js", true},
	{"fastify", "Fastify", true},
	{"nuxt", "Nuxt.js", true},
	{"astro", "Astro", true},
	{"remix", "Remix", true},
	// Go
	{"github.com/gin-gonic/gin", "Gin", true},
	{"github.com/labstack/echo", "Echo", false},
	{"github.com/gofiber/fiber", "Fiber", false},
	{"github.com/spf13/cobra", "Cobra CLI", true},
	{"github.com/go-chi/chi", "Chi", false},
	// Java / Maven / Gradle (Spring Boot starter prefix)
	{"spring-boot-starter", "Spring Boot", false},
	{"org.springframework.boot", "Spring Boot", false},
	// Python
	{"django", "Django", true},
	{"flask", "Flask", true},
	{"fastapi", "FastAPI", true},
	// Rust
	{"actix-web", "Actix Web", true},
	{"axum", "Axum", true},
	{"rocket", "Rocket", true},
}

// configFrameworks maps config file path substrings to framework names.
var configFrameworks = []struct {
	substring string
	framework string
}{
	{"tailwind.config", "Tailwind CSS"},
	{"Dockerfile", "Docker"},
	{"docker-compose", "Docker"},
	{".github/workflows", "GitHub Actions"},
	{"jest.config", "Jest"},
	{"vite.config", "Vite"},
	{"webpack.config", "Webpack"},
	{"next.config", "Next.js"},
}

// inferFrameworks derives framework names from detected dependency files and config files.
// It deduplicates results so each framework appears at most once.
func inferFrameworks(deps []DependencyFile, configs []string) []string {
	seen := map[string]bool{}
	var frameworks []string

	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			frameworks = append(frameworks, name)
		}
	}

	// Match dependencies.
	for _, dep := range deps {
		for name := range dep.Dependencies {
			nameLower := strings.ToLower(name)
			for _, rule := range depFrameworks {
				if rule.exact {
					if nameLower == strings.ToLower(rule.match) {
						add(rule.framework)
					}
				} else {
					if strings.Contains(nameLower, strings.ToLower(rule.match)) {
						add(rule.framework)
					}
				}
			}
		}
	}

	// Match config files.
	for _, cfg := range configs {
		for _, rule := range configFrameworks {
			if strings.Contains(cfg, rule.substring) {
				add(rule.framework)
			}
		}
	}

	return frameworks
}
