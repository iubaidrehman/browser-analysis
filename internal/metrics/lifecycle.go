package metrics

import "time"

// LifecycleEventType identifies a lifecycle stage (milestone section 10).
type LifecycleEventType string

const (
	EvTaskCreated            LifecycleEventType = "task_created"
	EvWorkerStarted          LifecycleEventType = "worker_started"
	EvBrowserLaunchStarted   LifecycleEventType = "browser_launch_started"
	EvBrowserLaunchCompleted LifecycleEventType = "browser_launch_completed"
	EvBrowserConnected       LifecycleEventType = "browser_connected"
	EvContextCreateStarted   LifecycleEventType = "context_create_started"
	EvContextCreateCompleted LifecycleEventType = "context_create_completed"
	EvPageCreateStarted      LifecycleEventType = "page_create_started"
	EvPageCreateCompleted    LifecycleEventType = "page_create_completed"
	EvNavigationStarted      LifecycleEventType = "navigation_started"
	EvNavigationCompleted    LifecycleEventType = "navigation_completed"
	EvWorkflowStarted        LifecycleEventType = "workflow_started"
	EvWorkflowCompleted      LifecycleEventType = "workflow_completed"
	EvTaskFinished           LifecycleEventType = "task_finished"
)

// LifecycleEvent is one timestamped lifecycle record.
type LifecycleEvent struct {
	At        time.Time          `json:"at"`
	Type      LifecycleEventType `json:"type"`
	WorkerID  int                `json:"worker_id,omitempty"`
	TaskID    int                `json:"task_id,omitempty"`
	BrowserID int                `json:"browser_id,omitempty"`
	ContextID int                `json:"context_id,omitempty"`
	PageID    int                `json:"page_id,omitempty"`
}
