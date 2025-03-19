// Copyright 2016-2020, Pulumi Corporation.
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
	cryptorand "crypto/rand"
	"errors"
	"fmt"
	"sort"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/util/gsync"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type Parameterization struct {
	// The base plugin name to use for this parameterization.
	PluginName tokens.Package
	// The version of the plugin to use for this parameterization.
	PluginVersion semver.Version
	// The value to use for this parameterization.
	Value []byte
}

// ToProviderParameterization converts a workspace parameterization to a provider parameterization.
func (p *Parameterization) ToProviderParameterization(
	typ tokens.Type, version *semver.Version,
) (tokens.Package, *semver.Version, *workspace.Parameterization, error) {
	if p == nil {
		return typ.Package(), version, nil, nil
	}

	if version == nil {
		return "", nil, nil, errors.New("version must be provided")
	}

	return p.PluginName, &p.PluginVersion, &workspace.Parameterization{
		Name:    string(typ.Package()),
		Version: *version,
		Value:   p.Value,
	}, nil
}

// An Import specifies a resource to import.
type Import struct {
	Type              tokens.Type       // The type token for the resource. Required.
	Name              string            // The name of the resource. Required.
	ID                resource.ID       // The ID of the resource. Required.
	Parent            resource.URN      // The parent of the resource, if any.
	Provider          resource.URN      // The specific provider to use for the resource, if any.
	Version           *semver.Version   // The provider version to use for the resource, if any.
	PluginDownloadURL string            // The provider PluginDownloadURL to use for the resource, if any.
	PluginChecksums   map[string][]byte // The provider checksums to use for the resource, if any.
	Protect           bool              // Whether to mark the resource as protected after import
	Properties        []string          // Which properties to include (Defaults to required properties)
	Parameterization  *Parameterization // The parameterization to use for the resource, if any.

	// True if this import should create an empty component resource. ID must not be set if this is used.
	Component bool
	// True if this is a remote component resource. Component must be true if this is true.
	Remote bool
}

// ImportOptions controls the import process.
type ImportOptions struct {
	Events   Events // an optional events callback interface.
	Parallel int    // the degree of parallelism for resource operations (<=1 for serial).
}

// NewImportDeployment creates a new import deployment from a resource snapshot plus a set of resources to import.
//
// From the old and new states, it understands how to orchestrate an evaluation and analyze the resulting resources.
// The deployment may be used to simply inspect a series of operations, or actually perform them; these operations are
// generated based on analysis of the old and new states.  If a resource exists in new, but not old, for example, it
// results in a create; if it exists in both, but is different, it results in an update; and so on and so forth.
//
// Note that a deployment uses internal concurrency and parallelism in various ways, so it must be closed if for some
// reason it isn't carried out to its final conclusion. This will result in cancellation and reclamation of resources.
func NewImportDeployment(
	ctx *plugin.Context,
	opts *Options,
	events Events,
	target *Target,
	projectName tokens.PackageName,
	imports []Import,
) (*Deployment, error) {
	contract.Requiref(ctx != nil, "ctx", "must not be nil")
	contract.Requiref(target != nil, "target", "must not be nil")

	prev := target.Snapshot
	source := NewErrorSource(projectName)
	if err := migrateProviders(target, prev, source); err != nil {
		return nil, err
	}

	// Produce a map of all old resources for fast access.
	_, olds, err := buildResourceMap(prev, opts.DryRun)
	if err != nil {
		return nil, err
	}

	// Create a goal map for the deployment.
	newGoals := &gsync.Map[resource.URN, *resource.Goal]{}

	builtins := newBuiltinProvider(
		nil, /*backendClient*/
		nil, /*news*/
		nil, /*reads*/
		ctx.Diag,
	)

	// Create a new provider registry.
	reg := providers.NewRegistry(ctx.Host, opts.DryRun, builtins)

	// Return the prepared deployment.
	return &Deployment{
		ctx:          ctx,
		opts:         opts,
		events:       events,
		target:       target,
		prev:         prev,
		olds:         olds,
		goals:        newGoals,
		imports:      imports,
		isImport:     true,
		schemaLoader: schema.NewPluginLoader(ctx.Host),
		source:       NewErrorSource(projectName),
		providers:    reg,
		newPlans:     newResourcePlan(target.Config),
		news:         &gsync.Map[resource.URN, *resource.State]{},
	}, nil
}

