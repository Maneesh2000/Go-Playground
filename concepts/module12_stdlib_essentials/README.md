# Module 12 — Standard Library Essentials

## What you'll learn

- `fmt` formatting verbs you'll actually use (`%v %+v %#v %T %q %d %x %f %s %w`)
- Text wrangling with `strings`, `strconv`, and `unicode`
- `time`: `Time` vs `Duration`, the famous reference-date formatting, timers and tickers
- Files and streams with `os`, `io`, `bufio` — and the `io.Reader`/`io.Writer` composition philosophy
- `encoding/json`: struct tags, `omitempty`, and decoding unknown JSON into `map[string]any`
- `net/http`: a client with a timeout and a server using Go 1.22 `METHOD /path/{param}` routing, plus middleware in ~10 lines
- Quick tours of `regexp`, `sort`/`slices`/`cmp`, `log/slog`, and `flag`

The standard library is Go's superpower: most production services are built
with remarkably few third-party dependencies. This module is a guided tour of
the packages you will touch every single day.

## fmt — formatting verbs

```go
fmt.Printf("%v", u)   // default format             {Ada 36}
fmt.Printf("%+v", u)  // + field names               {Name:Ada Age:36}
fmt.Printf("%#v", u)  // Go-syntax representation    main.User{Name:"Ada", Age:36}
fmt.Printf("%T", u)   // the TYPE of the value       main.User
fmt.Printf("%q", "hi")// quoted string               "hi"
fmt.Printf("%5.2f", π)// width 5, precision 2        " 3.14"
fmt.Errorf("read cfg: %w", err) // %w WRAPS an error (see module 08)
```

`fmt.Sprintf` returns the string instead of printing; `fmt.Fprintf(w, ...)`
writes to any `io.Writer` — the same verbs everywhere.

## strings, strconv, unicode

- `strings` — pure functions on strings: `Contains`, `Split`, `Join`, `TrimSpace`,
  `ReplaceAll`, `HasPrefix`, `Fields`. For building strings in a loop use
  `strings.Builder` (avoids quadratic `+=` copying).
- `strconv` — string ↔ number: `Atoi`, `Itoa`, `ParseFloat`, `ParseBool`,
  `FormatInt`, `Quote`. Conversions can fail, so most return `(value, error)`.
- `unicode` — character classes: `IsUpper`, `IsDigit`, `IsSpace`. Remember:
  ranging over a string yields **runes** (Unicode code points), while `len()`
  counts **bytes**.

## time — the reference date

Go formats time with a *reference date* instead of `YYYY-MM-DD` patterns.
The magic moment is:

```
Mon Jan 2 15:04:05 MST 2006     (think: 01/02 03:04:05PM '06 -0700)
```

You write the reference date the way you want YOUR output to look:

```go
t.Format("2006-01-02 15:04")        // 2026-07-04 09:30
t.Format(time.RFC3339)              // 2026-07-04T09:30:00Z
time.Parse("02/01/2006", "04/07/2026") // parsing uses the same trick
```

`time.Duration` is just an `int64` of nanoseconds with nice constants:
`3 * time.Second`, `d.Seconds()`, `time.Since(start)`. Timers fire once
(`time.After`, `time.NewTimer`); tickers fire repeatedly (`time.NewTicker`)
— always `Stop()` a ticker or it leaks.

## os, io, bufio — the composition philosophy

Go I/O is built on two tiny interfaces:

```go
type Reader interface { Read(p []byte) (n int, err error) }
type Writer interface { Write(p []byte) (n int, err error) }
```

Because everything speaks Reader/Writer, you snap pieces together like pipes:

```
┌───────────┐    ┌────────────────┐    ┌───────────────┐    ┌───────────┐
│  os.File  │───►│ bufio.Reader   │───►│ your parsing  │    │ any source│
│ (raw disk │    │ (adds an in-   │    │ code reads    │    │ works the │
│  reads)   │    │  memory buffer)│    │ lines cheaply │    │ same way  │
└───────────┘    └────────────────┘    └───────────────┘    └───────────┘

strings.NewReader("...")  ──┐
os.Stdin                  ──┼──► all satisfy io.Reader — your code can't tell
http resp.Body            ──┘    the difference, and that's the point
```

