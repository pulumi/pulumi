// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deploy

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"

	uuid "github.com/gofrs/uuid"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/resource/graph"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// BackendClient is used to retrieve information about stacks from a backend.
type BackendClient interface {
	// GetStackOutputs returns the outputs (if any) for the named stack or an error if the stack cannot be found.
	GetStackOutputs(ctx context.Context, name string) (resource.PropertyMap, error)

	// GetStackResourceOutputs returns the resource outputs for a stack, or an error if the stack
	// cannot be found. Resources are retrieved from the latest stack snapshot, which may include
	// ongoing updates. They are returned in a `PropertyMap` mapping resource URN to another
	// `Propertymap` with members `type` (containing the Pulumi type ID for the resource) and
	// `outputs` (containing the resource outputs themselves).
	GetStackResourceOutputs(ctx context.Context, stackName string) (resource.PropertyMap, error)
}

// Options controls the deployment process.
type Options struct {
	Events                    Events     // an optional events callback interface.
	Parallel                  int        // the degree of parallelism for resource operations (<=1 for serial).
	Refresh                   bool       // whether or not to refresh before executing the deployment.
	RefreshOnly               bool       // whether or not to exit after refreshing.
	Targets                   UrnTargets // If specified, only operate on specified resources.
	ReplaceTargets            UrnTargets // If specified, mark the specified resources for replacement.
	TargetDependents          bool       // true if we're allowing things to proceed, even with unspecified targets
	TrustDependencies         bool       // whether or not to trust the resource dependency graph.
	UseLegacyDiff             bool       // whether or not to use legacy diffing behavior.
	DisableResourceReferences bool       // true to disable resource reference support.
	DisableOutputValues       bool       // true to disable output value support.
	GeneratePlan              bool       // true to enable plan generation.
}

// DegreeOfParallelism returns the degree of parallelism that should be used during the
// deployment process.
func (o Options) DegreeOfParallelism() int {
	if o.Parallel <= 1 {
		return 1
	}
	return o.Parallel
}

// InfiniteParallelism returns whether or not the requested level of parallelism is unbounded.
func (o Options) InfiniteParallelism() bool {
	return o.Parallel == math.MaxInt32
}

// An immutable set of urns to target with an operation.
//
// The zero value of UrnTargets is the set of all URNs.
type UrnTargets struct {
	// UrnTargets is internally made up of two components: literals, which are fully
	// specified URNs and globs, which are partially specified URNs.

	literals []resource.URN
	globs    map[string]*regexp.Regexp
}

// Create a new set of targets.
//
// Each element is considered a glob if it contains any '*' and an URN otherwise. No other
// URN validation is performed.
//
// If len(urnOrGlobs) == 0, an unconstrained set will be created.
func NewUrnTargets(urnOrGlobs []string) UrnTargets {
	literals, globs := []resource.URN{}, map[string]*regexp.Regexp{}
	for _, urn := range urnOrGlobs {
		if strings.ContainsRune(urn, '*') {
			globs[urn] = nil
		} else {
			literals = append(literals, resource.URN(urn))
		}
	}
	return UrnTargets{literals, globs}
}

// Create a new set of targets from fully resolved URNs.
func NewUrnTargetsFromUrns(urns []resource.URN) UrnTargets {
	return UrnTargets{urns, nil}
}

// Return a copy of the UrnTargets
func (t UrnTargets) Clone() UrnTargets {
	newLiterals := append(make([]resource.URN, 0, len(t.literals)), t.literals...)
	newGlobs := make(map[string]*regexp.Regexp, len(t.globs))
	for k, v := range t.globs {
		newGlobs[k] = v
	}
	return UrnTargets{
		literals: newLiterals,
		globs:    newGlobs,
	}
}

// Return if the target set constrains the set of acceptable URNs.
func (t UrnTargets) IsConstrained() bool {
	return len(t.literals) > 0 || len(t.globs) > 0
}

// Get a regexp that can match on the glob. This function caches regexp generation.
func (t UrnTargets) getMatcher(glob string) *regexp.Regexp {
	if r := t.globs[glob]; r != nil {
		return r
	}
	segmentGlob := strings.Split(glob, "**")
	for i, v := range segmentGlob {
		part := strings.Split(v, "*")
		for i, v := range part {
			part[i] = regexp.QuoteMeta(v)
		}
		segmentGlob[i] = strings.Join(part, "[^:]*")
	}

	// Because we have quoted all input, this is safe to compile.
	r := regexp.MustCompile("^" + strings.Join(segmentGlob, ".*") + "$")

	// We cache and return the matcher
	t.globs[glob] = r
	return r
}

