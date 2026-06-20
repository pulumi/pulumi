// Copyright 2026, Pulumi Corporation.

package deploy

import "fmt"

// AwaitingError reports that a deployment suspended: one or more resources are not yet
// ready, their providers having returned the non-terminal `awaiting` disposition. It is
// not a failure -- the resources that did complete are persisted, and a later update
// resumes the deployment, retrying the suspended resources. It is deliberately NOT a bail
// error (which would collapse to the generic failure exit code); the CLI matches it with
// errors.As and maps it to its own distinct, honest "amber" exit code.
type AwaitingError struct {
	// Steps are the suspended steps (their URNs and reasons drive display).
	Steps []Step
}

func (e *AwaitingError) Error() string {
	if len(e.Steps) == 1 {
		return fmt.Sprintf("1 resource is awaiting: %s", e.Steps[0].URN())
	}
	return fmt.Sprintf("%d resources are awaiting", len(e.Steps))
}