Key players: `os.Open`/`os.Create` (files), `bufio.Scanner` (line-by-line
reading), `io.Copy(dst, src)` (move bytes between any writer/reader),
`io.MultiWriter` (tee output to several places at once).

## encoding/json

```go
type User struct {
    Name  string `json:"name"`
    Email string `json:"email,omitempty"` // dropped from output when ""
    admin bool                            // unexported → invisible to json
}

data, err := json.Marshal(u)         // Go → JSON bytes
err = json.Unmarshal(data, &u)       // JSON → Go (note the &)
```

- Struct tags map Go names to JSON keys; `omitempty` skips zero values;
  `json:"-"` excludes a field entirely.
- Unknown/dynamic JSON? Decode into `map[string]any` and type-assert. JSON
  numbers arrive as `float64` — a classic gotcha.
- Streams: `json.NewDecoder(r).Decode(&v)` and `json.NewEncoder(w).Encode(v)`
  work directly on readers/writers (e.g. HTTP bodies) with no intermediate
  `[]byte`.

## net/http

Client — the zero-config `http.Get` has **no timeout**; real code sets one:

```go
client := &http.Client{Timeout: 5 * time.Second}
resp, err := client.Get(url)
defer resp.Body.Close() // ALWAYS close the body
```

Server — since Go 1.22, `ServeMux` patterns include the method and path
wildcards, killing most needs for a router dependency:

```go
mux := http.NewServeMux()
mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")            // pulled from the {id} wildcard
    fmt.Fprintf(w, "user %s", id)
})
```

Middleware is just a function that wraps a handler:

```go
func logging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        log.Printf("%s %s took %v", r.Method, r.URL.Path, time.Since(start))
    })
}
```

## regexp, sort/slices/cmp, slog, flag

- `regexp` — RE2 syntax (no catastrophic backtracking, guaranteed linear
  time). Compile once (`regexp.MustCompile`) at package level, reuse everywhere.
- `slices` + `cmp` (Go 1.21+) — the modern way: `slices.Sort(s)`,
  `slices.SortFunc(people, func(a, b Person) int { return cmp.Compare(a.Age, b.Age) })`,
  `slices.Contains`, `slices.BinarySearch`. The older `sort` package still
  works but `slices` is clearer and generic.
- `log/slog` — structured logging: `slog.Info("user login", "user", name, "attempts", 3)`
  emits key=value pairs (or JSON with `slog.NewJSONHandler`), which log
  aggregators can index. Prefer it over bare `log.Printf` in services.
- `flag` — command-line flags: `port := flag.Int("port", 8080, "listen port")`,
  then `flag.Parse()`, then `*port`. Enough for most CLIs.

## Run the examples

```bash
cd module12_stdlib_essentials

go run 01_fmt_strings_strconv.go
go run 02_time_files_io.go
go run 03_json.go
go run 04_http.go        # starts a server on :8090, calls it, then exits
go run 05_regexp_sort_slog_flag.go -name=Gopher -verbose
```

## Key takeaways

- Learn `%v %+v %T %q %w` and you know 90% of `fmt`.
- The `time` reference date is `2006-01-02 15:04:05` — write your layout using
  those exact numbers.
- Everything is an `io.Reader`/`io.Writer`; compose small pieces instead of
  loading whole files when streams will do.
- JSON: exported fields + struct tags; `map[string]any` for unknown shapes;
  numbers decode as `float64`.
- Go 1.22 `ServeMux` (`"GET /users/{id}"`) + a middleware func covers most web
  services without any framework.
- Reach for `slices`/`cmp` over `sort`, and `slog` over `log`, in new code.

## Exercises

1. Write a program that reads lines from a file with `bufio.Scanner` and prints
   each line prefixed with its number and byte length — then swap the file for
   `strings.NewReader` without changing the counting code (accept an `io.Reader`).
2. Define an `Event` struct with `Name`, `At time.Time`, and optional `Notes`.
   Marshal a slice of events to indented JSON, write it to a file, read it back,
   and print events newer than a given time.
3. Extend the HTTP example with a `DELETE /notes/{id}` route and a middleware
   that rejects requests missing an `X-API-Key` header with status 401.
