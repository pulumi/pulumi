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
	"sort"
	"strings"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/internal"
)

type (
	// ID is a unique identifier assigned by a resource provider to a resource.
	ID string
	// URN is an automatically generated logical URN, used to stably identify resources.
	URN string
)

var (
	resourceStateType         = reflect.TypeOf(ResourceState{})
	customResourceStateType   = reflect.TypeOf(CustomResourceState{})
	providerResourceStateType = reflect.TypeOf(ProviderResourceState{})
)

// This type alias is a hack to embed the internal.ResourceState type
// into pulumi.ResourceState without exporting the field to the public API.
//
//nolint:unused
type internalResourceState = internal.ResourceState

// ResourceState is the base
type ResourceState struct {
	// internalResourceState marks this ResourceState as a resource
	// recognized by the internal package.
	internalResourceState

	m sync.RWMutex

	urn URNOutput `pulumi:"urn"`

	rawOutputs        Output
	children          resourceSet
	providers         map[string]ProviderResource
	provider          ProviderResource
	protect           bool
	version           string
	pluginDownloadURL string
	aliases           []URNOutput
	name              string
	transformations   []ResourceTransformation

	keepDep bool
}

var _ Resource = (*ResourceState)(nil)

func (s *ResourceState) URN() URNOutput {
	return s.urn
}

func (s *ResourceState) GetProvider(token string) ProviderResource {
	return s.providers[getPackage(token)]
}

// This is an internal method and future versions of the sdk may not support this API.
//
// InternalGetRawOutputs obtains the full PropertyMap returned during resource registration,
// allowing a caller of RegisterResource to obtain directly information about the outputs and their
// known and secret attributes.
func InternalGetRawOutputs(res *ResourceState) Output {
	return res.rawOutputs
}

func (s *ResourceState) getChildren() []Resource {
	s.m.RLock()
	defer s.m.RUnlock()

	var children []Resource
	if len(s.children) != 0 {
		children = slice.Prealloc[Resource](len(s.children))
		for r := range s.children {
			children = append(children, r)
		}
	}
	return children
}

func (s *ResourceState) addChild(r Resource) {
	s.m.Lock()
	defer s.m.Unlock()

	if s.children == nil {
		s.children = resourceSet{}
	}
	s.children.add(r)
}

func (s *ResourceState) getProviders() map[string]ProviderResource {
	return s.providers
}

func (s *ResourceState) getProvider() ProviderResource {
	return s.provider
}

func (s *ResourceState) getProtect() bool {
	return s.protect
}

func (s *ResourceState) getVersion() string {
	return s.version
}

func (s *ResourceState) getPluginDownloadURL() string {
	return s.pluginDownloadURL
}

func (s *ResourceState) getAliases() []URNOutput {
	return s.aliases
}

func (s *ResourceState) getName() string {
	return s.name
}

func (s *ResourceState) getTransformations() []ResourceTransformation {
	return s.transformations
}

func (s *ResourceState) addTransformation(t ResourceTransformation) {
	s.transformations = append(s.transformations, t)
}

func (s *ResourceState) setKeepDependency() {
	s.keepDep = true
}

func (s *ResourceState) keepDependency() bool {
	return s.keepDep
}

func (ctx *Context) newDependencyResource(urn URN) Resource {
	var res ResourceState
	res.urn.OutputState = ctx.newOutputState(res.urn.ElementType(), &res)
	internal.ResolveOutput(res.urn, urn, true, false, resourcesToInternal(nil))
	res.keepDep = true
	return &res
}

type CustomResourceState struct {
	ResourceState

	id IDOutput `pulumi:"id"`
}

func (s *CustomResourceState) ID() IDOutput {
	return s.id
}

func (*CustomResourceState) isCustomResource() {}

func (ctx *Context) newDependencyCustomResource(urn URN, id ID) CustomResource {
	var res CustomResourceState
	res.urn.OutputState = ctx.newOutputState(res.urn.ElementType(), &res)
	internal.ResolveOutput(res.urn, urn, true, false, resourcesToInternal(nil))
	res.id.OutputState = ctx.newOutputState(res.id.ElementType(), &res)
	internal.ResolveOutput(res.id, id, id != "", false, resourcesToInternal(nil))
	return &res
}