// Check if Targets contains the URN.
//
// If method receiver is not initialized, `true` is always returned.
func (t UrnTargets) Contains(urn resource.URN) bool {
	if !t.IsConstrained() {
		return true
	}
	for _, literal := range t.literals {
		if literal == urn {
			return true
		}
	}
	for glob := range t.globs {
		if t.getMatcher(glob).MatchString(string(urn)) {
			return true
		}
	}
	return false
}

// URN literals specified as targets.
//
// It doesn't make sense to iterate over all targets, since the list of targets may be
// infinite.
func (t UrnTargets) Literals() []resource.URN {
	return t.literals
}

// Adds a literal iff t is already initialized.
func (t *UrnTargets) addLiteral(urn resource.URN) {
	if t.IsConstrained() {
		t.literals = append(t.literals, urn)
	}
}

// StepExecutorEvents is an interface that can be used to hook resource lifecycle events.
type StepExecutorEvents interface {
	OnResourceStepPre(step Step) (interface{}, error)
	OnResourceStepPost(ctx interface{}, step Step, status resource.Status, err error) error
	OnResourceOutputs(step Step) error
}

// PolicyEvents is an interface that can be used to hook policy events.
type PolicyEvents interface {
	OnPolicyViolation(resource.URN, plugin.AnalyzeDiagnostic)
	OnPolicyRemediation(resource.URN, plugin.Remediation, resource.PropertyMap, resource.PropertyMap)
}

// Events is an interface that can be used to hook interesting engine events.
type Events interface {
	StepExecutorEvents
	PolicyEvents
}

type goalMap struct {
	m sync.Map
}

func (m *goalMap) set(urn resource.URN, goal *resource.Goal) {
	m.m.Store(urn, goal)
}

func (m *goalMap) get(urn resource.URN) (*resource.Goal, bool) {
	g, ok := m.m.Load(urn)
	if !ok {
		return nil, false
	}
	return g.(*resource.Goal), true
}

type resourceMap struct {
	m sync.Map
}

func (m *resourceMap) set(urn resource.URN, state *resource.State) {
	m.m.Store(urn, state)
}

func (m *resourceMap) get(urn resource.URN) (*resource.State, bool) {
	s, ok := m.m.Load(urn)
	if !ok {
		return nil, false
	}
	return s.(*resource.State), true
}

func (m *resourceMap) mapRange(callback func(urn resource.URN, state *resource.State) bool) {
	m.m.Range(func(k, v interface{}) bool {
		return callback(k.(resource.URN), v.(*resource.State))
	})
}

type resourcePlans struct {
	m     sync.RWMutex
	plans Plan
}

func newResourcePlan(config config.Map) *resourcePlans {
	return &resourcePlans{
		plans: NewPlan(config),
	}
}

func (m *resourcePlans) set(urn resource.URN, plan *ResourcePlan) {
	m.m.Lock()
	defer m.m.Unlock()

	if _, ok := m.plans.ResourcePlans[urn]; ok {
		panic(fmt.Sprintf("tried to set resource plan for %s but it's already been set", urn))
	}

	m.plans.ResourcePlans[urn] = plan
}

func (m *resourcePlans) get(urn resource.URN) (*ResourcePlan, bool) {
	m.m.RLock()
	defer m.m.RUnlock()

	p, ok := m.plans.ResourcePlans[urn]
	return p, ok
}

func (m *resourcePlans) plan() *Plan {
	return &m.plans
}

// A Deployment manages the iterative computation and execution of a deployment based on a stream of goal states.
// A running deployment emits events that indicate its progress. These events must be used to record the new state
// of the deployment target.
type Deployment struct {
	ctx                  *plugin.Context                  // the plugin context (for provider operations).
	target               *Target                          // the deployment target.
	prev                 *Snapshot                        // the old resource snapshot for comparison.
	olds                 map[resource.URN]*resource.State // a map of all old resources.
	plan                 *Plan                            // a map of all planned resource changes, if any.
	imports              []Import                         // resources to import, if this is an import deployment.
	isImport             bool                             // true if this is an import deployment.
	schemaLoader         schema.Loader                    // the schema cache for this deployment, if any.
	source               Source                           // the source of new resources.
	localPolicyPackPaths []string                         // the policy packs to run during this deployment's generation.
	preview              bool                             // true if this deployment is to be previewed.
	depGraph             *graph.DependencyGraph           // the dependency graph of the old snapshot.
	providers            *providers.Registry              // the provider registry for this deployment.
	goals                *goalMap                         // the set of resource goals generated by the deployment.
	news                 *resourceMap                     // the set of new resources generated by the deployment
	newPlans             *resourcePlans                   // the set of new resource plans.
}

