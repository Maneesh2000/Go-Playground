# Module 13 — Go for Unix & Systems Work

## What you'll learn

- Opening files precisely with `os.OpenFile` flags and permission bits
- Path manipulation and directory-tree walking with `path/filepath`
- Environment variables, command-line arguments, and the `flag` package
- Running external commands with `os/exec`: capturing output, piping one
  command into another, reading exit codes
- Handling signals (`SIGINT`/`SIGTERM`) and the graceful-shutdown pattern
- Treating stdin/stdout/stderr as `io` streams — building pipe-friendly CLI
  tools and detecting whether stdin is a pipe or a terminal
- Temp files and a note on file locking
- A mini `grep` clone that ties it all together
- When you'd drop to `syscall`/`golang.org/x/sys`, and cross-compiling with
  `GOOS`/`GOARCH`
- Blocking vs non-blocking file descriptors (`O_NONBLOCK`, `EAGAIN`) and the
  readiness APIs built on them: `select`/`poll` → `epoll` (Linux) / `kqueue`
  (macOS) / IOCP (Windows)
- How Go's **netpoller** uses epoll/kqueue to park and unblock *goroutines*
  instead of OS threads — why blocking-style `conn.Read` code scales to
  thousands of connections, and how `SetReadDeadline` forcibly unblocks a
  read
- Writing a raw single-threaded `epoll` echo server (Linux) to see what the
  runtime does for you

Go is a fantastic language for the kind of work you'd otherwise do in shell,
Python, or C: CLIs, daemons, glue tools. Single static binary, instant
startup, real concurrency, and the whole POSIX toolbox in the standard
library.

## Files, flags, and permission bits

`os.Open` is read-only and `os.Create` truncates. For everything else there is
`os.OpenFile(name, flags, perm)`:

```go
f, err := os.OpenFile("app.log",
    os.O_APPEND|os.O_CREATE|os.O_WRONLY, // OR the flags together
    0o644)                               // permissions if the file is created
```

| Flag         | Meaning                          |
|--------------|----------------------------------|
| `O_RDONLY`   | read only                        |
| `O_WRONLY`   | write only                       |
| `O_RDWR`     | read + write                     |
| `O_CREATE`   | create if it doesn't exist       |
| `O_APPEND`   | every write goes to the end      |
| `O_TRUNC`    | empty the file on open           |
| `O_EXCL`     | with `O_CREATE`: fail if exists (atomic "create only") |

Permissions are classic Unix octal: `0o644` = owner `rw-`, group `r--`, other
`r--`; `0o755` for executables/directories. Your process's **umask** may mask
bits off. `os.Stat` returns `FileInfo` (size, mode, mod time); `os.Chmod`
changes modes.

## Paths and walking trees

Use `path/filepath` for OS-specific filesystem paths (it knows about `\` on
Windows); the plain `path` package is only for forward-slash paths like URLs.

```go
filepath.Join("etc", "app", "config.yaml") // "etc/app/config.yaml" — never "+"
filepath.Ext("photo.jpeg")                 // ".jpeg"
filepath.Base("/a/b/c.txt"), filepath.Dir("/a/b/c.txt")
filepath.Abs("relative/path")
```

Walking a tree — `filepath.WalkDir` calls your function for every file and
directory (it's the modern, faster replacement for `filepath.Walk`):

```go
filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
    if d.IsDir() && d.Name() == ".git" {
        return fs.SkipDir // don't descend into .git
    }
    ...
    return nil
})
```

## Environment and arguments

- `os.Getenv("HOME")` — empty string if unset; `os.LookupEnv` tells you *if*
  it was set; `os.Setenv` / `os.Environ` for writing/listing.
- `os.Args` — raw arguments, `os.Args[0]` is the program name.
- The `flag` package layers option parsing on top; `flag.Args()` gives you the
  positional arguments left over after the flags.

## Running external commands: os/exec

```go
out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
```

- `.Output()` → stdout as `[]byte`; `.CombinedOutput()` → stdout+stderr;
  `.Run()` → just the error.
- Arguments are passed as a **list** — there is no shell, so no quoting bugs
  and no injection. If you truly need shell features, run
  `exec.Command("sh", "-c", ...)` and treat the input as dangerous.
- Exit codes: a failed command returns an `*exec.ExitError`; use `errors.As`
  and `.ExitCode()`.
- Pipes between commands: connect `cmd1.StdoutPipe()` to `cmd2.Stdin`, exactly
  like `ls | wc -l`.
- `exec.CommandContext(ctx, ...)` kills the process when the context is
  cancelled — timeouts for free.

## Signals and graceful shutdown

A Unix process is asked to die politely with `SIGTERM` (or Ctrl-C → `SIGINT`).
A well-behaved server finishes in-flight work, closes connections, flushes
logs — *then* exits. The modern pattern is one line:

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
```

