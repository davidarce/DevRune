---
name: git:pull-request
description: Create pull requests with template selection, JIRA enrichment, and platform auto-detection (GitHub/GitLab).
version: 1.0.0
---

# git:pull-request Skill

Create pull requests with template selection and JIRA enrichment.

## Usage

Invoke this skill when the user requests a pull request. It will:
1. Detect the platform (GitHub or GitLab)
2. Collect information about the change
3. Create the PR with a descriptive title and body
