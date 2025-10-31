package deploy

import deploy "github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy"

// StepCompleteFunc is the type of functions returned from Step.Apply. These
// functions are to be called when the engine has fully retired a step. You
// _should not_ modify the resource state in these functions -- doing so will
// race with the snapshot writing code.
type StepCompleteFunc = deploy.StepCompleteFunc

// Step is a specification for a deployment operation.
type Step = deploy.Step

// SameStep is a mutating step that does nothing.
type SameStep = deploy.SameStep

// CreateStep is a mutating step that creates an entirely new resource.
type CreateStep = deploy.CreateStep

// DeleteStep is a mutating step that deletes an existing resource. If `old` is marked "External",
// DeleteStep is a no-op.
type DeleteStep = deploy.DeleteStep

type RemovePendingReplaceStep = deploy.RemovePendingReplaceStep

// UpdateStep is a mutating step that updates an existing resource's state.
type UpdateStep = deploy.UpdateStep

// ReplaceStep is a logical step indicating a resource will be replaced.  This is comprised of three physical steps:
// a creation of the new resource, any number of intervening updates of dependents to the new resource, and then
// a deletion of the now-replaced old resource.  This logical step is primarily here for tools and visualization.
type ReplaceStep = deploy.ReplaceStep

// ReadStep is a step indicating that an existing resources will be "read" and projected into the Pulumi object
// model. Resources that are read are marked with the "External" bit which indicates to the engine that it does
// not own this resource's lifeycle.
// 
// A resource with a given URN can transition freely between an "external" state and a non-external state. If
// a URN that was previously marked "External" (i.e. was the target of a ReadStep in a previous deployment) is the
// target of a RegisterResource in the next deployment, a CreateReplacement step will be issued to indicate the
// transition from external to owned. If a URN that was previously not marked "External" is the target of a
// ReadResource in the next deployment, a ReadReplacement step will be issued to indicate the transition from owned to
// external.
type ReadStep = deploy.ReadStep

// RefreshStep is a step used to track the progress of a refresh operation. A refresh operation updates the an existing
// resource by reading its current state from its provider plugin. These steps are not issued by the step generator;
// instead, they are issued by the deployment executor as the optional first step in deployment execution.
type RefreshStep = deploy.RefreshStep

type ImportStep = deploy.ImportStep

// DiffStep isn't really a step like a normal step. It's just a way to get access to the parallel but bounded step
// workers. We use this step to call `provider.Diff` in parallel with other steps.
type DiffStep = deploy.DiffStep

// ViewStep isn't like a normal step. It's a virtual step for a view resource. The step itself
// doesn't perform any operations against a provider, it's used to communicate the steps that
// were taken for the view resource, primarily for display purposes and analysis.
type ViewStep = deploy.ViewStep

const OpSame = deploy.OpSame

const OpCreate = deploy.OpCreate

const OpUpdate = deploy.OpUpdate

const OpDelete = deploy.OpDelete

const OpReplace = deploy.OpReplace

const OpCreateReplacement = deploy.OpCreateReplacement

const OpDeleteReplaced = deploy.OpDeleteReplaced

const OpRead = deploy.OpRead

const OpReadReplacement = deploy.OpReadReplacement

const OpRefresh = deploy.OpRefresh

const OpReadDiscard = deploy.OpReadDiscard

const OpDiscardReplaced = deploy.OpDiscardReplaced

const OpRemovePendingReplace = deploy.OpRemovePendingReplace

const OpImport = deploy.OpImport

const OpImportReplacement = deploy.OpImportReplacement

const OpDiff = deploy.OpDiff

// StepOps contains the full set of step operation types.
var StepOps = deploy.StepOps

func NewSameStep(deployment *Deployment, reg RegisterResourceEvent, old, new *resource.State) Step {
	return deploy.NewSameStep(deployment, reg, old, new)
}

// NewSkippedCreateStep produces a SameStep for a resource that was created but not targeted
// by the user (and thus was skipped). These act as no-op steps (hence 'same') since we are not
// actually creating the resource, but ensure that we complete resource-registration and convey the
// right information downstream. For example, we will not write these into the checkpoint file.
func NewSkippedCreateStep(deployment *Deployment, reg RegisterResourceEvent, new *resource.State) Step {
	return deploy.NewSkippedCreateStep(deployment, reg, new)
}

func NewCreateStep(deployment *Deployment, reg RegisterResourceEvent, new *resource.State) Step {
	return deploy.NewCreateStep(deployment, reg, new)
}

func NewCreateReplacementStep(deployment *Deployment, reg RegisterResourceEvent, old, new *resource.State, keys, diffs []resource.PropertyKey, detailedDiff map[string]plugin.PropertyDiff, pendingDelete bool) Step {
	return deploy.NewCreateReplacementStep(deployment, reg, old, new, keys, diffs, detailedDiff, pendingDelete)
}

