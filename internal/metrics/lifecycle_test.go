package metrics

import (
	"testing"
	"time"
)

func TestLifecycleCounts(t *testing.T) {
	r := NewRecorder()
	r.RecordLifecycle(LifecycleEvent{Type: EvBrowserLaunchStarted, At: time.Now(), BrowserID: 1})
	r.RecordLifecycle(LifecycleEvent{Type: EvBrowserConnected, At: time.Now(), BrowserID: 1})
	r.RecordLifecycle(LifecycleEvent{Type: EvContextCreateStarted, At: time.Now(), ContextID: 1})
	r.RecordLifecycle(LifecycleEvent{Type: EvContextCreateStarted, At: time.Now(), ContextID: 2})
	r.RecordLifecycle(LifecycleEvent{Type: EvPageCreateStarted, At: time.Now(), PageID: 1})

	browsers, contexts, pages := r.LifecycleCounts()
	if browsers != 2 {
		t.Fatalf("browsers = %d, want 2 (launch_started + connected)", browsers)
	}
	if contexts != 2 {
		t.Fatalf("contexts = %d, want 2", contexts)
	}
	if pages != 1 {
		t.Fatalf("pages = %d, want 1", pages)
	}

	evs := r.Lifecycle()
	if len(evs) != 5 {
		t.Fatalf("lifecycle events = %d, want 5", len(evs))
	}
}

func TestLifecycleReset(t *testing.T) {
	r := NewRecorder()
	r.RecordLifecycle(LifecycleEvent{Type: EvBrowserLaunchStarted, At: time.Now()})
	r.RecordLifecycle(LifecycleEvent{Type: EvContextCreateStarted, At: time.Now()})
	r.Reset()
	// Counters are preserved across the warmup reset (setup-phase lifecycle);
	// the event log is cleared.
	b, c, p := r.LifecycleCounts()
	if b != 1 || c != 1 {
		t.Fatalf("after reset: browsers=%d contexts=%d, want 1/1 (preserved)", b, c)
	}
	if p != 0 {
		t.Fatalf("pages=%d, want 0", p)
	}
	if evs := r.Lifecycle(); len(evs) != 0 {
		t.Fatalf("lifecycle events after reset = %d, want 0", len(evs))
	}
}
