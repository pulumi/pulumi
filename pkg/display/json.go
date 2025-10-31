package display

import display "github.com/pulumi/pulumi/sdk/v3/pkg/display"

// StepOp represents the kind of operation performed by a step.  It evaluates to its string label.
type StepOp = display.StepOp

// ResourceChanges contains the aggregate resource changes by operation type.
type ResourceChanges = display.ResourceChanges

// PreviewDigest is a JSON-serializable overview of a preview operation.
type PreviewDigest = display.PreviewDigest

// PropertyDiff contains information about the difference in a single property value.
type PropertyDiff = display.PropertyDiff

// PreviewStep is a detailed overview of a step the engine intends to take.
type PreviewStep = display.PreviewStep

// PreviewDiagnostic is a warning or error emitted during the execution of the preview.
type PreviewDiagnostic = display.PreviewDiagnostic

