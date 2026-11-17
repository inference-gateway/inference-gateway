---
name: go-concurrency
description: >
  Idiomatic Go concurrency - sync primitives, channel semantics, the select
  statement, and the standard channel patterns (cancellation/done-channel,
  fan-out/fan-in, pipeline, or-done, context, worker-pool). Use when writing or
  reviewing Go that spawns goroutines, shares state, or coordinates over
  channels, even if the user never says "concurrency" - e.g. data races,
  deadlocks, goroutine leaks, sync/context/select usage, or "make this parallel".
  Distilled from luk4z7/go-concurrency-guide.
license: Apache-2.0
---

# Go Concurrency

A quick-reference for writing and reviewing concurrent Go. Reach for the table
that fits the problem, copy the nearest pattern, then check it against the
gotchas. Verify everything with `go test -race`.

> Don't communicate by sharing memory - share memory by communicating.
> Use **channels** to pass ownership of data, distribute work, and signal.
> Use a **mutex** for in-place shared state (caches, counters, registries).

## Pick your tool

| Need | Use |
|------|-----|
| Hand off / stream values between goroutines | `chan` |
| Protect shared state in place | `sync.Mutex` / `sync.RWMutex` (read-heavy) |
| Wait for N goroutines to finish | `sync.WaitGroup` |
| Run exactly once (lazy init) | `sync.Once` |
| Reuse expensive allocations, cut GC pressure | `sync.Pool` |
| Wake goroutines on a condition | `sync.Cond` (rare - a channel is usually clearer) |
| Lock-free counters / flags | `sync/atomic` |
| Cancellation, deadlines, request-scoped values | `context.Context` |

Always `defer mu.Unlock()` right after `mu.Lock()` so a panic can't leave the
lock held.

## Channel states

Behaviour is defined by the operation × the channel's state. Memorize this:

| Operation | nil channel | open channel | closed channel |
|-----------|-------------|--------------|----------------|
| receive `v := <-c` | blocks forever | a value (or blocks if empty) | zero value, `ok==false` (drains buffer first) |
| send `c <- v` | blocks forever | sends (or blocks if full) | **panic** |
| `close(c)` | **panic** | closes it | **panic** |

- **Unbuffered** (`make(chan T)`): send and receive rendezvous - each blocks
  until the other is ready. This *is* the synchronization.
- **Buffered** (`make(chan T, n)`): send blocks only when full, receive only
  when empty. A buffer decouples timing - it does not remove backpressure.
- **Ownership rule:** exactly one goroutine owns a channel - it creates, writes,
  and `close`s it, then hands consumers a receive-only `<-chan T`. This makes the
  panics above structurally impossible. Receivers never close.
- `range c` reads until the channel is closed - `v, ok := <-c` detects closure.

## `select` idioms

`select` blocks until one case is ready - if several are ready it picks one
**pseudo-randomly** (so no case can starve another). `nil` channels are never
ready - set a channel to `nil` to disable its case.

```go
// timeout - but time.After leaks a timer until it fires; in a hot loop use a
// context or a reused time.Timer instead.
select {
case v := <-c:
    use(v)
case <-time.After(time.Second):
    return errTimeout
}

// non-blocking poll
select {
case v := <-c:
    use(v)
default: // nothing ready, carry on
}

// cancellation - prefer sending under a done/ctx guard so you never block forever
select {
case out <- v:
case <-done:
    return
}
```

## Gotchas

- **Data race:** concurrent read+write of the same variable with no
  synchronization. Undefined behaviour. Catch it with `go test -race` / `go run -race`.
- **Goroutine leak:** a goroutine blocked on a channel that will never proceed.
  Every goroutine you start needs a guaranteed exit - a `done`/`ctx` signal or a
  channel that *will* close. Leaks accumulate silently.
- **Deadlock:** all goroutines blocked. Causes: acquiring locks in inconsistent
  order (always lock in the same order), or send/receive on a `nil` channel, or
  an unbuffered send with no receiver. `fatal error: all goroutines are asleep`.
- **Send on closed channel / double close:** panics. Follows from the ownership
  rule - one owner closes, once.
- **Loop-variable capture:** pre-Go 1.22, `for _, v := range xs { go f(v) }`
  shared one `v` - pass it as an argument. Go 1.22+ gives each iteration its own
  copy. Know which Go you target.
