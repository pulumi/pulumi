package deploy

import deploy "github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy"

// StepApplyFailed is a sentinel error for errors that arise when step application fails.
// We (the step executor) are not responsible for reporting those errors so this sentinel ensures
// that we don't do so.
type StepApplyFailed = deploy.StepApplyFailed