type ProviderResourceState struct {
	CustomResourceState

	pkg string
}

func (s *ProviderResourceState) getPackage() string {
	return s.pkg
}

func (ctx *Context) newDependencyProviderResource(urn URN, id ID) ProviderResource {
	var res ProviderResourceState
	res.urn.OutputState = ctx.newOutputState(res.urn.ElementType(), &res)
	res.id.OutputState = ctx.newOutputState(res.id.ElementType(), &res)
	internal.ResolveOutput(res.urn, urn, true, false, resourcesToInternal(nil))
	internal.ResolveOutput(res.id, id, id != "", false, resourcesToInternal(nil))
	res.pkg = string(resource.URN(urn).Type().Name())
	return &res
}

func (ctx *Context) newDependencyProviderResourceFromRef(ref string) ProviderResource {
	idx := strings.LastIndex(ref, "::")
	if idx == -1 {
		return nil
	}
	urn, id := ref[:idx], ref[idx+2:]
	return ctx.newDependencyProviderResource(URN(urn), ID(id))
}

// Resource represents a cloud resource managed by Pulumi.
type Resource interface {
	internal.Resource

	// URN is this resource's stable logical URN used to distinctly address it before, during, and after deployments.
	URN() URNOutput

	// getChildren returns the resource's children.
	getChildren() []Resource

	// addChild adds a child to the resource.
	addChild(r Resource)

	// getProviders returns the provider map for this resource.
	getProviders() map[string]ProviderResource

	// getProvider returns the provider for the resource.
	getProvider() ProviderResource

	// getProtect returns the protect flag for the resource.
	getProtect() bool

	// getVersion returns the version for the resource.
	getVersion() string

	// getPluginDownloadURL returns the provider plugin download url
	getPluginDownloadURL() string

	// getAliases returns the list of aliases for this resource
	getAliases() []URNOutput

	// getName returns the name of the resource
	getName() string

	// getTransformations returns the transformations for the resource.
	getTransformations() []ResourceTransformation

	// addTransformation adds a single transformation to the resource.
	addTransformation(t ResourceTransformation)

	// setKeepDependency marks this resource as a resource that should be kept as a dependency.
	// This is done for remote component resources, dependency resources, and rehydrated component resources.
	setKeepDependency()

	// keepDependency returns true if the resource should be kept as a dependency, which is the case for
	// remote component resources, dependency resources, and rehydrated component resources.
	keepDependency() bool
}

var _ internal.Resource = (Resource)(nil)

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

// CustomTimeouts specifies timeouts for resource provisioning operations.
// Use it with the [Timeouts] option when creating new resources
// to override default timeouts.
//
// Each timeout is specified as a duration string such as,
// "5ms" (5 milliseconds), "40s" (40 seconds),
// and "1m30s" (1 minute, 30 seconds).
//
// The following units are accepted.
//
//   - ns: nanoseconds
//   - us: microseconds
//   - µs: microseconds
//   - ms: milliseconds
//   - s: seconds
//   - m: minutes
//   - h: hours
type CustomTimeouts struct {
	Create string
	Update string
	Delete string
}

