// SPDX-License-Identifier: MIT

package recommend

// SkillsRegistry is the static, team-curated catalog of skills.sh skill references,
// grouped by framework or language name.
//
// Keys MUST match the exact strings returned by detect.inferFrameworks() for
// framework-level entries (e.g., "React", "Next.js", "Spring Boot"), or the
// LanguageInfo.Name field from detect.ProjectProfile.Languages for language-level
// entries (e.g., "Java", "Kotlin", "Go", "Python", "TypeScript").
//
// All skills listed here have been manually verified to pass the skills.sh audit.
// To add a new technology or skill, append an entry to this slice — no other
// changes are required. The registry is a compiled-in Go data structure with
// zero network calls; skill content is fetched only during `devrune resolve`.
var SkillsRegistry = []FrameworkSkills{
	// ── Frontend Frameworks ──────────────────────────────────────────────────
	{
		Framework: "React",
		Skills: []SkillRef{
			{
				Path:        "vercel-labs/agent-skills/vercel-react-best-practices",
				Description: "React best practices from Vercel",
			},
			{
				Path:        "vercel-labs/agent-skills/vercel-composition-patterns",
				Description: "Component composition patterns",
			},
		},
	},
	{
		Framework: "Next.js",
		Skills: []SkillRef{
			{
				Path:        "vercel-labs/next-skills/next-best-practices",
				Description: "Next.js best practices and conventions",
			},
			{
				Path:        "vercel-labs/next-skills/next-cache-components",
				Description: "Caching and server components patterns",
			},
		},
	},
	{
		Framework: "Vue.js",
		Skills: []SkillRef{
			{
				Path:        "hyf0/vue-skills/vue-best-practices",
				Description: "Vue.js best practices and conventions",
			},
		},
	},
	{
		Framework: "Angular",
		Skills: []SkillRef{
			{
				Path:        "angular/skills/angular-developer",
				Description: "Angular development best practices",
			},
		},
	},
	{
		Framework: "Svelte",
		Skills: []SkillRef{
			{
				Path:        "sveltejs/ai-tools/svelte-core-bestpractices",
				Description: "Svelte core best practices",
			},
		},
	},
	{
		Framework: "Tailwind CSS",
		Skills: []SkillRef{
			{
				Path:        "asyrafhussin/agent-skills/tailwind-best-practices",
				Description: "Tailwind CSS best practices",
			},
		},
	},
	// ── Backend Frameworks ───────────────────────────────────────────────────
	{
		Framework: "Spring Boot",
		Skills: []SkillRef{
			{
				Path:        "github/awesome-copilot/java-springboot",
				Description: "Spring Boot patterns and conventions",
			},
		},
	},
	{
		Framework: "Django",
		Skills: []SkillRef{
			{
				Path:        "vintasoftware/django-ai-plugins/django-expert",
				Description: "Django expert patterns and conventions",
			},
		},
	},
	{
		Framework: "FastAPI",
		Skills: []SkillRef{
			{
				Path:        "wshobson/agents/fastapi-templates",
				Description: "FastAPI templates and best practices",
			},
		},
	},
	{
		Framework: "Astro",
		Skills: []SkillRef{
			{
				Path:        "astrolicious/agent-skills/astro",
				Description: "Astro best practices and conventions",
			},
		},
	},
	{
		Framework: "Remix",
		Skills: []SkillRef{
			{
				Path:        "remix-run/agent-skills/react-router-framework-mode",
				Description: "React Router framework mode (Remix)",
			},
		},
	},
	// ── Language-level entries ───────────────────────────────────────────────
	// Matched via ProjectProfile.Languages[].Name rather than inferFrameworks().
	{
		Framework: "Java",
		Skills: []SkillRef{
			{
				Path:        "github/awesome-copilot/java-docs",
				Description: "Java documentation and reference patterns",
			},
			{
				Path:        "affaan-m/everything-claude-code/java-coding-standards",
				Description: "Java coding standards and patterns",
			},
		},
	},
	{
		Framework: "Kotlin",
		Skills: []SkillRef{
			{
				Path:        "github/awesome-copilot/kotlin-springboot",
				Description: "Kotlin with Spring Boot patterns",
			},
			{
				Path:        "affaan-m/everything-claude-code/kotlin-patterns",
				Description: "Kotlin coding patterns and conventions",
			},
		},
	},
	{
		Framework: "Go",
		Skills: []SkillRef{
			{
				Path:        "affaan-m/everything-claude-code/golang-patterns",
				Description: "Go patterns and idioms",
			},
		},
	},
	{
		Framework: "Python",
		Skills: []SkillRef{
			{
				Path:        "affaan-m/everything-claude-code/python-patterns",
				Description: "Python patterns and conventions",
			},
		},
	},
	{
		Framework: "TypeScript",
		Skills: []SkillRef{
			{
				Path:        "wshobson/agents/typescript-advanced-types",
				Description: "TypeScript advanced types and type safety",
			},
		},
	},
}
