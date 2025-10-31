package deploy

import deploy "github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy"

// BackendClient is used to retrieve information about stacks from a backend.
type BackendClient = deploy.BackendClient

// Options controls the deployment process.
type Options = deploy.Options

// An immutable set of urns to target with an operation.
// 
// The zero value of UrnTargets is the set of all URNs.
type UrnTargets = deploy.UrnTargets

// StepExecutorEvents is an interface that can be used to hook resource lifecycle events.
type StepExecutorEvents = deploy.StepExecutorEvents

// PolicyEvents is an interface that can be used to hook policy events.
type PolicyEvents = deploy.PolicyEvents

// Events is an interface that can be used to hook interesting engine events.
type Events = deploy.Events

// A Deployment manages the iterative computation and execution of a deployment based on a stream of goal states.
// A running deployment emits events that indicate its progress. These events must be used to record the new state
// of the deployment target.
type Deployment = deploy.Deployment

// Create a new set of targets.
// 
// Each element is considered a glob if it contains any '*' and an URN otherwise. No other
// URN validation is performed.
// 
// If len(urnOrGlobs) == 0, an unconstrained set will be created.
func NewUrnTargets(urnOrGlobs []string) UrnTargets {
	return deploy.NewUrnTargets(urnOrGlobs)
}

// Create a new set of targets from fully resolved URNs.
func NewUrnTargetsFromUrns(urns []resource.URN) UrnTargets {
	return deploy.NewUrnTargetsFromUrns(urns)
}

// NewDeployment creates a new deployment from a resource snapshot plus a package to evaluate.
// 
// From the old and new states, it understands how to orchestrate an evaluation and analyze the resulting resources.
// The deployment may be used to simply inspect a series of operations, or actually perform them; these operations are
// generated based on analysis of the old and new states.  If a resource exists in new, but not old, for example, it
// results in a create; if it exists in both, but is different, it results in an update; and so on and so forth.
// 
// Note that a deployment uses internal concurrency and parallelism in various ways, so it must be closed if for some
// reason it isn't carried out to its final conclusion. This will result in cancellation and reclamation of resources.
func NewDeployment(ctx *plugin.Context, opts *Options, events Events, target *Target, prev *Snapshot, plan *Plan, source Source, localPolicyPackPaths []string, backendClient BackendClient, resourceHooks *ResourceHooks) (*Deployment, error) {
	return deploy.NewDeployment(ctx, opts, events, target, prev, plan, source, localPolicyPackPaths, backendClient, resourceHooks)
}

