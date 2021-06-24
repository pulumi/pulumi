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
	urn URNOutput `pulumi:"urn"`

	providers map[string]ProviderResource

	aliases []URNOutput

	name string

	transformations []ResourceTransformation
}

func (s ResourceState) URN() URNOutput {
	return s.urn
}

func (s ResourceState) GetProvider(token string) ProviderResource {
	return s.providers[getPackage(token)]
}

func (s ResourceState) getProviders() map[string]ProviderResource {
	return s.providers
}

func (s ResourceState) getAliases() []URNOutput {
	return s.aliases
}

func (s ResourceState) getName() string {
	return s.name
}

func (s ResourceState) getTransformations() []ResourceTransformation {
	return s.transformations
}

func (s *ResourceState) addTransformation(t ResourceTransformation) {
	s.transformations = append(s.transformations, t)
}

func (ResourceState) isResource() {}

func (ctx *Context) newDependencyResource(urn URN) Resource {
	var res ResourceState
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
	res.urn.OutputState = ctx.newOutputState(res.urn.ElementType(), &res)
	res.id.OutputState = ctx.newOutputState(res.id.ElementType(), &res)
	res.urn.resolve(urn, true, false, nil)
	res.id.resolve(id, id != "", false, nil)
	res.pkg = string(resource.URN(urn).Type().Name())
	return &res
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

type invokeOptions struct {
	// Parent is an optional parent resource to use for default provider options for this invoke.
	Parent Resource
	// Provider is an optional provider resource to use for this invoke.
	Provider ProviderResource
	// Version is an optional version of the provider plugin to use for the invoke.
	Version string
}

type ResourceOption interface {
	applyResourceOption(context.Context, *resourceOptions) error
}

type InvokeOption interface {
	applyInvokeOption(context.Context, *invokeOptions) error
}

type ResourceOrInvokeOption interface {
	ResourceOption
	InvokeOption
}

type resourceOption func(context.Context, *resourceOptions) error

func (o resourceOption) applyResourceOption(ctx context.Context, opts *resourceOptions) error {
	return o(ctx, opts)
}

type resourceOrInvokeOption func(ctx context.Context, ro *resourceOptions, io *invokeOptions) error

func (o resourceOrInvokeOption) applyResourceOption(ctx context.Context, opts *resourceOptions) error {
	return o(ctx, opts, nil)
}

func (o resourceOrInvokeOption) applyInvokeOption(ctx context.Context, opts *invokeOptions) error {
	return o(ctx, nil, opts)
}

// merging is handled by each functional options call
// properties that are arrays/maps are always appended/merged together
// last value wins for non-array/map values and for conflicting map values (bool, struct, etc)
func merge(ctx context.Context, opts ...ResourceOption) (*resourceOptions, error) {
	options := &resourceOptions{}
	for _, o := range opts {
		if o != nil {
			err := o.applyResourceOption(ctx, options)
			if err != nil {
				return nil, err
			}
		}
	}
	return options, nil
}

// AdditionalSecretOutputs specifies a list of output properties to mark as secret.
func AdditionalSecretOutputs(o []string) ResourceOption {
	return resourceOption(func(ctx context.Context, ro *resourceOptions) error {
		ro.AdditionalSecretOutputs = append(ro.AdditionalSecretOutputs, o...)
		return nil
	})
}

// Aliases applies a list of identifiers to find and use existing resources.
func Aliases(o []Alias) ResourceOption {
	return resourceOption(func(ctx context.Context, ro *resourceOptions) error {
		ro.Aliases = append(ro.Aliases, o...)
		return nil
	})
}

// DeleteBeforeReplace, when set to true, ensures that this resource is deleted prior to replacement.
func DeleteBeforeReplace(o bool) ResourceOption {
	return resourceOption(func(ctx context.Context, ro *resourceOptions) error {
		ro.DeleteBeforeReplace = o
		return nil
	})
}

// DependsOn is an optional array of explicit dependencies on other resources.
func DependsOn(o []Resource) ResourceOption {
	return resourceOption(func(ctx context.Context, ro *resourceOptions) error {
		ro.DependsOn = append(ro.DependsOn, o...)
		return nil
	})
}

// Like DependsOn, but accepts ResourceInptu and ResourceOutput.
func DependsOnInputs(o []ResourceInput) ResourceOption {
	return deferResourceOption(func(ctx context.Context) (ResourceOption, error) {
		// Similarly to ParentInput, we force-await any
		// ResourceOutputs passed in right here, instead of
		// trying to lazily await them as needed downstream.

		var allDeps []Resource

		for _, ri := range o {
			dep, moreDeps, err := awaitResourceInputMAGIC(ctx, ri)
			if err != nil {
				return nil, err
			}
			allDeps = append(allDeps, dep)
			allDeps = append(allDeps, moreDeps...)
		}

		return DependsOn(allDeps), nil
	})
}

// Ignore changes to any of the specified properties.
func IgnoreChanges(o []string) ResourceOption {
	return resourceOption(func(ctx context.Context, ro *resourceOptions) error {
		ro.IgnoreChanges = append(ro.IgnoreChanges, o...)
		return nil
	})
}

// Import, when provided with a resource ID, indicates that this resource's provider should import its state from
// the cloud resource with the given ID. The inputs to the resource's constructor must align with the resource's
// current state. Once a resource has been imported, the import property must be removed from the resource's
// options.
func Import(o IDInput) ResourceOption {
	return resourceOption(func(ctx context.Context, ro *resourceOptions) error {
		ro.Import = o
		return nil
	})
}

// Parent sets the parent resource to which this resource or invoke belongs.
func Parent(r Resource) ResourceOrInvokeOption {
	return resourceOrInvokeOption(func(ctx context.Context, ro *resourceOptions, io *invokeOptions) error {
		switch {
		case ro != nil:
			ro.Parent = r
		case io != nil:
			io.Parent = r
		}
		return nil
	})
}

// Like Parent, but accepts ResourceInput and ResourceOutput.
func ParentInput(r ResourceInput) ResourceOrInvokeOption {
	return deferResourceOrInvokeOption(func(ctx context.Context) (ResourceOrInvokeOption, error) {
		// The Parent option is accessed a lot as it figures
		// in URN construction for example, so it is
		// reasonable to force-await it right away instead of
		// trying to defer lazily.
		p, deps, err := awaitResourceInputMAGIC(ctx, r)

		if err != nil {
			return nil, err
		}

		return combineResourceOrInvokeOptions(Parent(p), orInvokeOption(DependsOn(deps))), nil
	})
}

// Protect, when set to true, ensures that this resource cannot be deleted (without first setting it to false).
func Protect(o bool) ResourceOption {
	return resourceOption(func(ctx context.Context, ro *resourceOptions) error {
		ro.Protect = o
		return nil
	})
}

// Provider sets the provider resource to use for a resource's CRUD operations or an invoke's call.
func Provider(r ProviderResource) ResourceOrInvokeOption {
	return resourceOrInvokeOption(func(ctx context.Context, ro *resourceOptions, io *invokeOptions) error {
		switch {
		case ro != nil:
			return Providers(r).applyResourceOption(ctx, ro)
		case io != nil:
			io.Provider = r
		}
		return nil
	})
}

// Similar to Provider, but accepts ProviderResourceInput or ProviderResourceOutput.
func ProviderInput(pri ProviderResourceInput) ResourceOrInvokeOption {
	return deferResourceOrInvokeOption(func(ctx context.Context) (ResourceOrInvokeOption, error) {
		// Less obvious than ParentInput that we should
		// force-await here, but doing so for simplicity of
		// implementation.
		p, deps, err := awaitProviderResourceOutput(ctx, pri.ToProviderResourceOutput())
		if err != nil {
			return nil, err
		}

		return combineResourceOrInvokeOptions(Provider(p), orInvokeOption(DependsOn(deps))), nil
	})
}

// ProviderMap is an optional map of package to provider resource for a component resource.
func ProviderMap(o map[string]ProviderResource) ResourceOption {
	return resourceOption(func(ctx context.Context, ro *resourceOptions) error {
		if o != nil {
			if ro.Providers == nil {
				ro.Providers = make(map[string]ProviderResource)
			}
			for k, v := range o {
				ro.Providers[k] = v
			}
		}
		return nil
	})
}

// Similar to ProviderMap, but accepts ProviderResourceInput instead of ProviderResource.
func ProviderInputMap(inputMap map[string]ProviderResourceInput) ResourceOption {
	return deferResourceOption(func(ctx context.Context) (ResourceOption, error) {
		var allDeps []Resource

		resourceMap := make(map[string]ProviderResource)
		for k, v := range inputMap {
			pr, deps, err := awaitProviderResourceOutput(ctx, v.ToProviderResourceOutput())
			if err != nil {
				return nil, err
			}
			allDeps = append(allDeps, deps...)
			resourceMap[k] = pr
		}

		return combineResourceOptions(ProviderMap(resourceMap), DependsOn(allDeps)), nil
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
	return deferResourceOption(func(ctx context.Context) (ResourceOption, error) {
		var ps []ProviderResource
		var allDeps []Resource

		for _, pri := range o {
			p, deps, err := awaitProviderResourceOutput(ctx, pri.ToProviderResourceOutput())
			if err != nil {
				return nil, err
			}
			ps = append(ps, p)
			allDeps = append(allDeps, deps...)
		}

		return combineResourceOptions(Providers(ps...), DependsOn(allDeps)), nil
	})
}

// Timeouts is an optional configuration block used for CRUD operations
func Timeouts(o *CustomTimeouts) ResourceOption {
	return resourceOption(func(ctx context.Context, ro *resourceOptions) error {
		ro.CustomTimeouts = o
		return nil
	})
}

// Transformations is an optional list of transformations to be applied to the resource.
func Transformations(o []ResourceTransformation) ResourceOption {
	return resourceOption(func(ctx context.Context, ro *resourceOptions) error {
		ro.Transformations = append(ro.Transformations, o...)
		return nil
	})
}

// URN_ is an optional URN of a previously-registered resource of this type to read from the engine.
//nolint: golint
func URN_(o string) ResourceOption {
	return resourceOption(func(ctx context.Context, ro *resourceOptions) error {
		ro.URN = o
		return nil
	})
}

// Version is an optional version, corresponding to the version of the provider plugin that should be used when
// operating on this resource. This version overrides the version information inferred from the current package and
// should rarely be used.
func Version(o string) ResourceOrInvokeOption {
	return resourceOrInvokeOption(func(ctx context.Context, ro *resourceOptions, io *invokeOptions) error {
		switch {
		case ro != nil:
			ro.Version = o
		case io != nil:
			io.Version = o
		}
		return nil
	})
}

// TODO - helps incremental refactoring, but can we completely remove
// this function?
func awaitResourceInputMAGIC(ctx context.Context, ri ResourceInput) (Resource, []Resource, error) {
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

func awaitProviderResourceOutput(ctx context.Context, pri ProviderResourceInput) (ProviderResource, []Resource, error) {
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

// Trivially ResourceOption is also a ResourceOrInvokeOption.
func orInvokeOption(opt ResourceOption) ResourceOrInvokeOption {
	return resourceOrInvokeOption(func(ctx context.Context, ro *resourceOptions, io *invokeOptions) error {
		if ro != nil {
			return opt.applyResourceOption(ctx, ro)
		}
		return nil
	})
}

// The combined option will apply all the given options in order.
func combineResourceOptions(options ...ResourceOption) ResourceOption {
	return resourceOption(func(ctx context.Context, ro *resourceOptions) error {
		for _, opt := range options {
			err := opt.applyResourceOption(ctx, ro)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// The combined option will apply all the given options in order.
func combineResourceOrInvokeOptions(options ...ResourceOrInvokeOption) ResourceOrInvokeOption {
	return resourceOrInvokeOption(func(ctx context.Context, ro *resourceOptions, io *invokeOptions) error {
		switch {
		case ro != nil:
			for _, opt := range options {
				err := opt.applyResourceOption(ctx, ro)
				if err != nil {
					return err
				}
			}
		case io != nil:
			for _, opt := range options {
				err := opt.applyInvokeOption(ctx, io)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// Defers an init action until the option is applied.
func deferResourceOption(f func(ctx context.Context) (ResourceOption, error)) ResourceOption {
	return resourceOption(func(ctx context.Context, ro *resourceOptions) error {
		opt, err := f(ctx)
		if err != nil {
			return err
		}
		return opt.applyResourceOption(ctx, ro)
	})
}

// Defers an init action until the option is applied.
func deferResourceOrInvokeOption(f func(ctx context.Context) (ResourceOrInvokeOption, error)) ResourceOrInvokeOption {
	return resourceOrInvokeOption(func(ctx context.Context, ro *resourceOptions, io *invokeOptions) error {
		opt, err := f(ctx)
		if err != nil {
			return err
		}
		switch {
		case ro != nil:
			return opt.applyResourceOption(ctx, ro)
		case io != nil:
			return opt.applyInvokeOption(ctx, io)
		}
		return nil
	})
}
