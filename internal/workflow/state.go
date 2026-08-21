package workflow

// SessionState carries the session and order identifiers across steps of a
// workflow, so mixed-transport executors (hybrid) can share state between
// HTTP steps and browser steps.
type SessionState struct {
	SessionID string
	OrderID   string
}
