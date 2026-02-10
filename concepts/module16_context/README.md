# Module 16 — The `context` Package

## What you'll learn

- **Why** `context` exists: cancelling in-flight work and carrying request-scoped
  data across API boundaries and goroutines
- The context **tree**: cancel a parent and every derived child cancels with it
- The `Context` interface itself: `Done()`, `Err()`, `Deadline()`, `Value()` —
  what each means and who calls it
- Root contexts: `context.Background()` vs `context.TODO()`
- `WithCancel` (manual stop), `WithTimeout` / `WithDeadline` (time budgets),
  `WithValue` (request-scoped metadata — and what it's *not* for)
- The Go 1.20/1.21 additions: `WithCancelCause` + `Cause`, `WithoutCancel`, `AfterFunc`
- Real-world integration: `net/http` request contexts, `signal.NotifyContext`
  for graceful shutdown, `exec.CommandContext`, `database/sql`
- The idioms (`ctx` first, never in a struct, never nil) and the classic
  mistakes — including a measurable goroutine leak, then its fix

## Why context exists

A request arrives. You start goroutines, call libraries, query a database.
Then the client hangs up, or the deadline passes, or a sibling task fails.
Everything still running for that request is now **wasted work** — but how do
you tell code three packages away, across five goroutines, to stop?

Before `context`, every library invented its own stop channel or timeout knob.
`context.Context` standardizes it: **one value, passed explicitly down the call
chain**, carrying three things:

1. a **cancellation signal** (a channel that closes),
2. an optional **deadline**,
3. **request-scoped values** (request ID, auth identity).

You never cancel a context you received — you **derive** children from it.
That builds a tree, and cancellation flows strictly *downward*:

```
                 context.Background()          (root: never cancelled)
                          │
                 WithCancel ──► cancel()       ← cancelling HERE...
                 ┌────────┴────────┐
           WithTimeout          WithValue
           (per-RPC 2s)        (request id)
                 │                  │
             WithCancel        WithTimeout
             (subtask)          (DB query)
                                    
        ...closes Done() on EVERY context below it.
        Cancelling a leaf never affects its parent or siblings.
```

A context ends at whichever comes first: its own cancel/deadline, or **any
ancestor's**. A child asking for a longer deadline than its parent doesn't get
one.

## The Context interface

The whole interface — `context` is small on purpose:

```go
type Context interface {
    Done() <-chan struct{}                    // closed when the ctx ends
    Err() error                               // nil, Canceled, or DeadlineExceeded
    Deadline() (deadline time.Time, ok bool)  // "when must I be done?"
    Value(key any) any                        // request-scoped lookup
}
```

- **`Done()`** — a channel that is *closed* (never sent on) when the context is
  cancelled or expires. **Callees** receive from it, almost always in a `select`.
- **`Err()`** — `nil` while alive. After the end: `context.Canceled` (someone
  called cancel) or `context.DeadlineExceeded` (the clock ran out). Once
  non-nil, it never changes. Great for polling in CPU-bound loops.
- **`Deadline()`** — lets a callee budget its work ("only 50ms left, skip the
  cache warm-up"). `ok == false` means no deadline anywhere up the tree.
- **`Value(key)`** — walks up the tree looking for a matching key (see below).

You rarely implement `Context`; you **derive** with the `With*` constructors
and **consume** via `Done()`/`Err()`.

## Root contexts: Background vs TODO

```go
ctx := context.Background() // THE root: main(), init, tests, top of a request
ctx := context.TODO()       // placeholder: "not plumbed through yet"
```

Functionally identical (empty, never cancelled, no deadline, no values). The
difference is *intent*: `TODO()` marks unfinished migration for readers and
linters. **Never pass `nil`** as a context — use `TODO()` if unsure.

## WithCancel — manual cancellation

```go
ctx, cancel := context.WithCancel(parent)
defer cancel() // ALWAYS — even if you also cancel explicitly elsewhere
```

`cancel()` closes `ctx.Done()` for this context and its whole subtree. It's
idempotent (safe to call twice). Until it's called, the parent holds a
reference to the child — **forgetting `cancel` leaks** the context (and, for
timeout contexts, a timer). `go vet` warns about lost cancel functions.

Worker shape (from `01_cancellation_basics.go`):

```go
for {
    select {
    case <-ctx.Done():
        return              // stop reason available via ctx.Err()
    case job := <-jobs:
        process(job)
    }
}
```

## WithTimeout / WithDeadline — time budgets

Same mechanism, two spellings:

```go
ctx, cancel := context.WithTimeout(parent, 2*time.Second)     // relative: "2s from now"
ctx, cancel := context.WithDeadline(parent, absoluteTime)     // absolute: "until T"
defer cancel() // releases the timer early if you finish before it fires
```

`WithTimeout(ctx, d)` is literally `WithDeadline(ctx, time.Now().Add(d))`.
Prefer `WithDeadline` when the instant comes from outside ("upstream says
respond by T").

The canonical *race* between slow work and the budget:

```go
select {
case res := <-doWork():        // work finished first — use it
    return res, nil
case <-ctx.Done():             // budget blown — abandon
    return nil, ctx.Err()      // context.DeadlineExceeded
}
```

Check which way it ended with `errors.Is(err, context.DeadlineExceeded)` /
`errors.Is(err, context.Canceled)` — timeouts usually arrive wrapped in `%w`
chains.

### How one timeout cancels a whole call tree

Only `handle` sets a budget; everyone below just passes `ctx` on:

```
 client        handle(ctx)        fetchUser        queryDB      fetchAvatar
   │  request      │                  │                │             │
   ├──────────────►│ WithTimeout 150ms│                │             │
   │               ├─────────────────►│                │             │
   │               │                  ├───────────────►│  (60ms) ok  │
   │               │                  │◄───────────────┤             │
   │               │                  ├───────────────────────────  ►│ (500ms...)
   │               │    ⏰ 150ms elapse: timer fires, Done() closes  │
   │               │                  │                │      ✗ select sees
   │               │                  │◄─ DeadlineExceeded ──────────┤ <-ctx.Done()
   │               │◄─ "fetchAvatar: context deadline exceeded" ─────┤
   │◄─ 504 ────────┤                                                 │
```

`queryDB` fit inside the budget; `fetchAvatar` was cut off mid-flight — with
zero timeout code of its own. Run it: `02_timeout_deadline.go`.

## WithValue — request-scoped values ONLY

```go
type ctxKey int                 // unexported key type: collision-proof
const requestIDKey ctxKey = 0

ctx = context.WithValue(ctx, requestIDKey, "req-7f3a")
id, ok := ctx.Value(requestIDKey).(string)
```

- Keys use an **unexported type** so no other package can produce an equal key
  — export typed helpers (`WithRequestID(ctx, id)` / `RequestIDFrom(ctx)`), not keys.
- Lookup walks *up* the tree; children see parent values, parents never see
  child values; "setting" a value creates a new layer (copy-on-write).

**Use it for** cross-cutting metadata that code can safely ignore: request/trace
IDs, authenticated user, locale. **Do not use it for** function parameters,
config, or dependencies (DB handles, loggers) — those belong in signatures and
struct fields where the compiler checks them. Litmus test: *if deleting the
value breaks correctness, it should have been a parameter.*

## Go 1.20/1.21 additions

```go
// WHY was I cancelled? (1.20)
ctx, cancel := context.WithCancelCause(parent)
cancel(errQuotaExceeded)
context.Cause(ctx)   // errQuotaExceeded  (ctx.Err() stays context.Canceled)

// Detach: keep values, drop cancellation (1.21) — e.g. audit log that must
// finish even though the request died. Give it its own timeout!
audit := context.WithoutCancel(requestCtx)

// Callback when a context ends (1.21); stop() unregisters it.
stop := context.AfterFunc(ctx, func() { conn.SetDeadline(time.Now()) })
defer stop()
```

All three in `04_modern_additions.go`.

## Idioms & rules

- `ctx` is the **first** parameter, named `ctx`:
  `func Fetch(ctx context.Context, id int) (User, error)`
- **Don't store a ctx in a struct**; pass it per call. Rare exception: a type
  that *is* an in-flight request (`http.Request` does this) or must bridge an
  API you don't control — document it loudly.
- **Never pass nil**; use `context.TODO()` while migrating.
- `defer cancel()` immediately after every `With*` that returns one.
- Long CPU-bound loops: poll `if err := ctx.Err(); err != nil { return err }`.
- Anything blocking: `select { case <-ctx.Done(): ... case ...: }`.
- Return `ctx.Err()` when you stop because of the context (wrap it with `%w`).

## Real-world integration

- **`net/http`**: every request has `r.Context()`, cancelled by the server when
  the client disconnects (or the handler returns). Pass it into every DB/RPC
  call your handler makes. On the client, `http.NewRequestWithContext` applies
  your timeout to the whole exchange. Runnable demo in `05_realworld_and_leaks.go`.
- **`os/signal.NotifyContext`**: turns Ctrl-C/SIGTERM into cancellation of one
  root context — the modern graceful-shutdown skeleton for services. The demo
  self-sends SIGTERM after 2s so it finishes on its own.
- **`exec.CommandContext(ctx, "sleep", "60")`**: kills the child process when
  ctx ends. One line, no zombie processes.
- **`database/sql`** (and virtually every driver/client library): the
  `QueryContext`/`ExecContext`/`PingContext` variants cancel the server-side
  query, not just your wait for it. Same convention in gRPC, Redis, S3 clients…
  If an API offers a `...Context` variant, the plain one is legacy.

## Common mistakes

| Mistake | Consequence | Fix |
|---|---|---|
| Forgetting `cancel()` | ctx + timer leak until parent dies | `defer cancel()` right after `With*` |
| `WithValue` for params/config | invisible, untyped, runtime-only API | real parameters / struct fields |
| Ignoring ctx in long loops | work continues long after cancellation | poll `ctx.Err()` / select on `Done()` |
| Setting a timeout but not checking `Err()` | you can't tell timeout from real failure | `errors.Is(err, context.DeadlineExceeded)` |
| Goroutine sends to a channel nobody reads after the caller timed out | **permanent goroutine leak** | buffered channel (cap 1) *and/or* worker selects on `Done()` |

The last one is the classic. `05_realworld_and_leaks.go` runs 20 timed-out
requests against a leaky fetcher and a fixed one and prints
`runtime.NumGoroutine()` before/after — you can watch the 20 zombies pile up,
then vanish with the fix.

## Run the examples

```sh
go run 01_cancellation_basics.go
go run 02_timeout_deadline.go
go run 03_values_request_scope.go
go run 04_modern_additions.go
go run 05_realworld_and_leaks.go   # finishes by itself (~5s, self-sends SIGTERM)
```

Every example terminates on its own — no real Ctrl-C needed.

For how contexts combine with worker pools, pipelines and fan-in/fan-out, see
[module 11 — Advanced Concurrency](../module11_concurrency_advanced/) (this
module doesn't repeat that material).

## Key takeaways

- Context = cancellation signal + deadline + request-scoped values, passed
  explicitly as the first parameter; deriving builds a tree and cancellation
  flows **down** it, never up.
- Callees obey via `select { case <-ctx.Done(): return ctx.Err() }` or by
  polling `ctx.Err()`; `Canceled` vs `DeadlineExceeded` says *why* it ended.
- `Background()` at the top, `TODO()` while migrating, never `nil`; always
  `defer cancel()`.
- `WithValue` is for metadata code may ignore (IDs, auth) with unexported key
  types — never for parameters or config.
- `Cause` explains cancellations, `WithoutCancel` detaches must-finish work,
  `AfterFunc` runs cleanup on cancellation.
- If a goroutine can outlive its caller with no one listening, it's a leak —
  buffer the result channel and/or select on `Done()`.

## Exercises

1. Write `fetchAll(ctx context.Context, urls []string) ([]string, error)` that
   fetches fake results concurrently (each "fetch" is a random 50–500ms sleep),
   but uses `context.WithTimeout` so the whole batch is abandoned after 300ms.
   Return partial results plus an error that wraps `ctx.Err()`. Verify with
   `runtime.NumGoroutine()` that no goroutines are left behind.
2. Build a tiny middleware chain: `withTrace(ctx)` stores a random trace ID
   using an unexported key type, and every layer logs through one `logf(ctx, ...)`
   helper. Then add a second package-like key type with the same underlying
   value (`const 0`) and prove the two never collide.
3. Extend the graceful-shutdown demo: on SIGTERM, give in-flight "jobs" up to
   1 second to finish by deriving a *new* `WithTimeout` context from
   `context.Background()` (why not from the cancelled one? — see
   `WithoutCancel` for an alternative). Print whether the drain finished or
   was cut off.
