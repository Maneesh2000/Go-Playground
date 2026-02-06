# Module 10 — Concurrency Basics

## What you'll learn

- What goroutines are (and how they differ from OS threads)
- The `go` keyword — and why `main` must wait
- `sync.WaitGroup` for "wait until they're all done"
- Channels: unbuffered vs buffered, send/receive/close, `range`, comma-ok
- `select`: waiting on several channels, `default`, timeouts with `time.After`
- Building a small pipeline: generator → worker → printer
- Common deadlocks and how to read the panic message

## Goroutines

A **goroutine** is a function running concurrently with other functions,
managed by the **Go runtime**, not the operating system:

| | OS thread | goroutine |
|---|---|---|
| initial stack | ~1 MB, fixed | ~2–8 KB, grows/shrinks |
| creation cost | expensive syscall | cheap function call |
| scheduling | OS kernel | Go runtime (in-process) |
| realistic count | thousands | **millions** |

The runtime multiplexes many goroutines onto a few OS threads (the "M:N"
scheduler):

```
   G = goroutine   M = OS thread   P = processor slot (≈ one per CPU core)

   G1  G2  G3  G4  G5  G6  G7  G8      thousands of cheap goroutines
    \  |  /      \  |  /     |  /
    ┌──▼──┐      ┌──▼──┐   ┌─▼──┐
    │ P0  │      │ P1  │   │ P2 │      runtime schedules G's onto P's
    └──┬──┘      └──┬──┘   └─┬──┘
    ┌──▼──┐      ┌──▼──┐   ┌─▼──┐
    │ M0  │      │ M1  │   │ M2 │      few real OS threads
    └─────┘      └─────┘   └────┘
```

Starting one is a single keyword:

```go
go doWork()   // returns immediately; doWork runs concurrently
```

**Why main must wait:** when `main` returns, the program exits — running
goroutines are killed mid-flight, no cleanup. So you must *wait* for them,
and the two standard tools are `sync.WaitGroup` and channels.

## sync.WaitGroup

A counter you can wait on: `Add` before starting, `Done` when finished
(via `defer`), `Wait` blocks until the counter hits zero.

```go
var wg sync.WaitGroup
for i := 1; i <= 3; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        work(i)          // loop var is safe to capture since Go 1.22
    }()
}
wg.Wait() // blocks here until all three called Done
```

## Channels

A channel is a **typed pipe** between goroutines: `ch := make(chan int)`.
Send with `ch <- v`, receive with `v := <-ch`. *"Don't communicate by
sharing memory; share memory by communicating."*

### Unbuffered = synchronization

An unbuffered channel has no storage. A send **blocks until** a receiver is
ready (and vice versa) — the exchange is a rendezvous, a guaranteed
synchronization point between two goroutines:

```
   sender G1                            receiver G2
   ch <- 42 ──── blocks... ─────┐
                                │  handoff happens only when
                                ▼  BOTH sides are ready
                          v := <-ch
   (both goroutines continue only after the value changed hands)
```

### Buffered = a small queue

`make(chan int, 3)` holds up to 3 values. Sends block only when **full**;
receives block only when **empty**. Buffering decouples sender and receiver
speeds — it does *not* remove the need to think about blocking.

### close, range, comma-ok

- `close(ch)` — the **sender** signals "no more values". Never send after close (panic). Only the sender closes.
- `v, ok := <-ch` — `ok` is `false` once the channel is closed **and drained**; `v` is then the zero value.
- `for v := range ch` — receives until the channel is closed. The clean way to consume a stream.

## select

`select` waits on several channel operations and runs whichever is ready
first (random choice if several are ready):

```go
select {
case msg := <-newsCh:      // whichever channel
    fmt.Println(msg)       // delivers first...
case err := <-errCh:
    return err
case <-time.After(2 * time.Second):   // ...or a timeout
    return errors.New("timed out")
default:                   // optional: runs if NOTHING is ready
    fmt.Println("nothing yet")        // (makes select non-blocking)
}
```

## Pipelines

Channels chain naturally into assembly lines, each stage a goroutine:

```
  generator ──chan──► worker(s) ──chan──► printer
  (produce)          (transform)         (consume)
```

Each stage `close`s its output when done, and downstream `range` loops end
gracefully. See `04_pipeline.go`.

## Deadlocks

If **every** goroutine is blocked, the runtime kills the program:

```
fatal error: all goroutines are asleep - deadlock!

goroutine 1 [chan send]:
main.main()
        /path/to/main.go:12 +0x2c
```

Read it bottom-up: which line (`main.go:12`), doing what (`[chan send]` — a
send with no receiver). Classic causes: sending/receiving on an unbuffered
channel with no partner, forgetting to `close` a channel someone `range`s
over, `wg.Wait()` with a missing `Done`. See `05_deadlocks.go`.

## Run the examples

```sh
go run 01_goroutines_waitgroup.go
go run 02_channels.go
go run 03_select.go
go run 04_pipeline.go
go run 05_deadlocks.go
```

## Key takeaways

- Goroutines are runtime-scheduled and nearly free; `go f()` starts one.
- `main` exiting kills everything — always wait (WaitGroup or channels).
- Unbuffered channel = synchronizing handoff; buffered = small queue.
- Sender closes; receivers use `range` or comma-ok to detect the end.
- `select` multiplexes channels; `default` makes it non-blocking; `time.After` adds timeouts.
- A deadlock panic tells you each goroutine's blocked operation and line — read it.

## Exercises

1. Start 5 goroutines that each sleep a random 100–500 ms, then print their id.
   Use a `WaitGroup` so `main` waits. Then remove the `Wait()` and observe the
   difference.
2. Extend the pipeline in `04_pipeline.go` with a second worker reading from the
   same input channel (fan-out). Check that every number is processed exactly once,
   and use a `WaitGroup` to close the results channel only after both workers finish.
3. Write a `fetch(urls []string)` simulator: each "download" is a goroutine that
   sleeps then sends its result on a channel; `main` collects results with `select`
   plus a 300 ms `time.After` deadline, reporting which downloads were too slow.