type noopEvent int

func (noopEvent) event()                      {}
func (noopEvent) Goal() *resource.Goal        { return nil }
func (noopEvent) Done(result *RegisterResult) {}

type noopOutputsEvent resource.URN

func (noopOutputsEvent) event()                        {}
func (e noopOutputsEvent) URN() resource.URN           { return resource.URN(e) }
func (noopOutputsEvent) Outputs() resource.PropertyMap { return resource.PropertyMap{} }
func (noopOutputsEvent) Done()                         {}

type importer struct {
	deployment *Deployment
	executor   *stepExecutor
}

func (i *importer) executeSerial(ctx context.Context, steps ...Step) bool {
	return i.wait(ctx, i.executor.ExecuteSerial(steps))
}

func (i *importer) executeParallel(ctx context.Context, steps ...Step) bool {
	return i.wait(ctx, i.executor.ExecuteParallel(steps))
}

func (i *importer) wait(ctx context.Context, token completionToken) bool {
	token.Wait(ctx)
	return ctx.Err() == nil && i.executor.Errored() == nil
}

func (i *importer) registerExistingResources(ctx context.Context) bool {
	if i != nil && i.deployment != nil && i.deployment.prev != nil {
		// Issue same steps per existing resource to make sure that they are recorded in the snapshot.
		// We issue these steps serially s.t. the resources remain in the order in which they appear in the state.
		for _, r := range i.deployment.prev.Resources {
			if r.Delete {
				continue
			}

			// Clear the ID because Same asserts that the new state has no ID.
			new := r.Copy()
			new.ID = ""
			// Set a dummy goal so the resource is tracked as managed.
			i.deployment.goals.Store(r.URN, &resource.Goal{})
			if !i.executeSerial(ctx, NewSameStep(i.deployment, noopEvent(0), r, new)) {
				return false
			}
		}
	}
	return true
}

func (i *importer) getOrCreateStackResource(ctx context.Context) (resource.URN, bool, bool) {
	// Get or create the root resource.
	if i.deployment.prev != nil {
		for _, res := range i.deployment.prev.Resources {
			if res.Type == resource.RootStackType && res.Parent == "" {
				return res.URN, false, true
			}
		}
	}

	projectName, stackName := i.deployment.source.Project(), i.deployment.target.Name
	typ, name := resource.RootStackType, fmt.Sprintf("%s-%s", projectName, stackName)
	urn := resource.NewURN(stackName.Q(), projectName, "", typ, name)
	state := resource.NewState(typ, urn, false, false, "", resource.PropertyMap{}, nil, "", false, false, nil, nil, "",
		nil, false, nil, nil, nil, "", false, "", nil, nil, "", nil)
	// TODO(seqnum) should stacks be created with 1? When do they ever get recreated/replaced?
	if !i.executeSerial(ctx, NewCreateStep(i.deployment, noopEvent(0), state)) {
		return "", false, false
	}
	return urn, true, true
}

