package llamacpp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	assets "github.com/inference-gateway/inference-gateway/internal/platform/assets"
	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
)

const (
	// BinaryName is llama.cpp's OpenAI-compatible HTTP server.
	BinaryName = "llama-server"

	// RetryAfterSeconds is advised on the 503 returned while a model is not
	// ready yet.
	RetryAfterSeconds = 10

	// DefaultMaxModels bounds concurrently loaded models when the config
	// names none.
	DefaultMaxModels = 1

	loopbackHost       = "127.0.0.1"
	healthPath         = "/health"
	healthPollInterval = 500 * time.Millisecond
	healthProbeTimeout = 2 * time.Second
	// startTimeout bounds loading a model into llama-server, big ones included.
	startTimeout    = 10 * time.Minute
	stopGracePeriod = 5 * time.Second
	// idleCheckInterval caps how often idle servers are looked for; shorter
	// idle timeouts shorten it.
	idleCheckInterval = 15 * time.Second
	outputTailBytes   = 2048

	flagModel          = "-m"
	flagMmproj         = "--mmproj"
	flagHost           = "--host"
	flagPort           = "--port"
	flagContextSize    = "-c"
	flagGPULayers      = "-ngl"
	flagThreads        = "-t"
	flagBatchSize      = "-b"
	flagParallel       = "-np"
	flagFlashAttention = "-fa"
	flagJinja          = "--jinja"
)

// ErrShutDown is returned once Shutdown has been called.
var ErrShutDown = errors.New("local model runtime is shut down")

// LoadOptions is the subset of llama-server flags passed when a model is
// loaded. Zero values pass nothing, leaving llama-server's own defaults.
// Sampling parameters are not here: they ride in the OpenAI request body.
type LoadOptions struct {
	ContextSize    int
	GPULayers      int
	Threads        int
	BatchSize      int
	Parallel       int
	FlashAttention string
	Jinja          bool
	ExtraArgs      string
}

func (o LoadOptions) args() []string {
	var args []string
	for _, n := range []struct {
		flag  string
		value int
	}{
		{flagContextSize, o.ContextSize},
		{flagGPULayers, o.GPULayers},
		{flagThreads, o.Threads},
		{flagBatchSize, o.BatchSize},
		{flagParallel, o.Parallel},
	} {
		if n.value > 0 {
			args = append(args, n.flag, strconv.Itoa(n.value))
		}
	}
	if o.FlashAttention != "" {
		args = append(args, flagFlashAttention, o.FlashAttention)
	}
	if o.Jinja {
		args = append(args, flagJinja)
	}
	return append(args, strings.Fields(o.ExtraArgs)...)
}

// Config wires the LLAMACPP_LOCAL_* settings plus the cache root.
type Config struct {
	// AutoDownload allows fetching llama-server and GGUFs from the network.
	// When false the runtime only serves what the cache or PATH already has.
	AutoDownload bool
	// MaxModels caps concurrently loaded models; the least recently used
	// idle one is stopped to make room.
	MaxModels int
	// IdleTimeout stops a server unused for this long; zero keeps them.
	IdleTimeout time.Duration
	// Home is the root of the shared ~/.infer cache.
	Home string
	// HubToken is sent as a bearer to Hugging Face so gated repos resolve.
	HubToken string
	Load     LoadOptions
}

// NotReadyError means the model cannot be served right now (downloading,
// the last attempt failed, every slot is busy, or auto-download is off and
// assets are missing). The API layer maps it to 503 + Retry-After.
type NotReadyError struct{ Detail string }

func (e *NotReadyError) Error() string { return e.Detail }

type state int

const (
	stateDownloading state = iota
	stateStarting
	stateReady
	stateFailed
)

// server is one llama-server child. ready closes once it leaves
// stateStarting; the remaining fields are guarded by Runtime.mu.
type server struct {
	ref    Reference
	ready  chan struct{}
	output *tail

	state    state
	detail   string
	url      string
	cancel   context.CancelFunc
	exited   chan struct{}
	lastUsed time.Time
	inflight int
}

// Runtime owns the llama-server children, at most MaxModels at once, with
// their download state, health, idle eviction and shutdown. Construct with
// NewRuntime.
type Runtime struct {
	cfg       Config
	logger    logger.Logger
	cache     *Cache
	http      *http.Client
	hubHeader http.Header
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	mu      sync.Mutex
	servers map[Reference]*server
}

