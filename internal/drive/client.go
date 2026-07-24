package drive

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	driveapi "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// Options is set once at Init and never mutated afterward — the deliberate
// replacement for the scattered SharedMode package global.
type Options struct {
	Workers int
	Shared  bool
	ByID    bool
}

type Client struct {
	svc   *driveapi.Service
	opts  Options
	cache *pathCache
}

var client *Client

func Init(httpClient *http.Client, opts Options) error {
	svc, err := driveapi.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		return fmt.Errorf("failed to create Drive service: %w", err)
	}
	client = &Client{
		svc:   svc,
		opts:  opts,
		cache: newPathCache(),
	}
	return nil
}

// C panics if Init was never called, which can only happen through a programming
// error (a command bypassing the drive PreRun).
func C() *Client {
	if client == nil {
		panic("drive.C() called before drive.Init()")
	}
	return client
}

func (c *Client) Shared() bool               { return c.opts.Shared }
func (c *Client) ByID() bool                 { return c.opts.ByID }
func (c *Client) Workers() int               { return max(1, c.opts.Workers) }
func (c *Client) Service() *driveapi.Service { return c.svc }

type cacheEntry struct {
	id      string
	driveID string
}

type pathCache struct {
	mu sync.RWMutex
	m  map[string]cacheEntry
}

func newPathCache() *pathCache {
	return &pathCache{m: make(map[string]cacheEntry)}
}

func (p *pathCache) get(key string) (cacheEntry, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	e, ok := p.m[key]
	return e, ok
}

func (p *pathCache) set(key string, e cacheEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.m[key] = e
}

// Mutating operations call this so stale path→ID mappings never survive a
// move/delete/create along that path.
func (p *pathCache) invalidatePrefix(prefix string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for k := range p.m {
		// Only the known corpus prefix is stripped; a ':' inside a path segment
		// must never be mistaken for the delimiter (shared vs non-shared keys).
		key, _ := strings.CutPrefix(k, "shared:")
		if key == prefix || strings.HasPrefix(key, prefix+"/") {
			delete(p.m, k)
		}
	}
}

// Callers pass the plain path (no corpus prefix); both corpora are cleared.
func (c *Client) InvalidatePath(path string) {
	c.cache.invalidatePrefix(cleanPath(path))
}