func (i *importer) registerProviders(ctx context.Context) (map[resource.URN]string, bool, error) {
	urnToReference := map[resource.URN]string{}

	// Determine which default providers are not present in the state. If all default providers are accounted for,
	// we're done.
	//
	// NOTE: what if the configuration for an existing default provider has changed? If it has, we should diff it and
	// replace it appropriately or we should not use the ambient config at all.
	defaultProviderRequests := slice.Prealloc[providers.ProviderRequest](len(i.deployment.imports))
	defaultProviders := map[resource.URN]struct{}{}
	for _, imp := range i.deployment.imports {
		if imp.Component && !imp.Remote {
			// Skip local component resources, they don't have providers.
			continue
		}

		if imp.Provider != "" {
			// If the provider for this import exists, map its URN to its provider reference. If it does not exist,
			// the import step will issue an appropriate error or errors.
			ref := string(imp.Provider)
			if state, ok := i.deployment.olds[imp.Provider]; ok {
				r, err := providers.NewReference(imp.Provider, state.ID)
				contract.AssertNoErrorf(err,
					"could not create provider reference with URN %q and ID %q", imp.Provider, state.ID)
				ref = r.String()
			}
			urnToReference[imp.Provider] = ref
			continue
		}

		if imp.Type.Package() == "" {
			return nil, false, errors.New("incorrect package type specified")
		}

		pkg, version, parameterization, err := imp.Parameterization.ToProviderParameterization(imp.Type, imp.Version)
		if err != nil {
			return nil, false, err
		}
		req := providers.NewProviderRequest(
			pkg, version, imp.PluginDownloadURL, imp.PluginChecksums, parameterization)
		typ, name := providers.MakeProviderType(req.Package()), req.DefaultName()
		urn := i.deployment.generateURN("", typ, name)
		if state, ok := i.deployment.olds[urn]; ok {
			ref, err := providers.NewReference(urn, state.ID)
			contract.AssertNoErrorf(err,
				"could not create provider reference with URN %q and ID %q", urn, state.ID)
			urnToReference[urn] = ref.String()
			continue
		}
		if _, ok := defaultProviders[urn]; ok {
			continue
		}

		defaultProviderRequests = append(defaultProviderRequests, req)
		defaultProviders[urn] = struct{}{}
	}
	if len(defaultProviderRequests) == 0 {
		return urnToReference, true, nil
	}

	steps := make([]Step, len(defaultProviderRequests))
	sort.Slice(defaultProviderRequests, func(i, j int) bool {
		return defaultProviderRequests[i].String() < defaultProviderRequests[j].String()
	})
	for idx, req := range defaultProviderRequests {
		if req.Package() == "" {
			return nil, false, errors.New("incorrect package type specified")
		}

		typ, name := providers.MakeProviderType(req.Package()), req.DefaultName()
		urn := i.deployment.generateURN("", typ, name)

		// Fetch, prepare, and check the configuration for this provider.
		inputs, err := i.deployment.target.GetPackageConfig(req.Package())
		if err != nil {
			return nil, false, fmt.Errorf("failed to fetch provider config: %w", err)
		}

		// Calculate the inputs for the provider using the ambient config.
		if v := req.Version(); v != nil {
			providers.SetProviderVersion(inputs, v)
		}
		if url := req.PluginDownloadURL(); url != "" {
			providers.SetProviderURL(inputs, url)
		}
		if checksums := req.PluginChecksums(); checksums != nil {
			providers.SetProviderChecksums(inputs, checksums)
		}
		if parameterization := req.Parameterization(); parameterization != nil {
			providers.SetProviderName(inputs, req.Name())
			providers.SetProviderParameterization(inputs, parameterization)
		}
		resp, err := i.deployment.providers.Check(ctx, plugin.CheckRequest{
			URN:  urn,
			News: inputs,
		})
		if err != nil {
			return nil, false, fmt.Errorf("failed to validate provider config: %w", err)
		}

		state := resource.NewState(typ, urn, true, false, "", inputs, nil, "", false, false, nil, nil, "", nil, false,
			nil, nil, nil, "", false, "", nil, nil, "", nil)
		// TODO(seqnum) should default providers be created with 1? When do they ever get recreated/replaced?
		if issueCheckErrors(i.deployment, state, urn, resp.Failures) {
			return nil, false, nil
		}

		// Set a dummy goal so the resource is tracked as managed.
		i.deployment.goals.Store(urn, &resource.Goal{})
		steps[idx] = NewCreateStep(i.deployment, noopEvent(0), state)
	}

	// Issue the create steps.
	if !i.executeParallel(ctx, steps...) {
		return nil, false, nil
	}

	// Update the URN to reference map.
	for _, s := range steps {
		res := s.Res()
		ref, err := providers.NewReference(res.URN, res.ID)
		contract.AssertNoErrorf(err, "could not create provider reference with URN %q and ID %q", res.URN, res.ID)
		urnToReference[res.URN] = ref.String()
	}

	return urnToReference, true, nil
}