// ResourceOptions is a snapshot of one or more [ResourceOption]s.
//
// You cannot pass a ResourceOptions struct to a resource constructor.
// Instead, use individual [ResourceOption] values to configure a resource.
// The ResourceOptions struct only provides a read-only preview
// of the collective effect of the options.
//
// See https://www.pulumi.com/docs/intro/concepts/resources/options/
// for more details on individual options.
type ResourceOptions struct {
	// AdditionalSecretOutputs lists output properties
	// that must be encrypted as secrets.
	AdditionalSecretOutputs []string

	// Aliases lists aliases for this resource
	// that are used to find and use existing resources.
	Aliases []Alias

	// CustomTimeouts, if set, overrides the default timeouts
	// for resource CRUD operations.
	CustomTimeouts *CustomTimeouts

	// DeleteBeforeReplace specifies that resources being replaced
	// should be deleted before creating the replacement
	// instead of Pulumi's default behavior of creating the replacement
	// before performing deletion.
	DeleteBeforeReplace bool

	// DependsOn lists additional explicit dependencies for the resource
	// in addition to those tracked automatically by Pulumi.
	DependsOn []Resource

	// DependsOnInputs holds explicit dependencies for the resource
	// that may not be fully known yet.
	DependsOnInputs []ResourceArrayInput

	// IgnoreChanges lists properties changes to which should be ignored.
	IgnoreChanges []string

	// Import specifies that the provider for this resource
	// should import its state from a cloud resource with the given ID.
	Import IDInput

	// Parent is the parent resource for the resource being created,
	// or nil if this resource does not have a parent.
	Parent Resource

	// Protect prevents this resource from being deleted.
	Protect bool

	// Provider is the provider resource to use for this resource's CRUD operations.
	// It's nil if the default provider should be used.
	Provider ProviderResource

	// Providers is a bag of providers available
	// to instantiate resources of various types.
	// These are used for a type when a provider for that type
	// was not explicitly supplied.
	Providers []ProviderResource

	// ReplaceOnChanges lists properties that, when modified,
	// force a replacement of the resource.
	// The list may include '*' to indicate that all properties trigger
	// replacements.
	ReplaceOnChanges []string

	// Transformations is a list of functions that transform
	// the resource's properties during construction.
	Transformations []ResourceTransformation

	// Transforms is a list of functions that transform
	// the resource's properties during construction.
	Transforms []ResourceTransform

	// URN is the URN of a previously-registered resource of this type.
	URN string

	// Version changes the version of the provider plugin that should be used
	// when operating on this resource.
	// This will be blank if the version was automatically inferred.
	Version string

	// PluginDownloadURL specifies the URL from which the provider plugin
	// should be downloaded.
	// This will be blank if the URL was inferred automatically.
	PluginDownloadURL string

	// RetainOnDelete specifies that the resource should not be deleted
	// in the cloud provider, even if it's deleted from Pulumi.
	RetainOnDelete bool

	// DeletedWith holds a container resource that, if deleted,
	// also deletes this resource.
	DeletedWith Resource
}

// NewResourceOptions builds a preview of the effect of the provided options.
//
// Use this to get a read-only snapshot of a list of options
// inside mocks and component resources.
func NewResourceOptions(opts ...ResourceOption) (*ResourceOptions, error) {
	// The error return is currently unused,
	// but it's foreseeable that we'll need it
	// if we begin doing option validation at option merge time.
	return resourceOptionsSnapshot(merge(opts...)), nil
}

// resourceOptions is the internal representation of the effect of
// [ResourceOption]s.
type resourceOptions struct {
	AdditionalSecretOutputs []string
	Aliases                 []Alias
	CustomTimeouts          *CustomTimeouts
	DeleteBeforeReplace     bool
	DependsOn               []dependencySet
	IgnoreChanges           []string
	Import                  IDInput
	Parent                  Resource
	Protect                 bool
	Provider                ProviderResource
	Providers               map[string]ProviderResource
	ReplaceOnChanges        []string
	Transformations         []ResourceTransformation
	Transforms              []ResourceTransform
	URN                     string
	Version                 string
	PluginDownloadURL       string
	RetainOnDelete          bool
	DeletedWith             Resource
	Parameterization        []byte
}

func resourceOptionsSnapshot(ro *resourceOptions) *ResourceOptions {
	var (
		dependsOn       []Resource
		dependsOnInputs []ResourceArrayInput
	)
	for _, d := range ro.DependsOn {
		switch d := d.(type) {
		case resourceDependencySet:
			dependsOn = append(dependsOn, []Resource(d)...)
		case *resourceArrayInputDependencySet:
			dependsOnInputs = append(dependsOnInputs, d.input)
		default:
			// Unreachable.
			// We control all implementations of dependencySet.
			contract.Failf("Unknown dependencySet %T", d)
		}
	}

	sort.Slice(dependsOn, func(i, j int) bool {
		return dependsOn[i].getName() < dependsOn[j].getName()
	})

	var providers []ProviderResource
	if len(ro.Providers) > 0 {
		providers = slice.Prealloc[ProviderResource](len(ro.Providers))
		for _, p := range ro.Providers {
			providers = append(providers, p)
		}
		sort.Slice(providers, func(i, j int) bool {
			return providers[i].getPackage() < providers[j].getPackage()
		})
	}

	return &ResourceOptions{
		AdditionalSecretOutputs: ro.AdditionalSecretOutputs,
		Aliases:                 ro.Aliases,
		CustomTimeouts:          ro.CustomTimeouts,
		DeleteBeforeReplace:     ro.DeleteBeforeReplace,
		DependsOn:               dependsOn,
		DependsOnInputs:         dependsOnInputs,
		IgnoreChanges:           ro.IgnoreChanges,
		Import:                  ro.Import,
		Parent:                  ro.Parent,
		Protect:                 ro.Protect,
		Provider:                ro.Provider,
		Providers:               providers,
		ReplaceOnChanges:        ro.ReplaceOnChanges,
		Transformations:         ro.Transformations,
		Transforms:              ro.Transforms,
		URN:                     ro.URN,
		Version:                 ro.Version,
		PluginDownloadURL:       ro.PluginDownloadURL,
		RetainOnDelete:          ro.RetainOnDelete,
		DeletedWith:             ro.DeletedWith,
	}
}

