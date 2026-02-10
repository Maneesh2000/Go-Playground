# Module 17 — Networking with the net Package (TCP)

## What you'll learn

- What the `net` package covers: TCP, UDP, Unix sockets, IP, DNS lookups
- Network and address strings: `"tcp"`, `"tcp4"`, `"tcp6"`, `"host:port"`, `":8080"`
- The TCP server lifecycle: `Listen` → `Accept` loop → goroutine per connection → `Read`/`Write` → `Close`
- **Every method** of `net.Listener` and `net.Conn`, and what each does
- Deadlines (`SetDeadline` & friends) — absolute times, idle timeouts, `net.Error.Timeout()`
- The concrete types `*net.TCPListener` / `*net.TCPConn` and their TCP-only knobs
- Client side: `net.Dial`, `net.DialTimeout`, `net.Dialer` + `DialContext`
- Framing a byte stream into messages: newline delimiters vs length prefixes
- The error taxonomy: `io.EOF` vs `net.ErrClosed` vs timeouts
- Short tours of DNS helpers, `net.IP`, and how UDP differs

## The big picture

`net` is Go's portable interface to the operating system's sockets. One
package gives you:

| Area          | Entry points                                             |
|---------------|----------------------------------------------------------|
| TCP           | `net.Listen`, `net.Dial`, `net.TCPConn`, `net.TCPListener` |
| UDP           | `net.ListenPacket`, `net.DialUDP`, `net.UDPConn`         |
| Unix sockets  | same functions with network `"unix"` / `"unixgram"`      |
| IP / parsing  | `net.IP`, `net.ParseIP`, `net.SplitHostPort`, `net.JoinHostPort` |
| DNS           | `net.LookupHost`, `net.LookupAddr`, `net.Resolver`       |

Almost everything takes a **network string** and an **address string**:

- Network: `"tcp"` (IPv4 or IPv6), `"tcp4"`, `"tcp6"`, `"udp"`, `"unix"`, ...
- Address: `"host:port"` — `"localhost:8080"`, `"93.184.216.34:443"`,
  `"[2001:db8::1]:443"` (IPv6 needs brackets). For servers, `":8080"` means
  *all interfaces, port 8080*, and port `0` (`":0"`) means *give me any free
  port* — the examples use `"127.0.0.1:0"` so they never collide with a busy
  port.

### The TCP server lifecycle

```
                 net.Listen("tcp", ":8080")
                          │        (socket + bind + listen at the OS level)
                          ▼
                 ┌─────────────────┐
                 │  net.Listener   │
                 └────────┬────────┘
                          │  for { ... }
                          ▼
                 ln.Accept()  ◄────────────── blocks until a client connects
                          │
                          │ returns a fresh net.Conn per client
          ┌───────────────┼───────────────┐
          ▼               ▼               ▼
   go handle(conn)  go handle(conn)  go handle(conn)     one goroutine each
          │               │               │
      Read/Write      Read/Write      Read/Write          the conversation
          │               │               │
       conn.Close()   conn.Close()    conn.Close()
                          │
                     ln.Close()  ◄──── unblocks Accept: graceful shutdown
```

This *goroutine-per-connection* shape is idiomatic Go: blocking reads are
fine because goroutines are cheap, and the runtime multiplexes them onto
efficient OS event polling (epoll/kqueue) for you.

## net.Listen — creating a server

```go
ln, err := net.Listen("tcp", ":8080")   // (net.Listener, error)
```