// NewRuntime builds the runtime; it spawns nothing until a model is asked for.
func NewRuntime(log logger.Logger, cfg Config) *Runtime {
	cfg.MaxModels = max(cfg.MaxModels, DefaultMaxModels)
	ctx, cancel := context.WithCancel(context.Background())
	r := &Runtime{
		cfg:     cfg,
		logger:  log,
		cache:   newCache(cfg.Home, log),
		http:    &http.Client{},
		ctx:     ctx,
		cancel:  cancel,
		servers: make(map[Reference]*server),
	}
	if cfg.HubToken != "" {
		r.hubHeader = http.Header{"Authorization": {"Bearer " + cfg.HubToken}}
	}
	if cfg.IdleTimeout > 0 {
		r.wg.Add(1)
		go r.evictIdle()
	}
	return r
}

// Acquire returns the base URL of a ready llama-server for ref and a release
// func the caller invokes once its request is over, which keeps the server
// out of idle eviction meanwhile. While assets download it returns
// *NotReadyError at once, never holding the caller; a server that is merely
// starting is awaited, bounded by ctx. A failed attempt is retried on the
// next call.
func (r *Runtime) Acquire(ctx context.Context, ref Reference) (string, func(), error) {
	r.mu.Lock()
	s, err := r.serverLocked(ref)
	if err != nil {
		r.mu.Unlock()
		return "", nil, err
	}
	downloading, detail := s.state == stateDownloading, s.detail
	r.mu.Unlock()
	if downloading {
		return "", nil, &NotReadyError{Detail: detail}
	}

	select {
	case <-s.ready:
	case <-ctx.Done():
		return "", nil, ctx.Err()
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if s.state != stateReady {
		return "", nil, &NotReadyError{Detail: s.detail}
	}
	s.inflight++
	s.lastUsed = time.Now()
	var once sync.Once
	release := func() {
		once.Do(func() {
			r.mu.Lock()
			s.inflight--
			s.lastUsed = time.Now()
			r.mu.Unlock()
		})
	}
	return s.url, release, nil
}

// Warmup launches every reference in the background, downloading and
// starting them as a request would.
func (r *Runtime) Warmup(refs []Reference) {
	for _, ref := range refs {
		r.mu.Lock()
		_, err := r.serverLocked(ref)
		r.mu.Unlock()
		if err != nil {
			r.logger.Warn("llamacpp: cannot warm up local model", "model", ref.String(), "error", err.Error())
		}
	}
}

// Cached lists every reference whose files are all in the cache.
func (r *Runtime) Cached() []Reference {
	return r.cache.Cached()
}

// Shutdown stops every child and aborts in-flight downloads, waiting for
// the processes to exit until ctx is done.
func (r *Runtime) Shutdown(ctx context.Context) error {
	r.cancel()
	r.mu.Lock()
	var exits []chan struct{}
	for ref, s := range r.servers {
		if s.exited != nil {
			exits = append(exits, s.exited)
		}
		delete(r.servers, ref)
	}
	r.mu.Unlock()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, exited := range exits {
			<-exited
		}
		r.wg.Wait()
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("waiting for %s children: %w", BinaryName, ctx.Err())
	}
}

