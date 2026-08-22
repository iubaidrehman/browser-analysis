// Package metrics records per-run latency and task counters.
package metrics

import (
	"sync"
	"time"
)

// Recorder is a thread-safe sink for benchmark measurements.
type Recorder struct {
	mu sync.Mutex

	workflowLatencies []float64 // seconds
	stepLatencies     []float64 // seconds
	requestLatencies  []float64 // seconds
	httpStepLatencies []float64 // seconds (hybrid: time spent on HTTP steps)
	browserStepLatencies []float64 // seconds (hybrid: time spent on browser steps)
	browserLaunches   []float64 // seconds (scenarios A/B launch latency)
	contextCreations  []float64 // seconds (scenario C context creation)
	cdpConnects       []float64 // seconds (scenario D CDP connect)
	taskRSSDeltas     []float64 // bytes (per-task working-set delta)

	// Lifecycle counters measured from events (milestone section 10).
	browsersCreated int
	contextsCreated int
	pagesCreated    int
	workersActive   int

	// Lifecycle event log (task/browser/context/page timestamps).
	lifecycle []LifecycleEvent

	tasksCreated  int
	tasksQueued   int
	tasksActive   int
	tasksComplete int
	tasksFailed   int
	tasksCanceled int
	retries       int

	requestsOK     int
	requestsFailed int
	wsEvents       int
	workflowFailed int
	escalations    int

	failures map[string]int
}

// NewRecorder returns an empty recorder.
func NewRecorder() *Recorder {
	return &Recorder{failures: make(map[string]int)}
}

// RecordFailure counts one failure of the given reason.
func (r *Recorder) RecordFailure(reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failures[reason]++
}

// Failures returns a copy of the failure-reason counters.
func (r *Recorder) Failures() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]int, len(r.failures))
	for k, v := range r.failures {
		out[k] = v
	}
	return out
}

// Reset clears per-measurement recorded data at the warmup/measurement
// boundary. Setup-phase series (browser launches, context creations, CDP
// connects) are preserved because they are recorded once before warmup and
// never contain warmup samples.
func (r *Recorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workflowLatencies = nil
	r.stepLatencies = nil
	r.requestLatencies = nil
	r.httpStepLatencies = nil
	r.browserStepLatencies = nil
	r.taskRSSDeltas = nil
	// Lifecycle counters are preserved across the warmup reset: browsers,
	// contexts, and pages are created during setup before the measurement
	// window and never contain warmup samples (mirrors browser launch /
	// context creation series preservation).
	r.lifecycle = nil
	r.tasksCreated = 0
	r.tasksQueued = 0
	r.tasksActive = 0
	r.tasksComplete = 0
	r.tasksFailed = 0
	r.tasksCanceled = 0
	r.retries = 0
	r.requestsOK = 0
	r.requestsFailed = 0
	r.wsEvents = 0
	r.workflowFailed = 0
	r.escalations = 0
	r.failures = make(map[string]int)
}

// RecordBrowserLaunch adds a browser startup duration (seconds).
func (r *Recorder) RecordBrowserLaunch(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.browserLaunches = append(r.browserLaunches, d.Seconds())
}

// RecordContextCreation adds a browser context creation duration (seconds).
func (r *Recorder) RecordContextCreation(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.contextCreations = append(r.contextCreations, d.Seconds())
}

// RecordCDPConnect adds a CDP connection duration (seconds).
func (r *Recorder) RecordCDPConnect(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cdpConnects = append(r.cdpConnects, d.Seconds())
}

// RecordTaskRSSDelta records the working-set delta (bytes) attributed to one
// task, measured as the process RSS change across the task's execution.
func (r *Recorder) RecordTaskRSSDelta(delta uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.taskRSSDeltas = append(r.taskRSSDeltas, float64(delta))
}

// RecordLifecycle appends a lifecycle event and bumps the matching counter.
func (r *Recorder) RecordLifecycle(ev LifecycleEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lifecycle = append(r.lifecycle, ev)
	switch ev.Type {
	case EvBrowserLaunchStarted, EvBrowserLaunchCompleted, EvBrowserConnected:
		r.browsersCreated++
	case EvContextCreateStarted, EvContextCreateCompleted:
		r.contextsCreated++
	case EvPageCreateStarted, EvPageCreateCompleted:
		r.pagesCreated++
	}
}

// Lifecycle returns a copy of the lifecycle event log.
func (r *Recorder) Lifecycle() []LifecycleEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]LifecycleEvent(nil), r.lifecycle...)
}

// LifecycleCounts returns measured browser/context/page counts.
func (r *Recorder) LifecycleCounts() (browsers, contexts, pages int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.browsersCreated, r.contextsCreated, r.pagesCreated
}

