package engine

import engine "github.com/pulumi/pulumi/sdk/v3/pkg/engine"

// TranslateDetailedDiff converts the detailed diff stored in the step event into an ObjectDiff that is appropriate
// for display.
// 
// The second returned argument is the list of hidden diffs.
func TranslateDetailedDiff(step *StepEventMetadata, refresh bool) (*resource.ObjectDiff, []resource.PropertyPath) {
	return engine.TranslateDetailedDiff(step, refresh)
}

