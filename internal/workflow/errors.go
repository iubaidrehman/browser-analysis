package workflow

import "errors"

// Sentinel errors used to classify step outcomes (spec section 29).
var (
	// contextDeadline is wrapped by step executions that exceed their timeout.
	contextDeadline = errors.New("deadline exceeded")
	// contextCancelled is wrapped by steps aborted due to cancellation.
	contextCancelled = errors.New("cancelled")
)

// DeadlineError marks a step that timed out.
func DeadlineError() error { return contextDeadline }

// CancelledError marks a step aborted by cancellation.
func CancelledError() error { return contextCancelled }
