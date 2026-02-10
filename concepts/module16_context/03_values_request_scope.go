// Module 16, Example 3 — context.WithValue: carrying REQUEST-SCOPED data
// (request IDs, authenticated user) through a call chain, with an unexported
// key type to prevent collisions — and why ctx values are NOT for parameters.
//
// Run with: go run 03_values_request_scope.go
package main

import (
	"context"
	"fmt"
)

// THE KEY TYPE TRICK
// ------------------
// ctx.Value does lookups by key using ==. If everyone used plain strings,
// two packages could both set "id" and silently clobber each other. So each
// package defines its OWN unexported key type: values of `type ctxKey int`
// in package A can never equal values of `type ctxKey int` in package B,
// even with the same underlying number. Collisions become impossible.
type ctxKey int

const (
	requestIDKey ctxKey = iota
	userKey
)

// Export typed accessors, not the keys. Callers never touch ctx.Value or the
// key directly — the type assertion and the "missing" case live in ONE place.
func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func requestIDFrom(ctx context.Context) string {
	// Value returns `any`; the comma-ok assertion handles both "not set"
	// (Value returns nil) and "set to the wrong type".
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return "unknown" // a sane default beats a panic for observability data
}

type user struct {
	Name string
	Role string
}

func withUser(ctx context.Context, u user) context.Context {
	return context.WithValue(ctx, userKey, u)
}

func userFrom(ctx context.Context) (user, bool) {
	u, ok := ctx.Value(userKey).(user)
	return u, ok
}

// ---- A call chain: the request ID flows through WITHOUT being a parameter ----
// handleRequest -> loadProfile -> queryDatabase. Note the signatures: none of
// them mention request IDs, yet every log line can include one. That is the
// whole point — cross-cutting metadata rides along; business data does not.

func handleRequest(ctx context.Context, path string) {
	logf(ctx, "handling %s", path)
	loadProfile(ctx)
}

func loadProfile(ctx context.Context) {
	logf(ctx, "loading profile")
	if u, ok := userFrom(ctx); ok {
		logf(ctx, "authenticated as %s (%s)", u.Name, u.Role)
	}
	queryDatabase(ctx, "SELECT * FROM profiles")
}

func queryDatabase(ctx context.Context, query string) {
	// Three frames down, still tagged with the request ID — perfect for
	// correlating logs/traces across services.
	logf(ctx, "executing %q", query)
}

// logf is a toy structured logger that stamps every line with the request ID
// pulled from ctx. Real codebases do exactly this (often via slog handlers).
func logf(ctx context.Context, format string, args ...any) {
	fmt.Printf("[req=%s] %s\n", requestIDFrom(ctx), fmt.Sprintf(format, args...))
}

func main() {
	// Middleware-style setup: decorate the context once at the boundary...
	ctx := context.Background()
	ctx = withRequestID(ctx, "req-7f3a")
	ctx = withUser(ctx, user{Name: "ada", Role: "admin"})

	// ...then everything downstream can read it.
	handleRequest(ctx, "/api/profile")

	// A context with NO value set: the accessor degrades gracefully.
	fmt.Println()
	handleRequest(context.Background(), "/healthz")

	// ---- Values are copy-on-write, layered like the cancellation tree -------
	// WithValue wraps the parent; lookups walk UP the chain until a key
	// matches. Children see parent values; parents never see child values.
	child := withRequestID(ctx, "req-override") // shadows, doesn't mutate
	fmt.Println()
	fmt.Println("parent still sees:", requestIDFrom(ctx))   // req-7f3a
	fmt.Println("child sees:       ", requestIDFrom(child)) // req-override

	// ---- What does NOT belong in a context value -----------------------------
	// WithValue is for request-scoped, cross-cutting metadata that functions
	// can safely IGNORE: request/trace IDs, auth identity, locale.
	//
	// It is NOT for:
	//   - function parameters (a query, a filename, an amount) — those belong
	//     in the signature where the compiler checks them;
	//   - dependencies/config (DB handles, loggers, feature flags) — inject
	//     those via struct fields or arguments;
	//   - optional knobs you're too lazy to thread through.
	//
	// Why so strict? ctx.Value is invisible in signatures, stringly-typed at
	// the edges, checked only at runtime, and each lookup is a linear walk.
	// If removing the value would BREAK correctness, it wasn't metadata —
	// make it a real parameter.
	fmt.Println("\nrule of thumb: values inform, parameters instruct")
}