`Listen` performs the classic `socket()` + `bind()` + `listen()` system
calls: it claims the port and tells the kernel to start queueing incoming
connections. If the port is taken you get an error ("address already in
use"). Pass port `0` and the OS picks a free ephemeral port — read it back
with `ln.Addr()`.

### net.Listener — all three methods

```go
type Listener interface {
    Accept() (Conn, error)   // block until a client connects; returns its Conn
    Close() error            // stop listening; unblocks a pending Accept
    Addr() Addr              // the address actually bound (crucial with :0)
}
```

- **`Accept()`** blocks until the kernel has a completed connection to hand
  over, then returns a `net.Conn` dedicated to that one client. Call it in a
  loop; the listener keeps listening while each returned conn lives its own
  life.
- **`Close()`** is the graceful-shutdown lever: a goroutine blocked in
  `Accept` immediately gets an error (`net.ErrClosed`), which your accept
  loop treats as "time to stop", not as a failure.
- **`Addr()`** returns the bound address — with `":0"` it's the only way to
  learn which port you got.

## net.Conn — every method

`Accept` (server side) and `Dial` (client side) both return this interface:

```go
type Conn interface {
    Read(b []byte) (n int, err error)
    Write(b []byte) (n int, err error)
    Close() error
    LocalAddr() Addr
    RemoteAddr() Addr
    SetDeadline(t time.Time) error
    SetReadDeadline(t time.Time) error
    SetWriteDeadline(t time.Time) error
}
```

- **`Read(b)`** — fills `b` with *whatever bytes have arrived*: at least 1,
  at most `len(b)`. TCP is a **byte stream with no message boundaries**: one
  Read may return half a message or three messages glued together. Blocks
  until data arrives, the peer closes (`io.EOF`), a deadline expires, or an
  error occurs. (`net.Conn` satisfies `io.Reader`, so `bufio`, `io.ReadFull`,
  `io.Copy` all work on it — that's how you fix the boundary problem.)
- **`Write(b)`** — sends bytes into the stream. Conceptually a partial write
  is possible on a socket, but Go's `Write` loops internally: if `err == nil`
  then `n == len(b)`, so you don't write retry loops. Write can still block
  (peer not reading, buffers full) — that's what write deadlines are for.
- **`Close()`** — sends FIN and frees the descriptor. The peer's blocked
  `Read` returns `io.EOF`. Always `defer conn.Close()` in the handler.
- **`LocalAddr()` / `RemoteAddr()`** — our end / their end of this
  connection (`ip:port`). `RemoteAddr` is what you log to identify a client.
- **`SetDeadline(t)` / `SetReadDeadline(t)` / `SetWriteDeadline(t)`** —
  arm timers for I/O. Three things to burn in:
  1. Deadlines are **absolute times, not durations**:
     `conn.SetReadDeadline(time.Now().Add(30 * time.Second))`.
  2. When a deadline passes, blocked (and future) Reads/Writes fail with a
     timeout error — but the **connection is still usable**; set a new
     deadline and carry on. Zero time (`time.Time{}`) clears it.
  3. For an **idle timeout**, re-arm the read deadline *before every Read* —
     "at most 30s of silence per read", not "30s total".
     `SetDeadline` sets both directions at once.

Detecting a timeout, the supported way:

```go
var nerr net.Error
if errors.As(err, &nerr) && nerr.Timeout() {
    // deadline expired — retry, close, or report
}
```

## Concrete types: \*net.TCPListener and \*net.TCPConn

`Listen("tcp", ...)` really returns a `*net.TCPListener`, and conns are
`*net.TCPConn`. Type-assert when you need TCP-specific extras:

```go
tcpConn := conn.(*net.TCPConn)
tcpConn.SetKeepAlive(true)                    // enable TCP keep-alive probes
tcpConn.SetKeepAlivePeriod(30 * time.Second)  // probe interval
tcpConn.SetNoDelay(true)                      // the default in Go — see below
```

- **Keep-alive** makes the kernel probe an idle connection so a silently
  vanished peer (crashed machine, dropped cable) is eventually detected.
  Since Go 1.23 there's finer control via `net.KeepAliveConfig` (idle time,
  interval, probe count) through `TCPConn.SetKeepAliveConfig` and the
  `KeepAliveConfig` fields on `net.Dialer`/`net.ListenConfig`.
- **`SetNoDelay` / Nagle's algorithm**: Nagle batches small writes to reduce
  tiny-packet overhead, at a latency cost. Go sets `TCP_NODELAY` (no
  batching, send immediately) **by default** — right for request/response
  protocols. `SetNoDelay(false)` re-enables Nagle if you truly want batching.
- **`net.ResolveTCPAddr("tcp", "host:port")`** parses/resolves an address
  into a `*net.TCPAddr` (`IP`, `Port`, `Zone` fields) for the typed APIs
  `net.ListenTCP` / `net.DialTCP`. Plain address strings with
  `Listen`/`Dial` are fine for almost everything; reach for the typed form
  when you need to inspect or build addresses programmatically.

## The client side: Dial and friends

```go
conn, err := net.Dial("tcp", "example.com:80")            // no time bound!
conn, err := net.DialTimeout("tcp", addr, 3*time.Second)  // bounded

d := &net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}
conn, err := d.DialContext(ctx, "tcp", addr)              // bounded + cancelable
```

`Dial` resolves the host if needed and runs the TCP handshake — with **no
time limit**, which can mean minutes against a black-holed host. Production
code uses `net.Dialer`: a reusable policy bundle (connect `Timeout`,
`KeepAlive` period, `LocalAddr`, custom `Resolver`, ...) whose `DialContext`
also aborts if the `context` is canceled or its deadline passes — tying the
connection attempt to a request's lifetime.

## Framing: turning a stream into messages

TCP hands you a featureless byte river; *you* decide where messages start
and end. Writes on one side do **not** map 1:1 to Reads on the other:

```
sender:      Write("GET a\n")   Write("SET b 1\n")    Write("QUIT\n")
                  └──────────────┬───────────────────────┘
wire:         ...G E T  a \n S E T  b  1 \n Q U I T \n...   one byte stream
                  └──────┬──────┴────────┬───────┬────┘
receiver:      Read → "GET a\nSE"   Read → "T b 1\nQUI"   Read → "T\n"  (!!)
```

Two classic fixes:

1. **Delimiter framing** — one message per line, split on `'\n'`:

   ```go
   r := bufio.NewReader(conn)
   line, err := r.ReadString('\n')   // reads across packets until '\n'
   ```

   (`bufio.Scanner` works too.) Simple and debuggable with `nc`, but the
   payload must never contain the delimiter.

2. **Length-prefix framing** — a fixed header says how many bytes follow:

   ```
   [ 4-byte big-endian length = N ][ N payload bytes ][ 4-byte length ]...
   ```

   ```go
   var hdr [4]byte
   io.ReadFull(conn, hdr[:])                    // exactly 4 bytes
   n := binary.BigEndian.Uint32(hdr[:])
   payload := make([]byte, n)
   io.ReadFull(conn, payload)                   // exactly n bytes
   ```

   Handles arbitrary binary payloads. Note `io.ReadFull`, not bare `Read` —
   it loops until the buffer is full, exactly what "read exactly k bytes"
   needs. Always sanity-bound the decoded length.

## The error taxonomy

| Error | Meaning | Check with |
|---|---|---|
| `io.EOF` | Peer closed cleanly. Normal end of conversation. | `errors.Is(err, io.EOF)` |
| timeout | A deadline expired. Conn still usable. | `errors.As(err, &nerr) && nerr.Timeout()` |
| `net.ErrClosed` | *We* closed the listener/conn; pending calls unblocked. Expected during shutdown. | `errors.Is(err, net.ErrClosed)` |
| anything else | Real failure (reset, refused, no route...). | log / wrap / return |

`net.ErrClosed` is the error behind the famous *"use of closed network
connection"* message you'll see during shutdown — match the sentinel with
`errors.Is`, never the string.

## DNS and utility helpers, briefly

```go
addrs, err := net.LookupHost("example.com")      // name -> IPs
names, err := net.LookupAddr("8.8.8.8")          // IP -> names (reverse)
host, port, err := net.SplitHostPort("[::1]:80") // safe parsing (IPv6!)
addr := net.JoinHostPort("::1", "80")            // -> "[::1]:80"
ip := net.ParseIP("192.168.1.10")                // net.IP ([]byte), nil if bad
```

Always use `JoinHostPort`/`SplitHostPort` instead of string concatenation —
they handle the IPv6 brackets for you. (Newer code often prefers the
`net/netip` package's value-type `netip.Addr`, but `net.IP` is what `net`'s
own APIs speak.)

## UDP in one breath

UDP is **datagrams, not a stream**: each send is one self-contained packet
(framing solved!), but delivery, order, and non-duplication are *not*
guaranteed, and there are no connections or Accept:

```go
pc, err := net.ListenPacket("udp", ":9000")        // net.PacketConn
n, clientAddr, err := pc.ReadFrom(buf)             // one packet + who sent it
pc.WriteTo(buf[:n], clientAddr)                    // reply to that address
```

One socket serves all peers; `ReadFrom`/`WriteTo` carry the address that
`Accept`/`Conn` would otherwise encapsulate.

## Relation to net/http

`http.Server` is built on *exactly* the loop from this module: it calls
`net.Listen`, runs an `Accept` loop, starts a goroutine per connection, and
parses HTTP off each `net.Conn` — then hands you the parsed request in a
handler. When you configure `http.Server.ReadTimeout` or pass a custom
listener to `srv.Serve(ln)`, you're turning the knobs you now know by name.
Everything in module 17 is the floor that `net/http` stands on.

## Run the examples

Each file is self-contained: it starts a server in goroutines and runs the
client(s) in `main`, so it exercises real TCP over loopback and exits on its
own — no second terminal needed. All servers bind `127.0.0.1:0` (ephemeral
ports), so nothing collides with services on your machine.

```sh
go run 01_tcp_echo_server.go
go run 02_line_protocol_server.go
go run 03_deadlines_and_errors.go
go run 04_length_prefixed_frames.go
go run 05_tcp_client_dialer.go
```

## Key takeaways

- Server lifecycle: `Listen` → loop `Accept` → `go handleConn` → `Read`/`Write` → `Close`; closing the listener unblocks `Accept` for graceful shutdown.
- `net.Conn` is the whole conversation: `Read`/`Write`/`Close`, `LocalAddr`/`RemoteAddr`, and the three deadline setters.
- TCP is a byte stream — `Read` has no message boundaries; frame with delimiters (`bufio`) or length prefixes (`binary` + `io.ReadFull`).
- Deadlines are absolute `time.Time`s; re-arm per Read for idle timeouts; detect with `errors.As(err, &nerr) && nerr.Timeout()`.
- Classify errors: `io.EOF` = peer said goodbye, `net.ErrClosed` = our own shutdown, `Timeout()` = deadline — only the rest are real failures.
- Clients: prefer `net.Dialer.DialContext` (or at least `DialTimeout`) — bare `Dial` waits unboundedly.
- Use `":0"` + `ln.Addr()` in tests and demos; use `JoinHostPort`/`SplitHostPort` for address strings.

## Exercises

1. Extend the KV server in `02_line_protocol_server.go` with `DEL key` and
   `KEYS` commands, and add a write deadline before each reply. Then add a
   connection counter: log "client 3 connected / disconnected" using
   `RemoteAddr()` and a `sync/atomic` counter.
2. Build `proxy.go`: listen on an ephemeral port, and for every accepted
   conn, `Dial` the echo server from example 1 and shovel bytes both ways
   with two `io.Copy` goroutines. Prove that a client talking to the proxy
   still gets its echoes. What closes what when the client hangs up?
3. Write a length-prefixed *chat broadcast* server: keep a mutex-protected
   set of connected clients, and re-broadcast every frame received from one
   client to all the others. Have three in-process clients exchange a few
   messages, then shut down cleanly via `ln.Close()` — make sure you handle
   `net.ErrClosed` without logging it as an error.