- **Livelock / starvation:** goroutines run but make no progress (livelock), or a
  greedy lock-holder keeps others from ever running (starvation). Usually a sign
  to add backoff or rebalance critical-section size.

## Signature patterns

Short, copy-ready skeletons. They use `any` (= `interface{}`) and assume Go 1.22+
loop semantics.

**Done-channel cancellation** - pass a read-only `done` - close it once to stop
all listeners. Send results *under* the `done` guard so a cancelled consumer
can't wedge the producer.

```go
func worker(done <-chan struct{}, in <-chan string) <-chan string {
    out := make(chan string)
    go func() {
        defer close(out)
        for s := range in {
            select {
            case out <- transform(s):
            case <-done: // caller cancelled → exit, no leak
                return
            }
        }
    }()
    return out
}

done := make(chan struct{})
defer close(done) // every return path cancels the worker
```

**Fan-out / fan-in** - many workers read one input channel (distribute load),
one channel merges their results. No value is processed twice.

```go
func fanIn(done <-chan struct{}, cs ...<-chan int) <-chan int {
    out := make(chan int)
    var wg sync.WaitGroup
    wg.Add(len(cs))
    for _, c := range cs {
        go func(c <-chan int) {
            defer wg.Done()
            for v := range c {
                select {
                case out <- v:
                case <-done:
                    return
                }
            }
        }(c)
    }
    go func() { wg.Wait(); close(out) }() // close once every worker drains
    return out
}
```

**Pipeline** - compose stages, each a goroutine that reads one channel and
returns the next. Every stage closes the channel it owns, so closure propagates
downstream and `range` terminates cleanly.

```go
func sq(in <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for n := range in {
            out <- n * n
        }
    }()
    return out
}

for n := range sq(sq(gen(2, 3))) { // 16, 81
    fmt.Println(n)
}
```

**Or-done** - range over a channel without leaking when you might quit early.
Wrap the channel - the loop body stays clean.

```go
func orDone(done, c <-chan any) <-chan any {
    out := make(chan any)
    go func() {
        defer close(out)
        for {
            select {
            case <-done:
                return
            case v, ok := <-c:
                if !ok {
                    return
                }
                select {
                case out <- v:
                case <-done:
                }
            }
        }
    }()
    return out
}

for v := range orDone(done, stream) { // safe; exits on done or closure
    use(v)
}
```

**Context** - the standard cancellation/deadline carrier. Take `ctx` as the
first argument, always `defer cancel()`, and honor `ctx.Done()` in your loops.

```go
ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
defer cancel() // releases resources even on early return

req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
resp, err := http.DefaultClient.Do(req) // aborts on cancel/timeout

select {
case out <- v:
case <-ctx.Done():
    return ctx.Err() // context.Canceled or DeadlineExceeded
}
```

**Queuing / semaphore** - a buffered channel bounds how many goroutines run at
once. Acquire before launching, release on exit.

```go
sem := make(chan struct{}, runtime.NumCPU()) // N concurrent slots
var wg sync.WaitGroup
for _, job := range jobs {
    sem <- struct{}{} // blocks while N are in flight
    wg.Add(1)
    go func(job Job) {
        defer wg.Done()
        defer func() { <-sem }() // free the slot
        process(job)
    }(job)
}
wg.Wait()
```

Lower-frequency patterns from the source, when you need them: **or-channel**
(combine many done signals into one), **tee** (duplicate one stream into two),
**bridge** (flatten a channel-of-channels), **heartbeat** (periodic liveness
signal for monitoring), **replicated requests** (fan a request to N handlers,
take the first reply, cancel the rest).

## Scheduler, briefly

Go multiplexes goroutines onto OS threads with an **M:N work-stealing**
scheduler: **G** = goroutine, **M** = OS thread, **P** = processor (a run
context). `GOMAXPROCS` (default `runtime.NumCPU()`) caps how many Ps run Go code
at once - an idle P steals goroutines from a busy P's queue. Goroutines are cheap
- ~2 KB initial stacks that grow on demand and nanosecond-scale switches - so
thousands are fine where threads would not be. You rarely manage any of this
directly - you make goroutines *cancellable* and let the runtime schedule them.
None of it makes a racy program correct - run `-race`.

---

*Distilled from [luk4z7/go-concurrency-guide](https://github.com/luk4z7/go-concurrency-guide),
which draws on "Concurrency in Go" (Katherine Cox-Buday) and "The Go Programming Language".*
