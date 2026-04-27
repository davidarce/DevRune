# Go Performance Rules

## Profiling

- Always profile before optimizing — `go test -bench=. -cpuprofile=cpu.out -memprofile=mem.out`.
- Read flame graphs top-down: the widest frames at the bottom of the stack are the real cost centers.
- Compare profiles across commits with `pprof -base` to verify a change actually moved the needle.

## Slices

- `make([]T, 0, n)` when capacity is known — avoids growth-and-copy.
- Reuse a slice via `s = s[:0]` to clear without freeing the underlying array.
- Beware of `append` aliasing: if two slices share an array, an append on one can stomp the other.

## Maps

- Pre-size with `make(map[K]V, n)` when the entry count is roughly known.
- For tiny lookup sets (≤8 entries), a slice with linear scan is often faster than a map.
- `delete` does not shrink a map — if you delete most entries, build a fresh one.

## Strings vs. Bytes

- `strings.Builder` for building strings incrementally — avoids the per-step copy of `+=`.
- `bytes.Buffer` when working with `io.Writer` interfaces.
- A `string([]byte)` conversion copies. Use `unsafe.String` only when you can prove the byte slice is immutable for the string's lifetime.

## Goroutines

- Bound concurrency — unbounded `go` calls in a loop will exhaust memory.
- Every goroutine needs an exit: a context, a closed channel, or a sentinel value.
- `errgroup.WithContext` cancels siblings on first error — use it for fan-out work.