// serverLocked returns the server for ref, launching one when none is live.
// A failed one is replaced so the next request self-heals, carrying the old
// failure in the new status until it is superseded.
func (r *Runtime) serverLocked(ref Reference) (*server, error) {
	if s, ok := r.servers[ref]; ok && s.state != stateFailed {
		return s, nil
	}
	if r.ctx.Err() != nil {
		return nil, ErrShutDown
	}
	previous := ""
	if s, ok := r.servers[ref]; ok {
		previous = s.detail
		delete(r.servers, ref)
	}

	_, cached := r.cache.Complete(ref)
	_, haveBinary := assets.ResolveBinary(r.cfg.Home, BinaryName)
	if !r.cfg.AutoDownload && !(cached && haveBinary) {
		return nil, &NotReadyError{Detail: fmt.Sprintf(
			"%s is not available: it must be in the local cache (%s) and %s on PATH or in %s; pre-download them, or set LLAMACPP_LOCAL_AUTO_DOWNLOAD=true",
			ref, r.cache.RepoDir(ref), BinaryName, assets.BinPath(r.cfg.Home, BinaryName))}
	}
	if err := r.makeRoomLocked(); err != nil {
		return nil, err
	}

	s := &server{
		ref:      ref,
		ready:    make(chan struct{}),
		output:   &tail{},
		state:    stateDownloading,
		detail:   "preparing " + ref.String(),
		lastUsed: time.Now(),
	}
	if cached && haveBinary {
		s.state = stateStarting
		s.detail = "starting " + BinaryName + " for " + ref.String()
	}
	if previous != "" {
		s.detail += " (last attempt failed: " + previous + ")"
	}
	r.servers[ref] = s
	r.wg.Add(1)
	go r.load(s)
	return s, nil
}

// makeRoomLocked stops the least recently used idle server until a slot is
// free, failing when every slot is busy.
func (r *Runtime) makeRoomLocked() error {
	for len(r.servers) >= r.cfg.MaxModels {
		var victim *server
		for _, s := range r.servers {
			if s.state == stateReady && s.inflight == 0 && (victim == nil || s.lastUsed.Before(victim.lastUsed)) {
				victim = s
			}
		}
		if victim == nil {
			return &NotReadyError{Detail: fmt.Sprintf("all %d local model slots are busy; retry shortly or raise LLAMACPP_LOCAL_MAX_MODELS", r.cfg.MaxModels)}
		}
		r.logger.Info("llamacpp: stopping local model to make room", "model", victim.ref.String())
		r.stopLocked(victim)
	}
	return nil
}

// stopLocked forgets the server and asks its process to exit; the exit
// watcher started in spawn reaps it.
func (r *Runtime) stopLocked(s *server) {
	delete(r.servers, s.ref)
	if s.cancel != nil {
		s.cancel()
	}
}

// load runs in its own goroutine: it fetches the assets, spawns llama-server
// and waits for it to become healthy, publishing progress on the way.
func (r *Runtime) load(s *server) {
	defer r.wg.Done()
	status := func(msg string) { r.setDetail(s, msg) }
	hub := &assets.Downloader{HTTP: r.http, Logger: r.logger, Header: r.hubHeader, Status: status}
	release := &assets.Downloader{HTTP: r.http, Logger: r.logger, Status: status}

	bin, manifest, err := r.ensure(r.ctx, hub, release, s.ref)
	if err != nil {
		r.fail(s, err)
		return
	}
	r.transition(s, stateStarting, "starting "+BinaryName+" for "+s.ref.String())
	if err := r.spawn(r.ctx, s, bin, manifest); err != nil {
		r.fail(s, err)
		return
	}
	if err := r.awaitHealthy(r.ctx, s); err != nil {
		s.cancel()
		r.fail(s, err)
		return
	}
	r.transition(s, stateReady, "")
	close(s.ready)
	r.logger.Info("llamacpp: local model ready", "model", s.ref.String(), "url", s.url)
}

// ensure makes the binary and the model files available, downloading when
// allowed, and returns the binary path and the resolved manifest.
func (r *Runtime) ensure(ctx context.Context, hub, release *assets.Downloader, ref Reference) (string, Manifest, error) {
	if r.cfg.AutoDownload {
		if err := release.EnsureBinary(ctx, r.cfg.Home, BinaryName); err != nil {
			return "", Manifest{}, fmt.Errorf("obtaining %s: %w (install it on PATH or into %s)", BinaryName, err, assets.BinPath(r.cfg.Home, BinaryName))
		}
	}
	bin, ok := assets.ResolveBinary(r.cfg.Home, BinaryName)
	if !ok {
		return "", Manifest{}, fmt.Errorf("%s is not on PATH or in %s", BinaryName, assets.BinPath(r.cfg.Home, BinaryName))
	}
	if !r.cfg.AutoDownload {
		m, ok := r.cache.Complete(ref)
		if !ok {
			return "", Manifest{}, fmt.Errorf("%s is not in the local cache (%s)", ref, r.cache.RepoDir(ref))
		}
		return bin, m, nil
	}
	m, err := r.cache.Ensure(ctx, hub, ref)
	if err != nil {
		return "", Manifest{}, err
	}
	return bin, m, nil
}

