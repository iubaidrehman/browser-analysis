// Package contexts implements the persistent-context pool for scenario C:
// one persistent Chromium per worker, with many isolated browser contexts
// that can be acquired and released per logical task.
package contexts

import (
	"fmt"
	"sync"
	"time"

	"bcrl/internal/browser"
	"bcrl/internal/metrics"
	"github.com/mxschmitt/playwright-go"
)

// Pool owns one persistent Chromium and a set of isolated contexts.
type Pool struct {
	browser playwright.Browser
	rec     *metrics.Recorder

	mu        sync.Mutex
	available []playwright.BrowserContext
	inUse     map[playwright.BrowserContext]bool
	open      int
	closed    bool
}

// NewPool launches a persistent Chromium via the shared manager and
// pre-creates n isolated contexts. n is contexts.count / worker count.
func NewPool(manager *browser.Manager, headless bool, n int, rec *metrics.Recorder) (*Pool, error) {
	b, launchDur, err := manager.StartBrowser(headless)
	if err != nil {
		return nil, err
	}
	rec.RecordBrowserLaunch(launchDur)

	p := &Pool{browser: b, rec: rec, inUse: make(map[playwright.BrowserContext]bool)}
	for i := 0; i < n; i++ {
		ctx, dur, err := p.newContext()
		if err != nil {
			p.Close()
			return nil, fmt.Errorf("create context %d: %w", i, err)
		}
		rec.RecordContextCreation(dur)
		p.available = append(p.available, ctx)
	}
	return p, nil
}

func (p *Pool) newContext() (playwright.BrowserContext, time.Duration, error) {
	start := time.Now()
	ctx, err := p.browser.NewContext()
	if err != nil {
		return nil, 0, err
	}
	p.open++
	return ctx, time.Since(start), nil
}

// Acquire returns a context from the pool, creating a new one if the pool is
// exhausted. Returns an error if the pool is closed.
func (p *Pool) Acquire() (playwright.BrowserContext, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, fmt.Errorf("context pool closed")
	}
	if len(p.available) > 0 {
		c := p.available[len(p.available)-1]
		p.available = p.available[:len(p.available)-1]
		p.inUse[c] = true
		return c, nil
	}
	c, dur, err := p.newContext()
	if err != nil {
		return nil, err
	}
	p.rec.RecordContextCreation(dur)
	p.inUse[c] = true
	return c, nil
}

// Release returns a context to the pool. Safe to call after Close: the
// context is closed rather than re-added.
func (p *Pool) Release(c playwright.BrowserContext) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.inUse, c)
	if p.closed {
		_ = c.Close()
		return
	}
	p.available = append(p.available, c)
}

// Close closes all contexts (available and in-use) and the persistent
// browser. Idempotent.
func (p *Pool) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	all := append([]playwright.BrowserContext(nil), p.available...)
	for c := range p.inUse {
		all = append(all, c)
	}
	p.available = nil
	p.inUse = make(map[playwright.BrowserContext]bool)
	b := p.browser
	p.mu.Unlock()

	for _, c := range all {
		_ = c.Close()
	}
	if b != nil {
		_ = b.Close()
	}
}

// OpenContexts returns the number of contexts created so far.
func (p *Pool) OpenContexts() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.open
}