// addDefaultProviders adds any necessary default provider definitions and references to the given snapshot. Version
// information for these providers is sourced from the snapshot's manifest; inputs parameters are sourced from the
// stack's configuration.
func addDefaultProviders(target *Target, source Source, prev *Snapshot) error {
	if prev == nil {
		return nil
	}

	// Pull the versions we'll use for default providers from the snapshot's manifest.
	defaultProviderInfo := make(map[tokens.Package]workspace.PluginSpec)
	for _, p := range prev.Manifest.Plugins {
		defaultProviderInfo[tokens.Package(p.Name)] = p.Spec()
	}

	// Determine the necessary set of default providers and inject references to default providers as appropriate.
	//
	// We do this by scraping the snapshot for custom resources that does not reference a provider and adding
	// default providers for these resources' packages. Each of these resources is rewritten to reference the default
	// provider for its package.
	//
	// The configuration for each default provider is pulled from the stack's configuration information.
	var defaultProviders []*resource.State
	defaultProviderRefs := make(map[tokens.Package]providers.Reference)
	for _, res := range prev.Resources {
		if providers.IsProviderType(res.URN.Type()) || !res.Custom || res.Provider != "" {
			continue
		}

		pkg := res.URN.Type().Package()
		ref, ok := defaultProviderRefs[pkg]
		if !ok {
			inputs, err := target.GetPackageConfig(pkg)
			if err != nil {
				return fmt.Errorf("could not fetch configuration for default provider '%v'", pkg)
			}
			if pkgInfo, ok := defaultProviderInfo[pkg]; ok {
				providers.SetProviderVersion(inputs, pkgInfo.Version)
				providers.SetProviderURL(inputs, pkgInfo.PluginDownloadURL)
				providers.SetProviderChecksums(inputs, pkgInfo.Checksums)
			}

			uuid, err := uuid.NewV4()
			if err != nil {
				return err
			}

			urn, id := defaultProviderURN(target, source, pkg), resource.ID(uuid.String())
			ref, err = providers.NewReference(urn, id)
			contract.Assertf(err == nil,
				"could not create provider reference with URN %v and ID %v", urn, id)

			provider := &resource.State{
				Type:    urn.Type(),
				URN:     urn,
				Custom:  true,
				ID:      id,
				Inputs:  inputs,
				Outputs: inputs,
			}
			defaultProviders = append(defaultProviders, provider)
			defaultProviderRefs[pkg] = ref
		}
		res.Provider = ref.String()
	}

	// If any default providers are necessary, prepend their definitions to the snapshot's resources. This trivially
	// guarantees that all default provider references name providers that precede the referent in the snapshot.
	if len(defaultProviders) != 0 {
		prev.Resources = append(defaultProviders, prev.Resources...)
	}

	return nil
}

// migrateProviders is responsible for adding default providers to old snapshots and filling in output properties for
// providers that do not have them.
func migrateProviders(target *Target, prev *Snapshot, source Source) error {
	// Add any necessary default provider references to the previous snapshot in order to accommodate stacks that were
	// created prior to the changes that added first-class providers. We do this here rather than in the migration
	// package s.t. the inputs to any default providers (which we fetch from the stacks's configuration) are as
	// accurate as possible.
	if err := addDefaultProviders(target, source, prev); err != nil {
		return err
	}

	// Migrate provider resources from the old, output-less format to the new format where all inputs are reflected as
	// outputs.
	if prev != nil {
		for _, res := range prev.Resources {
			// If we have no old outputs for a provider, use its old inputs as its old outputs. This handles the
			// scenario where the CLI is being upgraded from a version that did not reflect provider inputs to
			// provider outputs, and a provider is being upgraded from a version that did not implement DiffConfig to
			// a version that does.
			if providers.IsProviderType(res.URN.Type()) && len(res.Inputs) != 0 && len(res.Outputs) == 0 {
				res.Outputs = res.Inputs
			}
		}
	}

	return nil
}

