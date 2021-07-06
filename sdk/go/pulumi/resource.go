// Copyright 2016-2021, Pulumi Corporation.
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

package pulumi

import (
	"context"
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

type (
	// ID is a unique identifier assigned by a resource provider to a resource.
	ID string
	// URN is an automatically generated logical URN, used to stably identify resources.
	URN string
)

var resourceStateType = reflect.TypeOf(ResourceState{})
var customResourceStateType = reflect.TypeOf(CustomResourceState{})
var providerResourceStateType = reflect.TypeOf(ProviderResourceState{})

// ResourceState is the base
type ResourceState struct {
	urn                    URNOutput `pulumi:"urn"`
	name                   string
	aliasesPromise         *aliasesPromise
	providersPromise       *providersPromise
	transformationsPromise *transformationsPromise
	addedTransformations   []ResourceTransformation
}

func (s *ResourceState) getProvidersPromise() *providersPromise {
	return initProvidersPromise(&s.providersPromise)
}

func (s *ResourceState) getAliasesPromise() *aliasesPromise {
	return initAliasesPromise(&s.aliasesPromise)
}

func (s *ResourceState) getTransformationsPromise() *transformationsPromise {
	return initTransformationsPromise(fmt.Sprintf("ResourceState name=%s", s.name), &s.transformationsPromise)
}

func (s *ResourceState) URN() URNOutput {
	return s.urn
}

func (s *ResourceState) GetProvider(token string) ProviderResource {
	return s.getProviders()[getPackage(token)]
}

func (s *ResourceState) getProviders() map[string]ProviderResource {
	return s.getProvidersPromise().await()
}

func (s *ResourceState) getAliases() []URNOutput {
	return s.getAliasesPromise().await()
}

func (s *ResourceState) getName() string {
	return s.name
}

func (s *ResourceState) getTransformations() []ResourceTransformation {
	return append(s.getTransformationsPromise().await("ResourceState.getTransformations()"), s.addedTransformations...)
}

func (s *ResourceState) addTransformation(t ResourceTransformation) {
	s.addedTransformations = append(s.addedTransformations, t)
}

func (ResourceState) isResource() {}

func (ctx *Context) newDependencyResource(urn URN) Resource {
	var res ResourceState
	initDependencyResource(&res)
	res.urn.OutputState = ctx.newOutputState(res.urn.ElementType(), &res)
	res.urn.resolve(urn, true, false, nil)
	return &res
}

type CustomResourceState struct {
	ResourceState

	id IDOutput `pulumi:"id"`
}

func (s CustomResourceState) ID() IDOutput {
	return s.id
}

func (CustomResourceState) isCustomResource() {}

func (ctx *Context) newDependencyCustomResource(urn URN, id ID) CustomResource {
	var res CustomResourceState
	initDependencyResource(&res.ResourceState)
	res.urn.OutputState = ctx.newOutputState(res.urn.ElementType(), &res)
	res.urn.resolve(urn, true, false, nil)
	res.id.OutputState = ctx.newOutputState(res.id.ElementType(), &res)
	res.id.resolve(id, id != "", false, nil)
	return &res
}

type ProviderResourceState struct {
	CustomResourceState

	pkg string
}

func (s ProviderResourceState) getPackage() string {
	return s.pkg
}

func (ctx *Context) newDependencyProviderResource(urn URN, id ID) ProviderResource {
	var res ProviderResourceState
	initDependencyResource(&res.ResourceState)
	res.urn.OutputState = ctx.newOutputState(res.urn.ElementType(), &res)
	res.id.OutputState = ctx.newOutputState(res.id.ElementType(), &res)
	res.urn.resolve(urn, true, false, nil)
	res.id.resolve(id, id != "", false, nil)
	res.pkg = string(resource.URN(urn).Type().Name())
	return &res
}

// Initialize a resource state for a dependency resource (one created
// from URN) and not registered in the SDK.
func initDependencyResource(resource *ResourceState) {
	// Assume it has no transformations. If the resource is
	// provisioned on a different language runtime, cross-language
	// transformations are not yet supported.
	resource.getTransformationsPromise().fulfill([]ResourceTransformation{})

	// Similarly, assume there are no aliases. This is not
	// something we can currently look up via `ctx.getResource`.
	resource.getAliasesPromise().fulfill([]URNOutput{})

	// Ditto for the providers map.
	resource.getProvidersPromise().fulfill(map[string]ProviderResource{})
}

// Resource represents a cloud resource managed by Pulumi.
type Resource interface {
	// URN is this resource's stable logical URN used to distinctly address it before, during, and after deployments.
	URN() URNOutput

	// getProviders returns the provider map for this resource.
	getProviders() map[string]ProviderResource

	// getAliases returns the list of aliases for this resource
	getAliases() []URNOutput

	// getName returns the name of the resource
	getName() string

	// isResource() is a marker method used to ensure that all Resource types embed a ResourceState.
	isResource()

	// getTransformations returns the transformations for the resource.
	getTransformations() []ResourceTransformation

	// addTransformation adds a single transformation to the resource.
	addTransformation(t ResourceTransformation)
}

// CustomResource is a cloud resource whose create, read, update, and delete (CRUD) operations are managed by performing
// external operations on some physical entity.  The engine understands how to diff and perform partial updates of them,
// and these CRUD operations are implemented in a dynamically loaded plugin for the defining package.
type CustomResource interface {
	Resource
	// ID is the provider-assigned unique identifier for this managed resource.  It is set during deployments,
	// but might be missing ("") during planning phases.
	ID() IDOutput

	isCustomResource()
}

// ComponentResource is a resource that aggregates one or more other child resources into a higher level abstraction.
// The component resource itself is a resource, but does not require custom CRUD operations for provisioning.
type ComponentResource interface {
	Resource
}

// ProviderResource is a resource that represents a configured instance of a particular package's provider plugin.
// These resources are supply the implementations of their package's CRUD operations. A specific provider instance can
// be used for a given resource by passing it in ResourceOpt.Provider.
type ProviderResource interface {
	CustomResource

	getPackage() string
}

type CustomTimeouts struct {
	Create string
	Update string
	Delete string
}

type resourceOptions struct {
	// AdditionalSecretOutputs is an optional list of output properties to mark as secret.
	AdditionalSecretOutputs []string
	// Aliases is an optional list of identifiers used to find and use existing resources.
	Aliases []Alias
	// CustomTimeouts is an optional configuration block used for CRUD operations
	CustomTimeouts *CustomTimeouts
	// DeleteBeforeReplace, when set to true, ensures that this resource is deleted prior to replacement.
	DeleteBeforeReplace bool
	// DependsOn is an optional array of explicit dependencies on other resources.
	DependsOn []Resource
	// IgnoreChanges ignores changes to any of the specified properties.
	IgnoreChanges []string
	// Import, when provided with a resource ID, indicates that this resource's provider should import its state from
	// the cloud resource with the given ID. The inputs to the resource's constructor must align with the resource's
	// current state. Once a resource has been imported, the import property must be removed from the resource's
	// options.
	Import IDInput
	// Parent is an optional parent resource to which this resource belongs.
	Parent Resource
	// Protect, when set to true, ensures that this resource cannot be deleted (without first setting it to false).
	Protect bool
	// Provider is an optional provider resource to use for this resource's CRUD operations.
	Provider ProviderResource
	// Providers is an optional map of package to provider resource for a component resource.
	Providers map[string]ProviderResource
	// ReplaceOnChanges will force a replacement when any of these property paths are set.  If this list includes `"*"`,
	// changes to any properties will force a replacement.  Initialization errors from previous deployments will
	// require replacement instead of update only if `"*"` is passed.
	ReplaceOnChanges []string
	// Transformations is an optional list of transformations to apply to this resource during construction.
	// The transformations are applied in order, and are applied prior to transformation and to parents
	// walking from the resource up to the stack.
	Transformations []ResourceTransformation
	// URN is an optional URN of a previously-registered resource of this type to read from the engine.
	URN string
	// Version is an optional version, corresponding to the version of the provider plugin that should be used when
	// operating on this resource. This version overrides the version information inferred from the current package and
	// should rarely be used.
	Version string
}

func awaitResourceInput(ctx context.Context, ri ResourceInput) (Resource, []Resource, error) {
	result, known, _, deps, err := ri.ToResourceOutput().await(ctx)

	if !known {
		return nil, nil, fmt.Errorf("Encountered unknown ResourceInput, this is currently not supported")
	}

	resource, isResource := result.(Resource)

	if !isResource {
		return nil, nil, fmt.Errorf("ResourceInput resolved to a value that is not a Resource but a %v",
			reflect.TypeOf(result))
	}

	return resource, deps, err
}

func awaitProviderResourceInput(ctx context.Context, pri ProviderResourceInput) (ProviderResource, []Resource, error) {
	result, known, _, deps, err := pri.ToProviderResourceOutput().await(ctx)

	if !known {
		return nil, nil, fmt.Errorf("Encountered unknown ProviderResourceInput, this is currently not supported")
	}

	resource, isResource := result.(ProviderResource)
	if !isResource {
		return nil, nil, fmt.Errorf("ProviderResourceInput resolved to a value that is not a Resource but a %v",
			reflect.TypeOf(result))
	}

	return resource, deps, err
}

type invokeOptions struct {
	// Parent is an optional parent resource to use for default provider options for this invoke.
	Parent Resource
	// Provider is an optional provider resource to use for this invoke.
	Provider ProviderResource
	// Version is an optional version of the provider plugin to use for the invoke.
	Version string
}

type ResourceOption interface {
	// If true, clients must use
	// applyResourceOptionAfterAwaitingInputs, otherwise clients
	// may use applyResourceOptionImmediately without awaiting.
	containsInputs() bool

	applyResourceOptionImmediately(*resourceOptions) error

	applyResourceOptionAfterAwaitingInputs(context.Context, *resourceOptions) error
}

type InvokeOption interface {
	containsInputs() bool
	applyInvokeOptionImmediately(*invokeOptions) error
	applyInvokeOptionAfterAwaitingInputs(context.Context, *invokeOptions) error
}

type ResourceOrInvokeOption interface {
	ResourceOption
	InvokeOption
}

type resourceOptionWithInputs func(context.Context, *resourceOptions) error

var _ ResourceOption = *new(resourceOptionWithInputs)

func (o resourceOptionWithInputs) containsInputs() bool {
	return true
}

func (o resourceOptionWithInputs) applyResourceOptionImmediately(opts *resourceOptions) error {
	return fmt.Errorf("This option contains Input values and " +
		"needs to be called with applyResourceOptionAfterAwaitingInputs")
}

func (o resourceOptionWithInputs) applyResourceOptionAfterAwaitingInputs(
	ctx context.Context, opts *resourceOptions) error {
	return o(ctx, opts)
}

type resourceOption func(*resourceOptions)

var _ ResourceOption = *new(resourceOption)

func (o resourceOption) containsInputs() bool {
	return false
}

func (o resourceOption) applyResourceOptionImmediately(opts *resourceOptions) error {
	o(opts)
	return nil
}

func (o resourceOption) applyResourceOptionAfterAwaitingInputs(ctx context.Context, opts *resourceOptions) error {
	o(opts)
	return nil
}

type resourceOrInvokeOption func(ro *resourceOptions, io *invokeOptions)

var _ ResourceOrInvokeOption = *new(resourceOrInvokeOption)

func (o resourceOrInvokeOption) toResourceOption() resourceOption {
	return resourceOption(func(opts *resourceOptions) { o(opts, nil) })
}

func (o resourceOrInvokeOption) containsInputs() bool {
	return o.toResourceOption().containsInputs()
}

func (o resourceOrInvokeOption) applyResourceOptionImmediately(opts *resourceOptions) error {
	return o.toResourceOption().applyResourceOptionImmediately(opts)
}

func (o resourceOrInvokeOption) applyResourceOptionAfterAwaitingInputs(
	ctx context.Context, opts *resourceOptions) error {
	return o.toResourceOption().applyResourceOptionAfterAwaitingInputs(ctx, opts)
}

func (o resourceOrInvokeOption) applyInvokeOptionImmediately(opts *invokeOptions) error {
	o(nil, opts)
	return nil
}

func (o resourceOrInvokeOption) applyInvokeOptionAfterAwaitingInputs(ctx context.Context, opts *invokeOptions) error {
	o(nil, opts)
	return nil
}

type resourceOrInvokeOptionWithInputs func(ctx context.Context, ro *resourceOptions, io *invokeOptions) error

var _ ResourceOrInvokeOption = *new(resourceOrInvokeOptionWithInputs)

func (o resourceOrInvokeOptionWithInputs) toResourceOptionWithInputs() resourceOptionWithInputs {
	return resourceOptionWithInputs(func(ctx context.Context, opts *resourceOptions) error {
		return o(ctx, opts, nil)
	})
}

func (o resourceOrInvokeOptionWithInputs) containsInputs() bool {
	return o.toResourceOptionWithInputs().containsInputs()
}

func (o resourceOrInvokeOptionWithInputs) applyResourceOptionImmediately(opts *resourceOptions) error {
	return o.toResourceOptionWithInputs().applyResourceOptionImmediately(opts)
}

func (o resourceOrInvokeOptionWithInputs) applyResourceOptionAfterAwaitingInputs(
	ctx context.Context, opts *resourceOptions) error {
	return o.toResourceOptionWithInputs().applyResourceOptionAfterAwaitingInputs(ctx, opts)
}

func (o resourceOrInvokeOptionWithInputs) applyInvokeOptionImmediately(opts *invokeOptions) error {
	return fmt.Errorf("This option contains Input values and " +
		"needs to be called with applyInvokeOptionAfterAwaitingInputs")
}

func (o resourceOrInvokeOptionWithInputs) applyInvokeOptionAfterAwaitingInputs(
	ctx context.Context, opts *invokeOptions) error {
	return o(ctx, nil, opts)
}

// merging is handled by each functional options call
// properties that are arrays/maps are always appended/merged together
// last value wins for non-array/map values and for conflicting map values (bool, struct, etc)
// if any options contain Inputs, these are awaited here, hence the need for a Context
func mergeAwait(ctx context.Context, opts ...ResourceOption) (*resourceOptions, error) {
	options := &resourceOptions{}
	for _, o := range opts {
		if o != nil {
			err := o.applyResourceOptionAfterAwaitingInputs(ctx, options)
			if err != nil {
				return nil, err
			}
		}
	}
	return options, nil
}

// Version of merge that only succeeds if none of the options contain inputs, but never awaits.
func tryMergeWithoutAwaiting(opts ...ResourceOption) (*resourceOptions, error) {
	options := &resourceOptions{}
	for _, o := range opts {
		err := o.applyResourceOptionImmediately(options)
		if err != nil {
			return nil, err
		}
	}
	return options, nil
}

// AdditionalSecretOutputs specifies a list of output properties to mark as secret.
func AdditionalSecretOutputs(o []string) ResourceOption {
	return resourceOption(func(ro *resourceOptions) {
		ro.AdditionalSecretOutputs = append(ro.AdditionalSecretOutputs, o...)
	})
}

// Aliases applies a list of identifiers to find and use existing resources.
func Aliases(o []Alias) ResourceOption {
	return resourceOption(func(ro *resourceOptions) {
		ro.Aliases = append(ro.Aliases, o...)
	})
}

// DeleteBeforeReplace, when set to true, ensures that this resource is deleted prior to replacement.
func DeleteBeforeReplace(o bool) ResourceOption {
	return resourceOption(func(ro *resourceOptions) {
		ro.DeleteBeforeReplace = o
	})
}

// DependsOn is an optional array of explicit dependencies on other resources.
func DependsOn(o []Resource) ResourceOption {
	return resourceOption(func(ro *resourceOptions) {
		ro.DependsOn = append(ro.DependsOn, o...)
	})
}

// Like DependsOn, but accepts ResourceInput and ResourceOutput.
func DependsOnInputs(o []ResourceInput) ResourceOption {
	return resourceOptionWithInputs(func(ctx context.Context, opts *resourceOptions) error {
		var allDeps []Resource
		for _, ri := range o {
			if ri == nil {
				continue
			}
			dep, moreDeps, err := awaitResourceInput(ctx, ri)
			if err != nil {
				return err
			}
			allDeps = append(allDeps, dep)
			allDeps = append(allDeps, moreDeps...)
		}
		opts.DependsOn = append(opts.DependsOn, allDeps...)
		return nil
	})
}

// Ignore changes to any of the specified properties.
func IgnoreChanges(o []string) ResourceOption {
	return resourceOption(func(ro *resourceOptions) {
		ro.IgnoreChanges = append(ro.IgnoreChanges, o...)
	})
}

// Import, when provided with a resource ID, indicates that this resource's provider should import its state from
// the cloud resource with the given ID. The inputs to the resource's constructor must align with the resource's
// current state. Once a resource has been imported, the import property must be removed from the resource's
// options.
func Import(o IDInput) ResourceOption {
	return resourceOption(func(ro *resourceOptions) {
		ro.Import = o
	})
}

// Parent sets the parent resource to which this resource or invoke belongs.
func Parent(r Resource) ResourceOrInvokeOption {
	return resourceOrInvokeOption(func(ro *resourceOptions, io *invokeOptions) {
		switch {
		case ro != nil:
			ro.Parent = r
		case io != nil:
			io.Parent = r
		}
	})
}

// Like Parent, but accepts ResourceInput and ResourceOutput.
func ParentInput(r ResourceInput) ResourceOrInvokeOption {
	return resourceOrInvokeOptionWithInputs(func(ctx context.Context, ro *resourceOptions, io *invokeOptions) error {
		if r == nil {
			return nil
		}

		parent, deps, err := awaitResourceInput(ctx, r)
		if err != nil {
			return err
		}

		switch {
		case ro != nil:
			ro.Parent = parent
			ro.DependsOn = append(ro.DependsOn, deps...)
		case io != nil:
			io.Parent = parent
		}

		return nil
	})
}

// Protect, when set to true, ensures that this resource cannot be deleted (without first setting it to false).
func Protect(o bool) ResourceOption {
	return resourceOption(func(ro *resourceOptions) {
		ro.Protect = o
	})
}

// Provider sets the provider resource to use for a resource's CRUD operations or an invoke's call.
func Provider(r ProviderResource) ResourceOrInvokeOption {
	return resourceOrInvokeOption(func(ro *resourceOptions, io *invokeOptions) {
		switch {
		case ro != nil:
			err := Providers(r).applyResourceOptionImmediately(ro)
			if err != nil {
				panic(err)
			}
		case io != nil:
			io.Provider = r
		}
	})
}

// Similar to Provider, but accepts ProviderResourceInput or ProviderResourceOutput.
func ProviderInput(pri ProviderResourceInput) ResourceOrInvokeOption {
	return resourceOrInvokeOptionWithInputs(func(
		ctx context.Context,
		ro *resourceOptions,
		io *invokeOptions) error {

		if pri == nil {
			return nil
		}

		p, deps, err := awaitProviderResourceInput(ctx, pri.ToProviderResourceOutput())
		if err != nil {
			return err
		}

		switch {
		case io != nil:
			io.Provider = p
		case ro != nil:
			ro.Provider = p
			ro.DependsOn = append(ro.DependsOn, deps...)
		}

		return nil
	})
}

// ProviderMap is an optional map of package to provider resource for a component resource.
func ProviderMap(o map[string]ProviderResource) ResourceOption {
	return resourceOption(func(ro *resourceOptions) {
		if o != nil {
			if ro.Providers == nil {
				ro.Providers = make(map[string]ProviderResource)
			}
			for k, v := range o {
				ro.Providers[k] = v
			}
		}
	})
}

// Similar to ProviderMap, but accepts ProviderResourceInput instead of ProviderResource.
func ProviderInputMap(inputMap map[string]ProviderResourceInput) ResourceOption {
	return resourceOptionWithInputs(func(ctx context.Context, opts *resourceOptions) error {
		resultMap := make(map[string]ProviderResource)
		var allDeps []Resource
		for k, v := range inputMap {
			if v == nil {
				continue
			}
			r, deps, err := awaitProviderResourceInput(ctx, v)
			if err != nil {
				return err
			}
			allDeps = append(allDeps, deps...)
			resultMap[k] = r
		}
		opts.DependsOn = append(opts.DependsOn, allDeps...)
		return ProviderMap(resultMap).applyResourceOptionImmediately(opts)
	})
}

// Providers is an optional list of providers to use for a resource's children.
func Providers(o ...ProviderResource) ResourceOption {
	m := map[string]ProviderResource{}
	for _, p := range o {
		m[p.getPackage()] = p
	}
	return ProviderMap(m)
}

// Like Providers, but accepts ProviderResourceInput.
func ProviderInputs(o ...ProviderResourceInput) ResourceOption {
	return resourceOptionWithInputs(func(ctx context.Context, opts *resourceOptions) error {
		var results []ProviderResource
		var allDeps []Resource
		for _, v := range o {
			if v == nil {
				continue
			}
			r, deps, err := awaitProviderResourceInput(ctx, v)
			if err != nil {
				return err
			}
			allDeps = append(allDeps, deps...)
			results = append(results, r)
		}
		opts.DependsOn = append(opts.DependsOn, allDeps...)
		return Providers(results...).applyResourceOptionImmediately(opts)
	})
}

// ReplaceOnChanges will force a replacement when any of these property paths are set.  If this list includes `"*"`,
// changes to any properties will force a replacement.  Initialization errors from previous deployments will
// require replacement instead of update only if `"*"` is passed.
func ReplaceOnChanges(o []string) ResourceOption {
	return resourceOption(func(ro *resourceOptions) {
		ro.ReplaceOnChanges = append(ro.ReplaceOnChanges, o...)
	})
}

// Timeouts is an optional configuration block used for CRUD operations
func Timeouts(o *CustomTimeouts) ResourceOption {
	return resourceOption(func(ro *resourceOptions) {
		ro.CustomTimeouts = o
	})
}

// Transformations is an optional list of transformations to be applied to the resource.
func Transformations(o []ResourceTransformation) ResourceOption {
	return resourceOption(func(ro *resourceOptions) {
		ro.Transformations = append(ro.Transformations, o...)
	})
}

// URN_ is an optional URN of a previously-registered resource of this type to read from the engine.
//nolint: golint
func URN_(o string) ResourceOption {
	return resourceOption(func(ro *resourceOptions) {
		ro.URN = o
	})
}

// Version is an optional version, corresponding to the version of the provider plugin that should be used when
// operating on this resource. This version overrides the version information inferred from the current package and
// should rarely be used.
func Version(o string) ResourceOrInvokeOption {
	return resourceOrInvokeOption(func(ro *resourceOptions, io *invokeOptions) {
		switch {
		case ro != nil:
			ro.Version = o
		case io != nil:
			io.Version = o
		}
	})
}
