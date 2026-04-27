# Go Profiling Cheatsheet

## Capture Profiles

```bash
# CPU + memory profile from benchmarks
go test -bench=. -benchmem -cpuprofile=cpu.out -memprofile=mem.out

# Live process profiles (when net/http/pprof is enabled)
curl -o cpu.out http://localhost:6060/debug/pprof/profile?seconds=30
curl -o heap.out http://localhost:6060/debug/pprof/heap
curl -o goroutine.out http://localhost:6060/debug/pprof/goroutine
```

## Inspect

```bash
go tool pprof -http=:8080 cpu.out      # interactive flame graph in browser
go tool pprof -top -cum cpu.out        # top functions by cumulative time
go tool pprof -base old.out new.out    # diff two profiles
```

## Benchmark Comparison

```bash
go test -bench=. -count=10 > old.txt
# ... make changes ...
go test -bench=. -count=10 > new.txt
benchstat old.txt new.txt
```

Look for: time/op delta, allocs/op delta, B/op delta. A change that improves time but worsens allocs may not survive in production where GC pressure matters.