func buildResourceMap(prev *Snapshot, preview bool) ([]*resource.State, map[resource.URN]*resource.State, error) {
	olds := make(map[resource.URN]*resource.State)
	if prev == nil {
		return nil, olds, nil
	}

	for _, oldres := range prev.Resources {
		// Ignore resources that are pending deletion; these should not be recorded in the LUT.
		if oldres.Delete {
			continue
		}

		urn := oldres.URN
		if olds[urn] != nil {
			return nil, nil, fmt.Errorf("unexpected duplicate resource '%s'", urn)
		}
		olds[urn] = oldres
	}

	return prev.Resources, olds, nil
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
func NewDeployment(ctx *plugin.Context, target *Target, prev *Snapshot, plan *Plan, source Source,
	localPolicyPackPaths []string, preview bool, backendClient BackendClient,
) (*Deployment, error) {
	contract.Requiref(ctx != nil, "ctx", "must not be nil")
	contract.Requiref(target != nil, "target", "must not be nil")
	contract.Requiref(source != nil, "source", "must not be nil")

	if err := migrateProviders(target, prev, source); err != nil {
		return nil, err
	}

	// Produce a map of all old resources for fast access.
	//
	// NOTE: we can and do mutate prev.Resources, olds, and depGraph during execution after performing a refresh. See
	// deploymentExecutor.refresh for details.
	oldResources, olds, err := buildResourceMap(prev, preview)
	if err != nil {
		return nil, err
	}

	// Build the dependency graph for the old resources.
	depGraph := graph.NewDependencyGraph(oldResources)

	// Create a goal map for the deployment.
	newGoals := &goalMap{}

	// Create a resource map for the deployment.
	newResources := &resourceMap{}

	// Create a new builtin provider. This provider implements features such as `getStack`.
	builtins := newBuiltinProvider(backendClient, newResources)

	// Create a new provider registry. Although we really only need to pass in any providers that were present in the
	// old resource list, the registry itself will filter out other sorts of resources when processing the prior state,
	// so we just pass all of the old resources.
	reg := providers.NewRegistry(ctx.Host, preview, builtins)

	return &Deployment{
		ctx:                  ctx,
		target:               target,
		prev:                 prev,
		plan:                 plan,
		olds:                 olds,
		source:               source,
		localPolicyPackPaths: localPolicyPackPaths,
		preview:              preview,
		depGraph:             depGraph,
		providers:            reg,
		goals:                newGoals,
		news:                 newResources,
		newPlans:             newResourcePlan(target.Config),
	}, nil
}

func (d *Deployment) Ctx() *plugin.Context                   { return d.ctx }
func (d *Deployment) Target() *Target                        { return d.target }
func (d *Deployment) Diag() diag.Sink                        { return d.ctx.Diag }
func (d *Deployment) Prev() *Snapshot                        { return d.prev }
func (d *Deployment) Olds() map[resource.URN]*resource.State { return d.olds }
func (d *Deployment) Source() Source                         { return d.source }

func (d *Deployment) SameProvider(res *resource.State) error {
	return d.providers.Same(res)
}

// EnsureProvider ensures that the provider for the given resource is available in the registry. It assumes
// the provider is available in the previous snapshot.
func (d *Deployment) EnsureProvider(provider string) error {
	if provider == "" {
		return nil
	}

	providerRef, err := providers.ParseReference(provider)
	if err != nil {
		return fmt.Errorf("invalid provider reference %v: %w", provider, err)
	}
	_, has := d.GetProvider(providerRef)
	if !has {
		// We need to create the provider in the registry, find its old state and just "Same" it.
		var providerResource *resource.State
		for _, r := range d.prev.Resources {
			if r.URN == providerRef.URN() && r.ID == providerRef.ID() {
				providerResource = r
				break
			}
		}
		if providerResource == nil {
			return fmt.Errorf("could not find provider %v", providerRef)
		}

		err := d.SameProvider(providerResource)
		if err != nil {
			return fmt.Errorf("could not create provider %v: %w", providerRef, err)
		}
	}

	return nil
}

func (d *Deployment) GetProvider(ref providers.Reference) (plugin.Provider, bool) {
	return d.providers.GetProvider(ref)
}

// generateURN generates a resource's URN from its parent, type, and name under the scope of the deployment's stack and
// project.
func (d *Deployment) generateURN(parent resource.URN, ty tokens.Type, name string) resource.URN {
	// Use the resource goal state name to produce a globally unique URN.
	parentType := tokens.Type("")
	if parent != "" && parent.Type() != resource.RootStackType {
		// Skip empty parents and don't use the root stack type; otherwise, use the full qualified type.
		parentType = parent.QualifiedType()
	}

	return resource.NewURN(d.Target().Name.Q(), d.source.Project(), parentType, ty, name)
}

// defaultProviderURN generates the URN for the global provider given a package.
func defaultProviderURN(target *Target, source Source, pkg tokens.Package) resource.URN {
	return resource.NewURN(target.Name.Q(), source.Project(), "", providers.MakeProviderType(pkg), "default")
}

// generateEventURN generates a URN for the resource associated with the given event.
func (d *Deployment) generateEventURN(event SourceEvent) resource.URN {
	contract.Requiref(event != nil, "event", "must not be nil")

	switch e := event.(type) {
	case RegisterResourceEvent:
		goal := e.Goal()
		return d.generateURN(goal.Parent, goal.Type, goal.Name)
	case ReadResourceEvent:
		return d.generateURN(e.Parent(), e.Type(), e.Name())
	case RegisterResourceOutputsEvent:
		return e.URN()
	default:
		return ""
	}
}

// Execute executes a deployment to completion, using the given cancellation context and running a preview or update.
func (d *Deployment) Execute(ctx context.Context, opts Options, preview bool) (*Plan, error) {
	deploymentExec := &deploymentExecutor{deployment: d}
	return deploymentExec.Execute(ctx, opts, preview)
}
