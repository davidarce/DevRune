---
name: git:commit
description: Automate git commits following Conventional Commits with JIRA ticket integration.
version: 1.0.0
---

# git:commit Skill

Automate git commits following Conventional Commits specification.

## Usage

Invoke this skill when the user requests a commit. It will:
1. Inspect staged changes
2. Draft a conventional commit message
3. Create the commit

## Commit Message Format

```
<type>(<scope>): <description>

[optional body]

Co-Authored-By: Claude <noreply@anthropic.com>
```