// InvokeOptions is a snapshot of one or more [InvokeOption]s.
//
// You cannot pass an InvokeOptions struct to a provider function.
// Instead, use individual [InvokeOption] values to configure a call.
// The InvokeOptions struct only provides a read-only preview
// of the collective effect of the options.
type InvokeOptions struct {
	// Parent is the parent resource for this operation.
	// It may be used to determine the provider to use.
	Parent Resource
	// Provider specifies the provider to use for this operation.
	// This is nil if the default provider should be used.
	Provider ProviderResource
	// Version is the version of the provider plugin that should be used.
	// This will be blank if the version was automatically inferred.
	Version string
	// PluginDownloadURL is the URL from which the provider plugin
	// should be downloaded.
	// This will be blank if the URL was inferred automatically.
	PluginDownloadURL string
	// DependsOn lists additional explicit dependencies for the resource
	// in addition to those tracked automatically by Pulumi.
	DependsOn []Resource
	// DependsOnInputs holds explicit dependencies for the resource
	// that may not be fully known yet.
	DependsOnInputs []ResourceArrayInput
}

// NewInvokeOptions builds a preview of the effect of the provided options.
//
// Use this to get a read-only snapshot of the collective effect
// of a list of [InvokeOption]s.
func NewInvokeOptions(opts ...InvokeOption) (*InvokeOptions, error) {
	// The error return is currently unused,
	// but it's foreseeable that we'll need it
	// if we begin doing option validation at option merge time.
	return invokeOptionsSnapshot(mergeInvokeOptions(opts...)), nil
}

// invokeOptions is the internal representation of the effect of
// [InvokeOptions]s.
type invokeOptions struct {
	Parent            Resource
	Provider          ProviderResource
	Version           string
	PluginDownloadURL string
	DependsOn         []dependencySet
	Parameterization  []byte
}

func invokeOptionsSnapshot(io *invokeOptions) *InvokeOptions {
	var (
		dependsOn       []Resource
		dependsOnInputs []ResourceArrayInput
	)
	for _, d := range io.DependsOn {
		switch d := d.(type) {
		case resourceDependencySet:
			dependsOn = append(dependsOn, []Resource(d)...)
		case *resourceArrayInputDependencySet:
			dependsOnInputs = append(dependsOnInputs, d.input)
		default:
			// Unreachable.
			// We control all implementations of dependencySet.
			contract.Failf("Unknown dependencySet %T", d)
		}
	}

	sort.Slice(dependsOn, func(i, j int) bool {
		return dependsOn[i].getName() < dependsOn[j].getName()
	})

	return &InvokeOptions{
		Parent:            io.Parent,
		Provider:          io.Provider,
		Version:           io.Version,
		PluginDownloadURL: io.PluginDownloadURL,
		DependsOn:         dependsOn,
		DependsOnInputs:   dependsOnInputs,
	}
}

type ResourceOption interface {
	applyResourceOption(*resourceOptions)
}

type InvokeOption interface {
	applyInvokeOption(*invokeOptions)
}

type ResourceOrInvokeOption interface {
	ResourceOption
	InvokeOption
}

type resourceOption func(*resourceOptions)

func (o resourceOption) applyResourceOption(opts *resourceOptions) {
	o(opts)
}

type invokeOption func(*invokeOptions)

func (o invokeOption) applyInvokeOption(opts *invokeOptions) {
	o(opts)
}

