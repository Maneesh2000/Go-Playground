# Module 11 — Advanced Concurrency

## What you'll learn

- What a **data race** is and how to catch one with the race detector (`go run -race`)
- Protecting shared state with `sync.Mutex` and `sync.RWMutex`
- One-time initialization with `sync.Once` and lock-free counters with `sync/atomic`
- `context.Context`: cancellation, timeouts, and request-scoped values — and how a context flows through a call tree
- Four everyday concurrency patterns: **worker pool**, **fan-out/fan-in**, **pipeline with cancellation**, and a **semaphore** built from a buffered channel
- The channel "axioms": exactly what happens when you send/receive/close on nil or closed channels

## Data races

A **data race** happens when two goroutines access the same memory at the same
time and at least one of them writes. The result is undefined — sometimes a
wrong number, sometimes a corrupted map, sometimes a crash that only appears in
production.

```go
counter := 0
for i := 0; i < 1000; i++ {
    go func() { counter++ }() // RACE: read + write from many goroutines
}
```

`counter++` is *not* atomic — it is a read, an add, and a write. Two goroutines
can both read `5`, both add one, and both write `6`. You lost an increment.

Go ships a race detector. It instruments memory accesses at runtime and prints
a report when it sees a race:

```bash
go run -race 01_races_and_sync.go
go test -race ./...          # also works for tests
```

Rule of thumb: **run your tests with `-race` in CI, always.** It has runtime
overhead (roughly 5–10x), so it's a testing tool, not a production flag.

## sync.Mutex and sync.RWMutex

A `Mutex` (mutual exclusion lock) lets only one goroutine into a critical
section at a time:

```go
var mu sync.Mutex
mu.Lock()
counter++      // only one goroutine can be here
mu.Unlock()
```

The classic use case is protecting a map, because **maps are not safe for
concurrent use**:

```go
type SafeMap struct {
    mu sync.RWMutex
    m  map[string]int
}

func (s *SafeMap) Get(k string) int {
    s.mu.RLock()         // many readers may hold RLock at once
    defer s.mu.RUnlock()
    return s.m[k]
}

func (s *SafeMap) Set(k string, v int) {
    s.mu.Lock()          // writers get exclusive access
    defer s.mu.Unlock()
    s.m[k] = v
}
```

`RWMutex` allows unlimited concurrent readers *or* one writer. Use it when
reads vastly outnumber writes; otherwise a plain `Mutex` is simpler and often
faster.

Tips:

- Keep the locked region as small as possible.
- `defer mu.Unlock()` right after locking, so early returns can't leak a lock.
- Never copy a struct containing a mutex (pass pointers). `go vet` catches this.

## sync.Once and sync/atomic

`sync.Once` runs a function exactly once, no matter how many goroutines call it
— perfect for lazy initialization:

```go
var once sync.Once
once.Do(loadConfig) // loadConfig runs once; other callers block until done
```

(Go 1.21+ also has `sync.OnceValue` / `sync.OnceFunc` helpers.)

`sync/atomic` gives you lock-free primitives for simple cases like counters and
flags:

```go
var hits atomic.Int64
hits.Add(1)
fmt.Println(hits.Load())
```

Prefer the typed API (`atomic.Int64`, `atomic.Bool`, `atomic.Pointer[T]`) over
the older function style. Use atomics for single values; use a mutex the moment
several values must change together.

## context.Context

A `context.Context` carries **cancellation signals, deadlines, and
request-scoped values** down a call tree. It is the standard way to say
"stop working, nobody needs this result anymore".

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel() // ALWAYS call cancel, even on success, to free resources

result, err := fetchUser(ctx, id) // pass ctx as the FIRST parameter
```

Inside a worker you *listen* for cancellation:

```go
select {
case <-ctx.Done():
    return ctx.Err() // context.Canceled or context.DeadlineExceeded
case out <- value:
}
```

How the context flows through a call tree — cancelling a parent cancels every
child derived from it:

```
Background()
    │
    ├── WithTimeout(2s) ─────────► HTTP handler
    │        │
    │        ├── WithCancel ─────► database query   ┐ when the timeout fires,
    │        │                                      │ Done() closes for ALL
    │        └── (same ctx) ─────► cache lookup     ┘ of these at once
    │
    └── WithCancel ──────────────► background job