func NewDeleteStep(deployment *Deployment, otherDeletions map[resource.URN]bool, old *resource.State, oldViews []plugin.View) Step {
	return deploy.NewDeleteStep(deployment, otherDeletions, old, oldViews)
}

func NewDeleteReplacementStep(deployment *Deployment, otherDeletions map[resource.URN]bool, old *resource.State, pendingReplace bool, oldViews []plugin.View) Step {
	return deploy.NewDeleteReplacementStep(deployment, otherDeletions, old, pendingReplace, oldViews)
}

func NewRemovePendingReplaceStep(deployment *Deployment, old *resource.State) Step {
	return deploy.NewRemovePendingReplaceStep(deployment, old)
}

func NewUpdateStep(deployment *Deployment, reg RegisterResourceEvent, old, new *resource.State, stables, diffs []resource.PropertyKey, detailedDiff map[string]plugin.PropertyDiff, ignoreChanges []string, oldViews []plugin.View) Step {
	return deploy.NewUpdateStep(deployment, reg, old, new, stables, diffs, detailedDiff, ignoreChanges, oldViews)
}

func NewReplaceStep(deployment *Deployment, old, new *resource.State, keys, diffs []resource.PropertyKey, detailedDiff map[string]plugin.PropertyDiff, pendingDelete bool) Step {
	return deploy.NewReplaceStep(deployment, old, new, keys, diffs, detailedDiff, pendingDelete)
}

// NewReadStep creates a new Read step.
func NewReadStep(deployment *Deployment, event ReadResourceEvent, old, new *resource.State) Step {
	return deploy.NewReadStep(deployment, event, old, new)
}

// NewReadReplacementStep creates a new Read step with the `replacing` flag set. When executed,
// it will pend deletion of the "old" resource, which must not be an external resource.
func NewReadReplacementStep(deployment *Deployment, event ReadResourceEvent, old, new *resource.State) Step {
	return deploy.NewReadReplacementStep(deployment, event, old, new)
}

// NewRefreshStep creates a new Refresh step.
func NewRefreshStep(deployment *Deployment, cts *interface{}, old *resource.State, oldViews []plugin.View, new *resource.State) Step {
	return deploy.NewRefreshStep(deployment, cts, old, oldViews, new)
}

func NewImportStep(deployment *Deployment, reg RegisterResourceEvent, new *resource.State, ignoreChanges []string, randomSeed []byte, cts *interface{}) Step {
	return deploy.NewImportStep(deployment, reg, new, ignoreChanges, randomSeed, cts)
}

func NewImportReplacementStep(deployment *Deployment, reg RegisterResourceEvent, original, new *resource.State, ignoreChanges []string, randomSeed []byte, cts *interface{}) Step {
	return deploy.NewImportReplacementStep(deployment, reg, original, new, ignoreChanges, randomSeed, cts)
}

func IsReplacementStep(op display.StepOp) bool {
	return deploy.IsReplacementStep(op)
}

// Color returns a suggested color for lines of this op type.
func Color(op display.StepOp) string {
	return deploy.Color(op)
}

// ColorProgress returns a suggested coloring for lines of this of type which
// are progressing.
func ColorProgress(op display.StepOp) string {
	return deploy.ColorProgress(op)
}

// Prefix returns a suggested prefix for lines of this op type.
func Prefix(op display.StepOp, done bool) string {
	return deploy.Prefix(op, done)
}

// RawPrefix returns the uncolorized prefix text.
func RawPrefix(op display.StepOp) string {
	return deploy.RawPrefix(op)
}

func PastTense(op display.StepOp) string {
	return deploy.PastTense(op)
}

// Suffix returns a suggested suffix for lines of this op type.
func Suffix(op display.StepOp) string {
	return deploy.Suffix(op)
}

// ConstrainedTo returns true if this operation is no more impactful than the constraint.
func ConstrainedTo(op display.StepOp, constraint display.StepOp) bool {
	return deploy.ConstrainedTo(op, constraint)
}

func NewDiffStep(deployment *Deployment, pcs *interface{}, old, new *resource.State, ignoreChanges []string) Step {
	return deploy.NewDiffStep(deployment, pcs, old, new, ignoreChanges)
}

func NewViewStep(deployment *Deployment, op display.StepOp, status resource.Status, err string, old, new *resource.State, keys, diffs []resource.PropertyKey, detailedDiff map[string]plugin.PropertyDiff, resultOp display.StepOp, persisted bool) Step {
	return deploy.NewViewStep(deployment, op, status, err, old, new, keys, diffs, detailedDiff, resultOp, persisted)
}

