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
	"fmt"
	"sort"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

// An Import specifies a resource to import.
type Import struct {
	Type     tokens.Type     // The type token for the resource. Required.
	Name     tokens.QName    // The name of the resource. Required.
	ID       resource.ID     // The ID of the resource. Required.
	Parent   resource.URN    // The parent of the resource, if any.
	Provider resource.URN    // The specific provider to use for the resource, if any.
	Version  *semver.Version // The provider version to use for the resource, if any.
	Protect  bool            // Whether to mark the resource as protected after import
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
func NewImportDeployment(ctx *plugin.Context, target *Target, projectName tokens.PackageName, imports []Import,
	preview bool) (*Deployment, error) {

	contract.Assert(ctx != nil)
	contract.Assert(target != nil)

	prev := target.Snapshot
	source := NewErrorSource(projectName)
	if err := migrateProviders(target, prev, source); err != nil {
		return nil, err
	}

	// Produce a map of all old resources for fast access.
	oldResources, olds, err := buildResourceMap(prev, preview)
	if err != nil {
		return nil, err
	}

	// Create a goal map for the deployment.
	newGoals := &goalMap{}

	builtins := newBuiltinProvider(nil, nil)

	// Create a new provider registry.
	reg, err := providers.NewRegistry(ctx.Host, oldResources, preview, builtins)
	if err != nil {
		return nil, err
	}

	// Return the prepared deployment.
	return &Deployment{
		ctx:          ctx,
		target:       target,
		prev:         prev,
		olds:         olds,
		goals:        newGoals,
		imports:      imports,
		isImport:     true,
		schemaLoader: schema.NewPluginLoader(ctx.Host),
		source:       NewErrorSource(projectName),
		preview:      preview,
		providers:    reg,
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
	preview    bool
}

func (i *importer) executeSerial(ctx context.Context, steps ...Step) bool {
	return i.wait(ctx, i.executor.ExecuteSerial(steps))
}

func (i *importer) executeParallel(ctx context.Context, steps ...Step) bool {
	return i.wait(ctx, i.executor.ExecuteParallel(steps))
}

func (i *importer) wait(ctx context.Context, token completionToken) bool {
	token.Wait(ctx)
	return ctx.Err() == nil && !i.executor.Errored()
}

func (i *importer) registerExistingResources(ctx context.Context) bool {
	if i != nil && i.deployment != nil && i.deployment.prev != nil {
		// Issue same steps per existing resource to make sure that they are recorded in the snapshot.
		// We issue these steps serially s.t. the resources remain in the order in which they appear in the state.
		for _, r := range i.deployment.prev.Resources {
			if r.Delete {
				continue
			}

			new := *r
			new.ID = ""
			if !i.executeSerial(ctx, NewSameStep(i.deployment, noopEvent(0), r, &new)) {
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
			if res.Type == resource.RootStackType {
				return res.URN, false, true
			}
		}
	}

	projectName, stackName := i.deployment.source.Project(), i.deployment.target.Name
	typ, name := resource.RootStackType, fmt.Sprintf("%s-%s", projectName, stackName)
	urn := resource.NewURN(stackName, projectName, "", typ, tokens.QName(name))
	state := resource.NewState(typ, urn, false, false, "", resource.PropertyMap{}, nil, "", false, false, nil, nil, "",
		nil, false, nil, nil, nil, "")
	if !i.executeSerial(ctx, NewCreateStep(i.deployment, noopEvent(0), state)) {
		return "", false, false
	}
	return urn, true, true
}

func (i *importer) registerProviders(ctx context.Context) (map[resource.URN]string, result.Result, bool) {
	urnToReference := map[resource.URN]string{}

	// Determine which default providers are not present in the state. If all default providers are accounted for,
	// we're done.
	//
	// NOTE: what if the configuration for an existing default provider has changed? If it has, we should diff it and
	// replace it appropriately or we should not use the ambient config at all.
	var defaultProviderRequests []providers.ProviderRequest
	defaultProviders := map[resource.URN]struct{}{}
	for _, imp := range i.deployment.imports {
		if imp.Provider != "" {
			// If the provider for this import exists, map its URN to its provider reference. If it does not exist,
			// the import step will issue an appropriate error or errors.
			ref := string(imp.Provider)
			if state, ok := i.deployment.olds[imp.Provider]; ok {
				r, err := providers.NewReference(imp.Provider, state.ID)
				contract.AssertNoError(err)
				ref = r.String()
			}
			urnToReference[imp.Provider] = ref
			continue
		}

		if imp.Type.Package() == "" {
			return nil, result.Error("incorrect package type specified"), false
		}
		req := providers.NewProviderRequest(imp.Version, imp.Type.Package())
		typ, name := providers.MakeProviderType(req.Package()), req.Name()
		urn := i.deployment.generateURN("", typ, name)
		if state, ok := i.deployment.olds[urn]; ok {
			ref, err := providers.NewReference(urn, state.ID)
			contract.AssertNoError(err)
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
		return urnToReference, nil, true
	}

	steps := make([]Step, len(defaultProviderRequests))
	sort.Slice(defaultProviderRequests, func(i, j int) bool {
		return defaultProviderRequests[i].String() < defaultProviderRequests[j].String()
	})
	for idx, req := range defaultProviderRequests {
		if req.Package() == "" {
			return nil, result.Error("incorrect package type specified"), false
		}

		typ, name := providers.MakeProviderType(req.Package()), req.Name()
		urn := i.deployment.generateURN("", typ, name)

		// Fetch, prepare, and check the configuration for this provider.
		inputs, err := i.deployment.target.GetPackageConfig(req.Package())
		if err != nil {
			return nil, result.Errorf("failed to fetch provider config: %v", err), false
		}

		// Calculate the inputs for the provider using the ambient config.
		if v := req.Version(); v != nil {
			inputs["version"] = resource.NewStringProperty(v.String())
		}
		inputs, failures, err := i.deployment.providers.Check(urn, nil, inputs, false)
		if err != nil {
			return nil, result.Errorf("failed to validate provider config: %v", err), false
		}

		state := resource.NewState(typ, urn, true, false, "", inputs, nil, "", false, false, nil, nil, "", nil, false,
			nil, nil, nil, "")
		if issueCheckErrors(i.deployment, state, urn, failures) {
			return nil, nil, false
		}

		steps[idx] = NewCreateStep(i.deployment, noopEvent(0), state)
	}

	// Issue the create steps.
	if !i.executeParallel(ctx, steps...) {
		return nil, nil, false
	}

	// Update the URN to reference map.
	for _, s := range steps {
		res := s.Res()
		id := res.ID
		if i.preview {
			id = providers.UnknownID
		}
		ref, err := providers.NewReference(res.URN, id)
		contract.AssertNoError(err)
		urnToReference[res.URN] = ref.String()
	}

	return urnToReference, nil, true
}

func (i *importer) importResources(ctx context.Context) result.Result {
	contract.Assert(len(i.deployment.imports) != 0)

	if !i.registerExistingResources(ctx) {
		return nil
	}

	stackURN, createdStack, ok := i.getOrCreateStackResource(ctx)
	if !ok {
		return nil
	}

	urnToReference, res, ok := i.registerProviders(ctx)
	if !ok {
		return res
	}

	// Create a step per resource to import and execute them in parallel. If there are duplicates, fail the import.
	urns := map[resource.URN]struct{}{}
	steps := make([]Step, 0, len(i.deployment.imports))
	for _, imp := range i.deployment.imports {
		parent := imp.Parent
		if parent == "" {
			parent = stackURN
		}
		urn := i.deployment.generateURN(parent, imp.Type, imp.Name)

		// Check for duplicate imports.
		if _, has := urns[urn]; has {
			return result.Errorf("duplicate import '%v' of type '%v'", imp.Name, imp.Type)
		}
		urns[urn] = struct{}{}

		// If the resource already exists and the ID matches the ID to import, skip this resource. If the ID does
		// not match, the step itself will issue an error.
		if old, ok := i.deployment.olds[urn]; ok {
			oldID := old.ID
			if old.ImportID != "" {
				oldID = old.ImportID
			}
			if oldID == imp.ID {
				continue
			}
		}

		providerURN := imp.Provider
		if providerURN == "" {
			req := providers.NewProviderRequest(imp.Version, imp.Type.Package())
			typ, name := providers.MakeProviderType(req.Package()), req.Name()
			providerURN = i.deployment.generateURN("", typ, name)
		}

		// Fetch the provider reference for this import. All provider URNs should be mapped.
		provider, ok := urnToReference[providerURN]
		contract.Assert(ok)

		// Create the new desired state. Note that the resource is protected.
		new := resource.NewState(urn.Type(), urn, true, false, imp.ID, resource.PropertyMap{}, nil, parent, imp.Protect,
			false, nil, nil, provider, nil, false, nil, nil, nil, "")
		steps = append(steps, newImportDeploymentStep(i.deployment, new))
	}

	if !i.executeParallel(ctx, steps...) {
		return nil
	}

	if createdStack {
		i.executor.ExecuteRegisterResourceOutputs(noopOutputsEvent(stackURN))
	}

	return nil
}