```

Rules of the road:

- `ctx` is always the **first** parameter: `func Do(ctx context.Context, ...)`.
- Never store a context in a struct; pass it explicitly.
- `context.WithValue` is only for request-scoped metadata (trace IDs, auth
  info), **not** for passing normal function arguments. Use a private key type
  to avoid collisions.

## Patterns

### Worker pool

N workers read jobs from one channel and write results to another. Backpressure
is free: if workers are busy, sends into `jobs` block.

```
             jobs                       results
producer ──►[ ch ]──► worker 1 ──┐
                  ──► worker 2 ──┼──►[ ch ]──► collector
                  ──► worker 3 ──┘
```

### Fan-out / fan-in

Fan-out: several goroutines read from the same channel (work gets distributed).
Fan-in: merge several channels into one, using a `sync.WaitGroup` to know when
to close the merged output.

### Pipeline with cancellation

Each stage is a goroutine connected by channels: `generate → square → print`.
Every stage `select`s on `ctx.Done()` so the whole pipeline unwinds cleanly
when the consumer stops early.

### Semaphore with a buffered channel

A buffered channel of capacity N limits concurrency to N:

```go
sem := make(chan struct{}, 3) // at most 3 in flight
sem <- struct{}{}             // acquire (blocks when full)
go func() { defer func() { <-sem }(); work() }() // release
```

> **In real projects:** the `golang.org/x/sync/errgroup` package wraps
> "start N goroutines, wait for all, return the first error, cancel the rest"
> into a tidy API (`g, ctx := errgroup.WithContext(ctx)`), and
> `errgroup.SetLimit` gives you a bounded pool. We stick to the standard
> library here, but reach for errgroup at work.

## Channel axioms

Memorize this table — it explains almost every channel bug:

| Operation   | nil channel      | open channel        | closed channel                     |
|-------------|------------------|---------------------|------------------------------------|
| send `ch<-` | blocks forever   | blocks until recv   | **panic**                          |
| recv `<-ch` | blocks forever   | blocks until send   | returns zero value immediately, `ok == false` |
| `close(ch)` | **panic**        | closes it           | **panic** (can't close twice)      |

Consequences:

- Only the **sender** closes a channel, never the receiver.
- `for v := range ch` exits when the channel is closed and drained.
- A nil channel in a `select` disables that case — a useful trick for turning
  cases off.

## Run the examples

```bash
cd module11_concurrency_advanced

go run -race 01_races_and_sync.go   # see the race report, then the fixes
go run 02_context.go
go run 03_worker_pool.go
go run 04_fanout_fanin_pipeline.go
```

## Key takeaways

- If two goroutines touch the same variable and one writes, you need a mutex,
  an atomic, or a channel. Run tests with `-race` to be sure.
- `RWMutex` for read-heavy maps; keep critical sections tiny; defer unlocks.
- `context.Context` is Go's cancellation tree: pass it first, respect
  `ctx.Done()`, always `defer cancel()`.
- Buffered channels double as semaphores; worker pools give you bounded
  parallelism with backpressure built in.
- Senders close channels; receiving from a closed channel is safe, sending is a
  panic.

## Exercises

1. Write a `SafeCounter` struct with `Inc(key string)` and `Value(key string) int`
   methods backed by a `map[string]int` and an `RWMutex`. Hammer it from 100
   goroutines and verify with `go run -race` that it's clean.
2. Build a pipeline `numbers → filter even → multiply by 10` where each stage
   takes a `context.Context`. Cancel after receiving 3 results and confirm (with
   a print in each stage) that all stages exit.
3. Extend the worker pool example so each job can fail: give `result` an `err`
   field, make the collector count successes and failures, and print a summary.
