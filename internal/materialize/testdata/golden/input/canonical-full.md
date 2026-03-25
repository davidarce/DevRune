---
name: git:commit
description: Automate git commits following Conventional Commits with JIRA ticket integration.
allowed-tools:
    - Bash(git:*)
    - Read
    - Edit
model: sonnet
argument-hint: "[message]"
mode: subagent
reasoning-effort: low
temperature: 0.7
tools-mode: auto
disable-model-invocation: false
---
# git:commit

Automate git commits following Conventional Commits.
