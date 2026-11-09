---
name: go
description: >
  Idiomatic Go - package and interface design, error wrapping, table-driven
  tests, generics, the modern standard library (slices/maps/cmp/errors.Join),
  current syntax, and logging discipline. Use when writing, reviewing, or
  refactoring any Go code, especially code drifting toward Java/Spring shapes
  (deep layer trees, generic repositories, heavy frameworks) - even if the user
  never says "idiomatic". Distilled from spf13/go-skills and adapted to this repo.
license: Apache-2.0
---

# Idiomatic Go

Write Go that is **boring in the best way** - direct, predictable, obvious on the
first read. *Clear is better than clever.* The common failure mode (and the LLM
default) is importing Java/Spring habits - deep layer trees, generic
repositories, heavy frameworks, manual worker pools - into a language built for
flat, simple, discoverable APIs. When in doubt, delete the abstraction.

## Core principles

**Clear is better than clever.** If a function's control flow takes three reads to
follow, rewrite it. Keep the happy path un-indented: handle errors and edge cases
first and `return`.

```go
func GetUser(id string) (*User, error) {
    user, err := db.FindUser(id)
    if err != nil {
        return nil, fmt.Errorf("finding user %s: %w", id, err)
    }
    return user, nil // happy path stays at the left margin
}
```

**Make the zero value useful.** Design types so the zero value works without a
constructor - `sync.Mutex` and `bytes.Buffer` are the gold standard.

```go
type Counter struct {
    mu    sync.Mutex // usable at the zero value - no New() needed
    count int
}

func (c *Counter) Inc() { c.mu.Lock(); c.count++; c.mu.Unlock() }
```

## Packages: flat, named for what they do

Start flat; add a package only when a domain earns a namespace. Name packages by
**what they do** (`auth`, `billing`, `jobs`), never by what *layer* they are
(`service`, `repository`, `controller`) - layer-named packages breed circular
imports and interface bloat and add zero clarity in Go. No `utils/`, `helpers/`,
or `common/` junk drawers; they signal unclear ownership. Packages don't import
each other sideways - cycles mean wrong boundaries; the composition root wires them.

> **In this repo:** code lives under `internal/` in a deliberate **domain** (pure
> interfaces) / **infra** (adapters) / **services** (business logic) split, wired
> by a DI container (`internal/container/container.go`). That *is* this project's
> structure - follow it; don't "flatten `internal/`." spf13's transferable rules
> still hold: name by purpose, no junk-drawer packages, wire at the composition
> root (here, the container).

## Interfaces: discovered, not designed

Write concrete types first. Introduce an interface only when a consumer genuinely
needs to swap implementations, and define it **in the consuming package**, kept
small (`io.Reader`, not `*os.File`). **Accept interfaces, return structs** -
callers get the narrow dependency they need without type-asserting to reach real
fields.

```go
// the consumer declares exactly what it needs; the concrete store needn't know
type UserFetcher interface {
    GetUser(id string) (*User, error)
}

type Processor struct{ fetcher UserFetcher }
```

> **In this repo:** domain contracts are centralized in
> `internal/domain/interfaces.go` so counterfeiter can generate fakes from one
> place - that's the established seam. Still prefer small interfaces; just declare
> new ones there when they need a generated mock.

## Errors are values

Check them explicitly; they aren't exceptions. Wrap with `%w` to add context while
preserving the chain for `errors.Is` / `errors.As`. Combine siblings with
`errors.Join` (Go 1.20) - no `multierr` needed.

```go
data, err := os.ReadFile(path)
if err != nil {
    return fmt.Errorf("loading config %s: %w", path, err)
}
```

## Functional options for complex construction

When a type has many optional settings, avoid telescoping constructors:

```go
type Option func(*Server)

func WithTimeout(d time.Duration) Option { return func(s *Server) { s.timeout = d } }

func NewServer(addr string, opts ...Option) *Server {
    s := &Server{addr: addr, timeout: 30 * time.Second} // sane defaults
    for _, opt := range opts {
        opt(s)
    }
    return s
}
```

## Testing is just Go

Go testing is just Go programming - reach for the stdlib `testing` package, not a
BDD framework.

- **Table-driven tests** with `t.Run` subtests are the standard.
- Call **`t.Helper()`** in assertion helpers so failures point at the case, not
  the helper.
- Prefer simple **fakes/stubs** for stand-ins - implicit interfaces make them
  cheap to hand-write.
- Put fixtures in **`testdata/`** (the tool ignores it); diff large output against
  **golden files**.
- Compare structs with **`github.com/google/go-cmp/cmp`**, not `reflect.DeepEqual`.
- Abstract the filesystem with **`afero`** and inject `afero.NewMemMapFs()` in tests.