// RecordWorkflow adds a completed workflow duration (seconds).
func (r *Recorder) RecordWorkflow(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workflowLatencies = append(r.workflowLatencies, d.Seconds())
}

// RecordStep adds a single step duration (seconds).
func (r *Recorder) RecordStep(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stepLatencies = append(r.stepLatencies, d.Seconds())
}

// RecordHTTPStep adds a hybrid HTTP-step duration (seconds).
func (r *Recorder) RecordHTTPStep(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.httpStepLatencies = append(r.httpStepLatencies, d.Seconds())
}

// RecordBrowserStep adds a hybrid browser-step duration (seconds).
func (r *Recorder) RecordBrowserStep(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.browserStepLatencies = append(r.browserStepLatencies, d.Seconds())
}

// RecordRequest adds an API request duration (seconds).
func (r *Recorder) RecordRequest(d time.Duration, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requestLatencies = append(r.requestLatencies, d.Seconds())
	if ok {
		r.requestsOK++
	} else {
		r.requestsFailed++
	}
}

// Task lifecycle counters.
func (r *Recorder) TaskCreated()   { r.bump(func() { r.tasksCreated++ }) }
func (r *Recorder) TaskQueued()    { r.bump(func() { r.tasksQueued++ }) }
func (r *Recorder) TaskActive()    { r.bump(func() { r.tasksActive++ }) }
func (r *Recorder) TaskComplete()  { r.bump(func() { r.tasksComplete++; r.tasksActive-- }) }
func (r *Recorder) TaskFailed()    { r.bump(func() { r.tasksFailed++; r.tasksActive-- }) }
func (r *Recorder) TaskCancelled() { r.bump(func() { r.tasksCanceled++ }) }
func (r *Recorder) Retry()         { r.bump(func() { r.retries++ }) }

func (r *Recorder) bump(f func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f()
}

// SetActive overrides the active count (used on task start).
func (r *Recorder) SetActive(n int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasksActive = n
}

// WSEvent records one WebSocket event delivered.
func (r *Recorder) WSEvent() { r.bump(func() { r.wsEvents++ }) }

// WorkflowFailed records a failed workflow.
func (r *Recorder) WorkflowFailed() { r.bump(func() { r.workflowFailed++ }) }

// RecordEscalation records one browser escalation (hybrid scenario F).
func (r *Recorder) RecordEscalation() { r.bump(func() { r.escalations++ }) }

// Latencies returns copies of the latency series.
func (r *Recorder) Latencies() (workflow, step, request, browserLaunch, contextCreation, cdpConnect []float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	workflow = append([]float64(nil), r.workflowLatencies...)
	step = append([]float64(nil), r.stepLatencies...)
	request = append([]float64(nil), r.requestLatencies...)
	browserLaunch = append([]float64(nil), r.browserLaunches...)
	contextCreation = append([]float64(nil), r.contextCreations...)
	cdpConnect = append([]float64(nil), r.cdpConnects...)
	return
}

// TransportLatencies returns the hybrid HTTP/browser step time splits.
func (r *Recorder) TransportLatencies() (httpSteps, browserSteps []float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	httpSteps = append([]float64(nil), r.httpStepLatencies...)
	browserSteps = append([]float64(nil), r.browserStepLatencies...)
	return
}

// RSSDeltas returns the per-task working-set deltas in bytes.
func (r *Recorder) RSSDeltas() []float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]float64(nil), r.taskRSSDeltas...)
}

// Counters returns the current task/request counters.
type Counters struct {
	Created  int `json:"tasks_created"`
	Queued   int `json:"tasks_queued"`
	Active   int `json:"tasks_active"`
	Complete int `json:"tasks_completed"`
	Failed   int `json:"tasks_failed"`
	Canceled int `json:"tasks_cancelled"`
	Retries  int `json:"retries"`

	RequestsOK     int `json:"requests_ok"`
	RequestsFailed int `json:"requests_failed"`
	WSEvents       int `json:"ws_events"`
	WorkflowFailed int `json:"workflow_failures"`
	Escalations    int `json:"escalations"`
}

// Snapshot returns a copy of the counters.
func (r *Recorder) Snapshot() Counters {
	r.mu.Lock()
	defer r.mu.Unlock()
	return Counters{
		Created:        r.tasksCreated,
		Queued:         r.tasksQueued,
		Active:         r.tasksActive,
		Complete:       r.tasksComplete,
		Failed:         r.tasksFailed,
		Canceled:       r.tasksCanceled,
		Retries:        r.retries,
		RequestsOK:     r.requestsOK,
		RequestsFailed: r.requestsFailed,
		WSEvents:       r.wsEvents,
		WorkflowFailed: r.workflowFailed,
		Escalations:    r.escalations,
	}
}