// spawn starts llama-server on a free loopback port and watches its exit.
func (r *Runtime) spawn(ctx context.Context, s *server, bin string, m Manifest) error {
	port, err := freePort()
	if err != nil {
		return fmt.Errorf("picking a port for %s: %w", BinaryName, err)
	}
	args := []string{
		flagModel, r.cache.Path(s.ref, m.Model),
		flagHost, loopbackHost,
		flagPort, strconv.Itoa(port),
	}
	if m.Mmproj != "" {
		args = append(args, flagMmproj, r.cache.Path(s.ref, m.Mmproj))
	}
	args = append(args, r.cfg.Load.args()...)

	cctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cctx, bin, args...)
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = stopGracePeriod
	cmd.Stdout = s.output
	cmd.Stderr = s.output
	r.logger.Info("llamacpp: starting "+BinaryName, "model", s.ref.String(), "args", strings.Join(args, " "))
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("starting %s: %w", BinaryName, err)
	}

	exited := make(chan struct{})
	r.mu.Lock()
	s.url = "http://" + net.JoinHostPort(loopbackHost, strconv.Itoa(port))
	s.cancel = cancel
	s.exited = exited
	r.mu.Unlock()

	go func() {
		err := cmd.Wait()
		close(exited)
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.servers[s.ref] != s || s.state != stateReady {
			return
		}
		s.state = stateFailed
		s.detail = fmt.Sprintf("%s exited (%v): %s", BinaryName, err, s.output.String())
		r.logger.Error("llamacpp: local model server exited", err, "model", s.ref.String(), "output", s.output.String())
	}()
	return nil
}

// awaitHealthy polls /health until llama-server reports the model loaded.
func (r *Runtime) awaitHealthy(ctx context.Context, s *server) error {
	deadline := time.NewTimer(startTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(healthPollInterval)
	defer ticker.Stop()
	for {
		if r.healthy(ctx, s.url) {
			return nil
		}
		select {
		case <-ticker.C:
		case <-s.exited:
			return fmt.Errorf("%s exited before becoming healthy: %s", BinaryName, s.output.String())
		case <-deadline.C:
			return fmt.Errorf("%s did not become healthy within %s: %s", BinaryName, startTimeout, s.output.String())
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (r *Runtime) healthy(ctx context.Context, baseURL string) bool {
	ctx, cancel := context.WithTimeout(ctx, healthProbeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+healthPath, nil)
	if err != nil {
		return false
	}
	resp, err := r.http.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// evictIdle stops servers nobody has used for IdleTimeout.
func (r *Runtime) evictIdle() {
	defer r.wg.Done()
	ticker := time.NewTicker(min(idleCheckInterval, r.cfg.IdleTimeout))
	defer ticker.Stop()
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
		}
		r.mu.Lock()
		for _, s := range r.servers {
			if s.state == stateReady && s.inflight == 0 && time.Since(s.lastUsed) >= r.cfg.IdleTimeout {
				r.logger.Info("llamacpp: stopping idle local model", "model", s.ref.String(), "idle", r.cfg.IdleTimeout.String())
				r.stopLocked(s)
			}
		}
		r.mu.Unlock()
	}
}

func (r *Runtime) fail(s *server, err error) {
	r.transition(s, stateFailed, err.Error())
	close(s.ready)
	r.logger.Error("llamacpp: local model is not ready; requests get 503 with details", err, "model", s.ref.String())
}

func (r *Runtime) transition(s *server, to state, detail string) {
	r.mu.Lock()
	s.state = to
	s.detail = detail
	r.mu.Unlock()
}

func (r *Runtime) setDetail(s *server, detail string) {
	r.mu.Lock()
	s.detail = detail
	r.mu.Unlock()
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", net.JoinHostPort(loopbackHost, "0"))
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// tail keeps the last outputTailBytes of process output for diagnostics.
type tail struct {
	mu  sync.Mutex
	buf []byte
}

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > outputTailBytes {
		t.buf = append([]byte(nil), t.buf[len(t.buf)-outputTailBytes:]...)
	}
	return len(p), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimSpace(string(t.buf))
}