```go
func TestParse(t *testing.T) {
    tests := []struct {
        name    string
        in      string
        wantErr bool
    }{
        {"valid", "port=8080", false},
        {"bad", "port=abc", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := Parse(tt.in)
            if (err != nil) != tt.wantErr {
                t.Fatalf("Parse(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
            }
        })
    }
}
```

> **In this repo:** fakes for domain interfaces are **counterfeiter-generated**
> under `tests/mocks/`, not hand-written. Regenerate with `task mocks:generate`
> (pre-commit runs it when `internal/domain/interfaces.go` changes); for a *new*
> domain interface, add a `counterfeiter` line under `mocks:generate` in
> `Taskfile.yml`. Use the generated fakes - don't hand-roll rivals. Table tests,
> `testdata`, and `go-cmp` still apply.

## Use current Go, not 2018 Go

This repo targets **Go 1.27** - reach for the modern stdlib before any third-party
helper or hand-rolled loop:

| Instead of | Use |
| --- | --- |
| `sort.Slice(s, ...)` | `slices.Sort(s)`, `slices.SortFunc`, `slices.Contains/Index` |
| manual key/value loops | `maps.Keys/Values/Clone/Copy/Equal` |
| `if x == 0 { x = def }` | `cmp.Or(x, def)`; `min`/`max` built-ins |
| ad-hoc multi-error joins | `errors.Join(err1, err2)` |
| `atomic.AddInt64(&n, 1)` | typed `atomic.Int64`/`Bool`/`Pointer[T]` (`n.Add(1)`) |
| gorilla/mux for basic routing | `net/http`: `mux.HandleFunc("GET /u/{id}", ...)` + `r.PathValue("id")` |
| `for i := 0; i < n; i++` | `for i := range n` |
| `interface{}` | `any` |
| `// +build` tags | `//go:build` |

To keep a request's values but drop its cancellation for work that outlives it,
use `context.WithoutCancel(ctx)` (Go 1.21).

## Generics eliminate duplication, not model hierarchies

Use a generic when the **same algorithm** runs over **many concrete types**.

```go
func Map[S, T any](s []S, f func(S) T) []T {
    out := make([]T, len(s))
    for i, v := range s {
        out[i] = f(v)
    }
    return out
}
```

Do **not** write generic "repositories", services, or base types - that's Java in
Go syntax. Use `comparable` for map keys/equality and `cmp.Ordered` for `<`/`>`.
Start concrete; generify only once the same logic repeats across 3+ types.

## Logging discipline

```go
logger.Debug("cache miss", "key", key)      // high-volume internal state
logger.Info("server started", "addr", addr) // lifecycle events
logger.Warn("retrying", "attempt", n)        // recoverable problems
logger.Error("request failed", "err", err)   // needs attention
```

- **Inject** the logger as a dependency; no package-level logger beyond `main`.
- **Never log *and* return** the same error - log once at the boundary, return it
  up the stack everywhere else.

> **In this repo:** logging is **`go.uber.org/zap`** (`*zap.Logger`, from the
> container's `Logger()`), not `slog`. The discipline above is identical.

## Concurrency, briefly

- **Share memory by communicating** - pass data over a channel instead of guarding
  it with a mutex where you can. Channels orchestrate; mutexes serialize.
- **Bound concurrency** with a buffered-channel semaphore, not a static worker pool
  (Go's scheduler is cheap):

```go
func FetchAll(ctx context.Context, urls []string, max int) error {
    sem := make(chan struct{}, max)
    g, ctx := errgroup.WithContext(ctx)
    for _, u := range urls {
        sem <- struct{}{} // blocks at the limit
        g.Go(func() error {
            defer func() { <-sem }()
            return fetch(ctx, u)
        })
    }
    return g.Wait()
}
```

- **Never start a goroutine without knowing how it stops** - every `go func()`
  needs a `ctx`/closed-channel exit, or it leaks.

For channel patterns in depth - done-channel, fan-out/fan-in, pipeline, or-done,
context propagation, worker semaphore - see the **go-concurrency** skill, and
verify with `go test -race`.

## Debugging: the Go toolchain is not the bug

`go build`/`test`/`run` are deterministic and the build cache is keyed by source -
**don't** suspect them or reach for `go clean -cache`. When an error survives your
edit, it's almost always one of these, in order:

1. The edit didn't fix the actual logic.
2. It was in the wrong file, function, or package.
3. A second call site has the same bug, unfixed.
4. The error comes from a different code path than you edited.

Re-read the (accurate) error, confirm the file is actually compiled
(`go list -f '{{.GoFiles}}' .`), and add a `t.Log`/print at the exact site.

---

*Adapted from [spf13/go-skills](https://github.com/spf13/go-skills) (MIT, by Steve
Francia), trimmed and reconciled with this repo's conventions. Pairs with
**go-concurrency**, **cobra-viper**, and **go-spec-reviewer**.*
