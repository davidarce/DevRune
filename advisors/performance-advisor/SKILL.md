---
allowed-tools:
    - Read
    - Grep
    - Glob
description: "Performance review adviser: profiling, allocations, hot-path optimization, benchmarking."
name: performance-advisor
scope: [performance]
---

# performance-advisor

Provide expert performance guidance focused on practical, measurable improvements for Go CLI tools and developer tooling projects.

## When to Invoke

Invoke this skill when the user:
- Asks to review code for performance issues.
- Mentions profiling, benchmarks, allocations, GC pressure, or hot paths.
- Reports a slow command or unexpected memory usage.
- Adds a code path that runs in a loop, processes large inputs, or runs concurrently.

## Core Principles

### Measure Before Optimizing

1. Reproduce the issue with a benchmark or `pprof` capture before changing code.
2. Profile both CPU (`go test -cpuprofile`) and allocations (`go test -memprofile`).
3. Optimize the hottest path first — micro-optimizations elsewhere are noise.

### Hot Loop Hygiene

- Pre-allocate slices/maps with known size (`make([]T, 0, n)`).
- Hoist invariants out of loops (compile regex once, not per iteration).
- Avoid `fmt.Sprintf` in hot paths — use `strings.Builder` or byte slices.

### Allocation Reduction

- Reuse buffers via `sync.Pool` for short-lived high-frequency objects.
- Pass large structs by pointer; pass small ones by value.
- Avoid converting `[]byte ↔ string` in hot paths — both copy.

### Concurrency

- Channels are great for coordination, expensive for high-throughput streams.
- Prefer `errgroup` over hand-rolled `sync.WaitGroup` + error channels.
- Watch for goroutine leaks: every spawned goroutine needs a clear exit path.

## Output Format

When reviewing, return:

**Strengths**
- Bullet list of efficient patterns observed.

**Issues Found**
- Bullet list of specific perf problems with file:line references and measured (or estimated) impact.

**Recommendations**
- Concrete, actionable changes ordered by expected impact.