```
 SIGTERM/SIGINT arrives
        │
        ▼
 signal.NotifyContext ──► ctx.Done() closes
        │                     │
        │            ┌────────┼────────────┐
        │            ▼        ▼            ▼
        │       HTTP server  workers   cron loops     (everything that
        │       srv.Shutdown  return    stop          watches ctx)
        │            │
        ▼            ▼
   deadline for   cleanup: flush logs, close DB, remove pidfile
   stragglers          │
                       ▼
                     exit 0
```

Everything that took the `ctx` (from Module 11!) unwinds at once. Pair it with
`http.Server.Shutdown(timeoutCtx)` for HTTP.

## stdin/stdout/stderr — pipe-friendly tools

`os.Stdin`, `os.Stdout`, `os.Stderr` are just `*os.File`s, i.e. Readers and
Writers. The Unix contract for a good CLI tool:

- **data** goes to stdout, **diagnostics** go to stderr (so
  `mytool | grep x` doesn't grep your error messages);
- read stdin when no file argument is given, so the tool works in pipelines;
- exit `0` on success, non-zero on failure (grep tradition: `1` = no matches,
  `2` = real error).

Detecting whether stdin is a pipe or an interactive terminal:

```go
fi, _ := os.Stdin.Stat()
isPipe := fi.Mode()&os.ModeCharDevice == 0 // char device = terminal
```

## Temp files and locking

- `os.CreateTemp(dir, "app-*.tmp")` creates a uniquely-named file safely (no
  race with other processes guessing names); `os.MkdirTemp` for directories.
  Clean up with `defer os.Remove(...)`.
- The classic "write temp file, then `os.Rename` over the target" gives you an
  atomic file replace on the same filesystem.
- **File locking** has no portable stdlib API. On Unix you can use
  `syscall.Flock` (advisory locks); a simple cross-platform trick is an
  `O_CREATE|O_EXCL` lockfile. For serious use, reach for
  `golang.org/x/sys/unix` or a small library.

## syscall vs golang.org/x/sys

The stdlib `syscall` package is frozen — kept for compatibility. If you need
raw system calls (ioctls, `epoll`, extended attributes, per-OS quirks), use
`golang.org/x/sys/unix` instead. You will rarely need either: `os`, `os/exec`,
`net`, and `os/signal` wrap the common 99%.

## Blocking, non-blocking I/O and epoll — how Go unblocks goroutines

### The problem: a blocking read() parks a whole OS thread

By default a file descriptor is **blocking**: call `read()` when no data is
available and the kernel parks the calling *thread* until some arrives. For
one fd that's fine. For a server it's a scaling wall: with the classic
"one thread per connection" model, 10,000 mostly-idle connections mean
10,000 mostly-idle threads — each with a stack, a scheduler slot, and
context-switch cost.

```
  BLOCKING MODEL (1 thread per conn)        EVENT-DRIVEN MODEL (1 thread)

  conn1 ──► thread1 ─┐ parked in read()     conn1 ─┐
  conn2 ──► thread2 ─┤ parked in read()     conn2 ─┼─► epoll/kqueue ──► one
  conn3 ──► thread3 ─┤ parked in read()     conn3 ─┤   "interest list"   thread
   ...        ...    │  ...                  ...   │        │            handles
  connN ──► threadN ─┘ parked in read()     connN ─┘        ▼            only the
                                                    "fds 3 and 17 are    READY fds
  N conns = N threads (RAM, ctx switches)    ready" — wake up once
```

### Non-blocking mode: EAGAIN instead of waiting

Any fd can be flipped to **non-blocking** with the `O_NONBLOCK` flag
(`fcntl`; in Go: `syscall.SetNonblock(fd, true)`). Now `read()` with no data
returns *immediately* with the error `EAGAIN` ("try again"; `EWOULDBLOCK` is
the same value on modern systems). Check for it with
`errors.Is(err, syscall.EAGAIN)`.

That alone doesn't scale either — if you just retry in a loop you're
**busy-polling** at 100% CPU. `EAGAIN` is only half the mechanism: you also
need the kernel to *tell you when to try again*. That's what readiness APIs
do: "here are the fds I care about — wake me when one becomes readable or
writable."

### Readiness APIs: select/poll → epoll / kqueue / IOCP

The historical APIs are `select` and `poll`: you pass the whole fd list on
*every* call and the kernel scans all of them — O(n) per wakeup, and
`select` tops out at 1024 fds. The modern replacements keep the interest
list *inside the kernel* so each wakeup only reports the fds that are
actually ready:

- **epoll** (Linux): `epoll_create1()` makes the interest list,
  `epoll_ctl(EPOLL_CTL_ADD/MOD/DEL)` edits it, `epoll_wait()` blocks until
  something is ready and returns *only the ready fds*.
- **kqueue** (macOS/BSD — what this machine uses): same idea, one
  `kevent()` call both edits the list and waits.
- **IOCP** (Windows): completion-based rather than readiness-based ("this
  read *finished*" instead of "you *can* read now"), same scalability goal.

**Level- vs edge-triggered** (epoll offers both): level-triggered (the
default) keeps reporting an fd as ready for as long as unread data remains —
forgiving, you may read only part of it and get woken again. Edge-triggered
(`EPOLLET`) fires only on the *transition* from not-ready to ready, so you
must drain the fd until `EAGAIN` every single time or you'll never be woken
for the leftover bytes. Edge-triggered saves redundant wakeups but is easy
to get wrong; Go's runtime uses edge-triggered and always drains.

| API      | OS          | Interest list      | Wakeup cost         | Limit    |
|----------|-------------|--------------------|---------------------|----------|
| `select` | everywhere  | passed every call  | O(n) scan           | 1024 fds |
| `poll`   | everywhere  | passed every call  | O(n) scan           | none     |
| `epoll`  | Linux       | in-kernel          | O(ready)            | none     |
| `kqueue` | macOS/BSD   | in-kernel          | O(ready)            | none     |
| IOCP     | Windows     | in-kernel (completion) | O(completed)    | none     |

### How Go uses this: the netpoller

Here's the payoff. In Go you *never* see `EAGAIN` from a `net.Conn` — yet
there is no thread parked under your blocked `Read` either. The runtime's
**netpoller** sits between goroutines and the kernel:

1. Every socket (`net.Conn`, `net.Listener`) and pollable `*os.File` (pipes,
   ttys) is put into **non-blocking mode** and **registered with
   epoll/kqueue** by the runtime when it's created.
2. A goroutine calls `conn.Read`. Data ready? Great, return it. Not ready?
   The read got `EAGAIN`, and the runtime **parks the goroutine**
   (`gopark`) — a cheap in-memory operation, ~KBs of stack, *no OS thread
   blocks*. The thread immediately picks up another runnable goroutine.
3. The scheduler periodically (and when idle) calls `epoll_wait`/`kevent`.
   When the kernel reports "fd 42 is readable", the netpoller looks up the
   goroutine parked on fd 42 and **readies exactly that goroutine**. Its
   `Read` resumes, retries the syscall, gets the data, returns.

```
   goroutine A ──Read()──► not ready ──► gopark ─┐
   goroutine B ──Read()──► not ready ──► gopark ─┼── parked goroutines
   goroutine C ──Read()──► data!  ✓ returns      │   (cheap, no threads)
                                                 │
              ┌──────────────────────────────────┘
              ▼
        ┌───────────┐   epoll_wait /   ┌────────┐   data arrives   ┌────────┐
        │ netpoller │ ◄── kevent ────  │ kernel │ ◄─────────────── │ fds    │
        └───────────┘  "fd of A ready" └────────┘                  └────────┘
              │
              ▼
        scheduler readies goroutine A  ──►  A's Read returns data
```

So you write simple, sequential, blocking-*style* code — and underneath, Go
runs the same event loop nginx or Node.js use. Goroutines are "unblocked" by
the netpoller. This is *the* reason a Go server handles tens of thousands of
concurrent connections with a handful of threads.

**Deadlines ride the same mechanism.** `conn.SetReadDeadline(t)` arms a
timer inside the poller. If the fd never becomes readable, the timer fires
and the poller readies the parked goroutine anyway — `Read` returns an error
satisfying `errors.Is(err, os.ErrDeadlineExceeded)` and `net.Error` with
`Timeout() == true`. That's how a blocked read is *forcibly* unblocked
without data, EOF, or closing the conn (`context` cancellation on sockets is
built from this too). Pipes from `os.Pipe` support `SetReadDeadline` the
same way, because they're registered with the same poller; regular disk
files are not (a disk read is always "ready").

### When you'd touch this directly (rarely)

`net.Conn` already does all of the above for you. You only go lower when
wrapping *foreign* fds: a device, a raw socket from `syscall.Socket`, an fd
passed in from a parent process. Then the tools are `syscall.SetNonblock`,
`os.NewFile(uintptr(fd), "name")` (which registers a pollable fd with the
runtime poller — you get deadlines for free), and `golang.org/x/sys/unix`
for raw `epoll`/`kqueue` when you're building your own event loop (see
`08_epoll_linux.go` for what that looks like — and why you probably don't
want to maintain it).

## Cross-compiling

Go cross-compiles by setting two environment variables — no toolchains, no
Docker:

```bash
GOOS=linux   GOARCH=amd64 go build -o app-linux-amd64 .
GOOS=linux   GOARCH=arm64 go build -o app-linux-arm64 .
GOOS=darwin  GOARCH=arm64 go build -o app-mac .
GOOS=windows GOARCH=amd64 go build -o app.exe .
go tool dist list   # all supported GOOS/GOARCH pairs
```

Caveat: cgo is disabled by default when cross-compiling — pure-Go code (most
code) just works.

## Run the examples

```bash
cd module13_unix_systems

go run 01_files_paths_env.go
go run 02_exec_commands.go
go run 03_signals_shutdown.go      # press Ctrl-C to trigger the shutdown, or wait 10s
go run 04_stdin_streams.go         # interactive mode
echo "hello pipe" | go run 04_stdin_streams.go   # pipe mode

# the mini-grep: pattern, then files (or stdin)
go run 05_minigrep.go func 05_minigrep.go
echo -e "cat\ndog\ncow" | go run 05_minigrep.go -n 'c.t'

go run 06_nonblocking_fd.go        # EAGAIN, blocking vs non-blocking, pipe deadlines
go run 07_netpoller_unblock.go     # 1000 goroutines parked & unblocked by the netpoller

# 08 is LINUX-ONLY (epoll doesn't exist on macOS). From this Mac:
GOOS=linux GOARCH=amd64 go build 08_epoll_linux.go            # cross-compile check
docker run --rm -v $PWD:/src -w /src golang:1.26 go run 08_epoll_linux.go
# (or copy it to any Linux box and `go run` it there; it self-tests and exits)
```

## Key takeaways

- `os.OpenFile` + OR'd flags + octal perms give you exact control; `0o644`
  and `0o755` cover most cases.
- Always build paths with `filepath.Join`; walk trees with `filepath.WalkDir`
  and `fs.SkipDir`.
- `os/exec` passes args as a list — no shell, no injection; `errors.As` +
  `*exec.ExitError` for exit codes; `StdoutPipe` to chain commands.
- Graceful shutdown = `signal.NotifyContext` + everything respecting the ctx.
- Data → stdout, errors → stderr, read stdin when no args: your tool becomes a
  good pipeline citizen.
- `GOOS=linux GOARCH=arm64 go build` — cross-compiling is that short.
- A blocking `read()` parks an OS *thread*; non-blocking fds return `EAGAIN`
  instead — the raw signal that readiness APIs (`epoll` on Linux, `kqueue`
  on macOS) turn into "wake me when it's ready".
- Go's netpoller registers every socket/pipe fd with epoll/kqueue, parks the
  *goroutine* (not the thread) on `EAGAIN`, and readies exactly that
  goroutine when the kernel reports the fd ready — blocking-style code,
  event-driven scalability.
- `SetReadDeadline` is the poller forcibly unblocking you: a timer readies
  the parked goroutine and `Read` returns `os.ErrDeadlineExceeded`.
- You almost never write epoll yourself in Go — `net.Conn` *is* the event
  loop, done right.

## Exercises

1. Write `lsbig`: walk a directory tree and print the 5 largest files with
   sizes, skipping any directory named `node_modules` or `.git`.
2. Extend the mini-grep with a `-c` flag (print only the count of matching
   lines) and a `-i` flag (case-insensitive — hint: prefix the pattern with
   `(?i)`).
3. Write a `retry` tool: `retry 3 somecmd args...` runs the command up to 3
   times, stopping on the first success, and exits with the last attempt's
   exit code. Use `os/exec`, `errors.As`, and `os.Exit`.
4. Busy-polling, felt: change `06_nonblocking_fd.go` so that instead of
   printing `EAGAIN` once, it retries the non-blocking read in a tight loop
   until data arrives (write from a goroutine after 2 seconds). Count the
   retries and watch CPU with `top` — then add a 1ms `time.Sleep` to the
   loop and compare. That gap between "spin" and
   "sleep-poll" is exactly what `epoll_wait` closes.
5. Extend `08_epoll_linux.go` to handle `EPOLLOUT`: make the echo handle
   short writes — if `syscall.Write` returns `EAGAIN` or writes fewer bytes
   than read, buffer the remainder per-fd, register the fd for `EPOLLOUT`
   with `EPOLL_CTL_MOD`, flush when the writable event fires, then switch
   the registration back to `EPOLLIN` only. (This is the part every real
   event loop must get right.)