func (i *importer) importResources(ctx context.Context) error {
	contract.Assertf(len(i.deployment.imports) != 0, "no resources to import")

	if !i.registerExistingResources(ctx) {
		return nil
	}

	stackURN, createdStack, ok := i.getOrCreateStackResource(ctx)
	if !ok {
		return nil
	}

	urnToReference, ok, err := i.registerProviders(ctx)
	if !ok {
		return err
	}

	// Create a step per resource to import and execute them in parallel batches which don't depend on each other.
	// If there are duplicates, fail the import.
	urns := map[resource.URN]struct{}{}
	steps := slice.Prealloc[Step](len(i.deployment.imports))
	for _, imp := range i.deployment.imports {
		parent := imp.Parent
		if parent == "" {
			parent = stackURN
		}
		urn := i.deployment.generateURN(parent, imp.Type, imp.Name)

		// Check for duplicate imports.
		if _, has := urns[urn]; has {
			return fmt.Errorf("duplicate import '%v' of type '%v'", imp.Name, imp.Type)
		}
		urns[urn] = struct{}{}

		// If the resource already exists and the ID matches the ID to import, then Same this resource. If the ID does
		// not match, the step itself will issue an error.
		if old, ok := i.deployment.olds[urn]; ok {
			oldID := old.ID
			if old.ImportID != "" {
				oldID = old.ImportID
			}
			if oldID == imp.ID {
				// Clear the ID because Same asserts that the new state has no ID.
				new := old.Copy()
				new.ID = ""
				// Set a dummy goal so the resource is tracked as managed.
				i.deployment.goals.Store(old.URN, &resource.Goal{})
				steps = append(steps, NewSameStep(i.deployment, noopEvent(0), old, new))
				continue
			}
		}

		providerURN := imp.Provider
		if providerURN == "" && (!imp.Component || imp.Remote) {
			pkg, version, parameterization, err := imp.Parameterization.ToProviderParameterization(imp.Type, imp.Version)
			if err != nil {
				return err
			}
			req := providers.NewProviderRequest(
				pkg, version, imp.PluginDownloadURL, imp.PluginChecksums, parameterization)
			typ, name := providers.MakeProviderType(req.Package()), req.DefaultName()
			providerURN = i.deployment.generateURN("", typ, name)
		}

		var provider string
		if providerURN != "" {
			// Fetch the provider reference for this import. All provider URNs should be mapped.
			provider, ok = urnToReference[providerURN]
			contract.Assertf(ok, "provider reference for URN %v not found", providerURN)
		}

		// Create the new desired state. Note that the resource is protected. Provider might be "" at this point.
		new := resource.NewState(
			urn.Type(), urn, !imp.Component, false, imp.ID, resource.PropertyMap{}, nil, parent, imp.Protect,
			false, nil, nil, provider, nil, false, nil, nil, nil, "", false, "", nil, nil, "", nil)
		// Set a dummy goal so the resource is tracked as managed.
		i.deployment.goals.Store(urn, &resource.Goal{})

		if imp.Component {
			if imp.Remote {
				contract.Assertf(ok, "provider reference for URN %v not found", providerURN)
			}

			steps = append(steps, newImportDeploymentStep(i.deployment, new, nil))
		} else {
			contract.Assertf(ok, "provider reference for URN %v not found", providerURN)

			// If we have a plan for this resource we need to feed the saved seed to Check to remove non-determinism
			var randomSeed []byte
			if i.deployment.plan != nil {
				if resourcePlan, ok := i.deployment.plan.ResourcePlans[urn]; ok {
					randomSeed = resourcePlan.Seed
				}
			} else {
				randomSeed = make([]byte, 32)
				n, err := cryptorand.Read(randomSeed)
				contract.AssertNoErrorf(err, "could not read random bytes")
				contract.Assertf(n == len(randomSeed), "read %d random bytes, expected %d", n, len(randomSeed))
			}

			steps = append(steps, newImportDeploymentStep(i.deployment, new, randomSeed))
		}
	}

	// We've created all the steps above but we need to execute them in parallel batches which don't depend on each other
	for len(urns) > 0 {
		// Find all the steps that can be executed in parallel. `urns` is a map of every resource we still
		// need to import so if we need a resource from that map we can't yet build this resource.
		parallelSteps := []Step{}
		for _, step := range steps {
			// If we've already done this step don't do it again
			if _, ok := urns[step.New().URN]; !ok {
				continue
			}

			// If the step has no dependencies (we actually only need to look at parent), it can be executed in parallel
			if _, ok := urns[step.New().Parent]; !ok {
				parallelSteps = append(parallelSteps, step)
			}
		}

		// Remove all the urns we're about to import
		for _, step := range parallelSteps {
			delete(urns, step.New().URN)
		}

		if !i.executeParallel(ctx, parallelSteps...) {
			return nil
		}
	}

	if createdStack {
		return i.executor.ExecuteRegisterResourceOutputs(noopOutputsEvent(stackURN))
	}

	return nil
}