type resourceOrInvokeOption func(ro *resourceOptions, io *invokeOptions)

func (o resourceOrInvokeOption) applyResourceOption(opts *resourceOptions) {
	o(opts, nil)
}

func (o resourceOrInvokeOption) applyInvokeOption(opts *invokeOptions) {
	o(nil, opts)
}

// merging is handled by each functional options call
// properties that are arrays/maps are always appended/merged together
// last value wins for non-array/map values and for conflicting map values (bool, struct, etc)
func merge(opts ...ResourceOption) *resourceOptions {
	options := &resourceOptions{}
	for _, o := range opts {
		if o != nil {
			o.applyResourceOption(options)
		}
	}
	return options
}

func mergeInvokeOptions(opts ...InvokeOption) *invokeOptions {
	options := &invokeOptions{}
	for _, o := range opts {
		if o != nil {
			o.applyInvokeOption(options)
		}
	}
	return options
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

// Composite is a resource option that contains other resource options.
func Composite(opts ...ResourceOption) ResourceOption {
	return resourceOption(func(ro *resourceOptions) {
		for _, o := range opts {
			o.applyResourceOption(ro)
		}
	})
}

// CompositeInvoke is an invoke option that contains other invoke options.
func CompositeInvoke(opts ...InvokeOption) InvokeOption {
	return invokeOption(func(ro *invokeOptions) {
		for _, o := range opts {
			o.applyInvokeOption(ro)
		}
	})
}

// dependencySet unifies types that can provide dependencies for a
// resource.
type dependencySet interface {
	// Adds URNs for addURNs from this set
	// into the given urnSet.
	// Optionally pass the last Resource arg to short-circuit component
	// children cycles.
	addDeps(context.Context, depSet, Resource) error
}

// DependsOn is an optional array of explicit dependencies on other resources.
func DependsOn(o []Resource) ResourceOrInvokeOption {
	return resourceOrInvokeOption(func(ro *resourceOptions, io *invokeOptions) {
		switch {
		case ro != nil:
			ro.DependsOn = append(ro.DependsOn, resourceDependencySet(o))
		case io != nil:
			io.DependsOn = append(io.DependsOn, resourceDependencySet(o))
		}
	})
}

// resourceDependencySet is a dependencySet comprised of references to
// resources.
type resourceDependencySet []Resource

var _ dependencySet = (resourceDependencySet)(nil)

func (rs resourceDependencySet) addDeps(ctx context.Context, deps depSet, from Resource) error {
	for _, r := range rs {
		if err := addDependency(ctx, deps, r, from); err != nil {
			return err
		}
	}
	return nil
}

// Declares explicit dependencies on other resources. Similar to
// `DependsOn`, but also admits resource inputs and outputs:
//
//	var r Resource
//	var ri ResourceInput
//	var ro ResourceOutput
//	allDeps := NewResourceArrayOutput(NewResourceOutput(r), ri.ToResourceOutput(), ro)
//	DependsOnInputs(allDeps)
func DependsOnInputs(o ResourceArrayInput) ResourceOrInvokeOption {
	return resourceOrInvokeOption(func(ro *resourceOptions, io *invokeOptions) {
		switch {
		case ro != nil:
			ro.DependsOn = append(ro.DependsOn, &resourceArrayInputDependencySet{o})
		case io != nil:
			io.DependsOn = append(io.DependsOn, &resourceArrayInputDependencySet{o})
		}
	})
}

// resourceArrayInputDependencySet is a dependencySet built from
// collections of resources that are not yet known.
type resourceArrayInputDependencySet struct{ input ResourceArrayInput }

var _ dependencySet = (*resourceArrayInputDependencySet)(nil)

func (ra *resourceArrayInputDependencySet) addDeps(ctx context.Context, deps depSet, from Resource) error {
	out := ra.input.ToResourceArrayOutput()

	value, known, _ /* secret */, _ /* deps */, err := internal.AwaitOutput(ctx, out)
	if err != nil || !known {
		return err
	}

	resources, ok := value.([]Resource)
	if !ok {
		return fmt.Errorf("ResourceArrayInput resolved to a value of unexpected type %v, expected []Resource",
			reflect.TypeOf(value))
	}

	// For some reason, deps returned above are incorrect; instead:
	toplevelDeps := getOutputDeps(out)

	for _, r := range append(resources, toplevelDeps...) {
		if err := addDependency(ctx, deps, r, from); err != nil {
			return err
		}
	}
	return nil
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

// Protect, when set to true, ensures that this resource cannot be deleted (without first setting it to false).
func Protect(o bool) ResourceOption {
	return resourceOption(func(ro *resourceOptions) {
		ro.Protect = o
	})
}

////////////////////////////////////////////////////////////////////////
// NOTE(Provider and Providers)
//
// For Provider vs Providers, there's a bit of complexity.
//
// # Background
//
// First, here's the desired behavior across languages,
// as standardized in #8796:
//
// - Providers is a bag of providers available to a resource.
//   These are used by the resource and its children.
// - Provider passed to a resource indicates that that specific resource
//   MUST use this provider over all else,
//   and it should add it to the bag of providers for use by its children.
//
// One discrepancy from the above is that originally, for the Provider option,
// mismatch between the provider package and resource type was an error.
// However, this appears to have been relaxed over time.
// In such a case, the provider is added to the bag of providers
// and used by the children of the resource.
// This is necessary, for example, for resources to inherit a provider
// passed to a component resource via the Provider option.
//
// # Go-specific difference
//
// In other languages, the Provider and Providers options can be provided at
// most once per resource.
// In Go, because we use functional options, we allow multiple calls to
// the Provider and Providers options.
// This has allowed for the following to be equivalen in Go:
//
//	NewFoo(..., Provider(p1), Provider(p2), Provider(p3))
//	NewFoo(..., Providers(p1, p2, p3))
//
// To support this while still having the Provider option take precedence,
// we need to do the following:
//
// 1. All providers (whether passed with Provider or Providers)
//    are merged into a single map.
//    Last provider for a given package wins.
// 2. For the Provider option, we additionally track the last
//    passed value in a separate field.
//    If this provider handles the current resource,
//    it takes precedence over the map.
////////////////////////////////////////////////////////////////////////

// Provider sets the provider resource to use for a resource's CRUD operations or an invoke's call.
func Provider(r ProviderResource) ResourceOrInvokeOption {
	return resourceOrInvokeOption(func(ro *resourceOptions, io *invokeOptions) {
		switch {
		case ro != nil:
			ro.Provider = r
			Providers(r).applyResourceOption(ro)
		case io != nil:
			io.Provider = r
		}
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

// Providers is an optional list of providers to use for a resource's children.
func Providers(o ...ProviderResource) ResourceOption {
	m := map[string]ProviderResource{}
	for _, p := range o {
		m[p.getPackage()] = p
	}
	return ProviderMap(m)
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

// Transforms is an optional list of transforms to be applied to the resource.
func Transforms(o []ResourceTransform) ResourceOption {
	return resourceOption(func(ro *resourceOptions) {
		ro.Transforms = append(ro.Transforms, o...)
	})
}

// URN_ is an optional URN of a previously-registered resource of this type to read from the engine.
//
//nolint:revive
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

// PluginDownloadURL is an optional url, corresponding to the download url of the provider plugin
// that should be used when operating on this resource. This url overrides the url information
// inferred from the current package and should rarely be used.
func PluginDownloadURL(o string) ResourceOrInvokeOption {
	return resourceOrInvokeOption(func(ro *resourceOptions, io *invokeOptions) {
		switch {
		case ro != nil:
			ro.PluginDownloadURL = o
		case io != nil:
			io.PluginDownloadURL = o
		}
	})
}

// If set to True, the providers Delete method will not be called for this resource.
func RetainOnDelete(b bool) ResourceOption {
	return resourceOption(func(ro *resourceOptions) {
		ro.RetainOnDelete = b
	})
}

// If set, the providers Delete method will not be called for this resource
// if specified resource is being deleted as well.
func DeletedWith(r Resource) ResourceOption {
	return resourceOption(func(ro *resourceOptions) {
		ro.DeletedWith = r
	})
}

// If set this resource will be parameterized with the given package reference.
func Parameterization(parameter []byte) ResourceOrInvokeOption {
	return resourceOrInvokeOption(func(ro *resourceOptions, io *invokeOptions) {
		switch {
		case ro != nil:
			ro.Parameterization = parameter
		case io != nil:
			io.Parameterization = parameter
		}
	})
}
