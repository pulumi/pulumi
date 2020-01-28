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

package pulumi

import (
	"fmt"
	"os"
	"reflect"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
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
var stackReferenceStateType = reflect.TypeOf(StackReferenceState{})

// ResourceState is the base
type ResourceState struct {
	urn URNOutput `pulumi:"urn"`

	providers map[string]ProviderResource
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

func (ResourceState) isResource() {}

type CustomResourceState struct {
	ResourceState

	id IDOutput `pulumi:"id"`
}

func (s CustomResourceState) ID() IDOutput {
	return s.id
}

func (CustomResourceState) isCustomResource() {}

type ProviderResourceState struct {
	CustomResourceState

	pkg string
}

func (s ProviderResourceState) getPackage() string {
	return s.pkg
}

type StackReferenceState struct {
	CustomResourceState

	name    StringOutput `pulumi:"name"`
	outputs MapOutput    `pulumi:"outputs"`
}

func (s StackReferenceState) GetOutput(name StringInput) StringOutput {
	return All(name.ToStringOutput(), s.outputs).
		ApplyT(func(args []interface{}) (string, error) {
			n := args[0].(string)
			outs := args[1].(map[string]interface{})
			return outs[n].(string), nil
		}).(StringOutput)
}

// Resource represents a cloud resource managed by Pulumi.
type Resource interface {
	// URN is this resource's stable logical URN used to distinctly address it before, during, and after deployments.
	URN() URNOutput

	// getProviders returns the provider map for this resource.
	getProviders() map[string]ProviderResource

	// isResource() is a marker method used to ensure that all Resource types embed a ResourceState.
	isResource()
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

// StackReference manages a reference to a Pulumi stack. The referenced stack's outputs are available via its
// "GetOutput" method.
type StackReference interface {
	CustomResource

	GetOutput(name StringInput) StringOutput
}

// StackReferenceArgs is the input to NewStackReference that allows specifying a stack name
// Name is in the form "Org/Program/Stack"
type StackReferenceArgs struct {
	Name StringInput
}

type stackReferenceProps struct {
	Name    string                 `pulumi:"name"`
	Outputs map[string]interface{} `pulumi:"outputs"`
}

func (s stackReferenceProps) GetOutput(name StringInput) StringOutput {
	return All(name.ToStringOutput(), s.Outputs).
		ApplyT(func(args []interface{}) (string, error) {
			n := args[0].(string)
			outs := args[1].(map[string]string)
			return outs[n], nil
		}).(StringOutput)
}

var stackReferencePropsType = reflect.TypeOf((*stackReferenceProps)(nil)).Elem()

type stackReferencePropsInputValue struct {
	Name    StringInput `pulumi:"name"`
	Outputs MapInput    `pulumi:"outputs"`
}

func (stackReferencePropsInputValue) ElementType() reflect.Type {
	return stackReferencePropsType
}

func (v stackReferencePropsInputValue) ToStackReferencePropsOutput() stackReferencePropsOutput {
	return ToOutput(v).(stackReferencePropsOutput)
}

func (v stackReferencePropsInputValue) ToStackReferencePropsOutputWithContext(
	ctx context.Context) stackReferencePropsOutput {
	return ToOutputWithContext(ctx, v).(stackReferencePropsOutput)
}

type stackReferencePropsOutput struct{ *OutputState }

func (stackReferencePropsOutput) ElementType() reflect.Type {
	return stackReferencePropsType
}

func (o stackReferencePropsOutput) ToStackReferencePropsOutput() stackReferencePropsOutput {
	return o
}

func (o stackReferencePropsOutput) ToStackReferencePropsOutputWithContext(
	ctx context.Context) stackReferencePropsOutput {
	return o
}

// NewStackReference creates a stack reference that makes available outputs from the specified stack
func NewStackReference(
	ctx Context, name string, args *StackReferenceArgs, opts ...ResourceOption) (StackReference, error) {
	var stack StringInput
	if args != nil {
		stack = args.Name
	}
	if stack == nil {
		stack = StringInput(String(name))
	}
	var oo MapOutput = MapOutput{newOutputState(reflect.TypeOf(map[string]interface{}{}))}
	var oi MapInput

	var stackRef StackReferenceState = StackReferenceState{
		name:    stack.ToStringOutput(),
		outputs: oo,
	}

	t := "pulumi:pulumi:StackReference"
	id := stack.ToStringOutput().ApplyT(func(s string) (ID, error) { return ID(s), nil }).(IDOutput)

	props := stackReferencePropsInputValue{
		Name:    stack,
		Outputs: oi,
	}

	err := ctx.ReadResource(t, name, id, &props, &stackRef, opts...)

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return nil, errors.Wrap(err, "error reading stack reference")
	}

	return &stackRef, nil
}

type CustomTimeouts struct {
	Create string
	Update string
	Delete string
}

type resourceOptions struct {
	// Parent is an optional parent resource to which this resource belongs.
	Parent Resource
	// DependsOn is an optional array of explicit dependencies on other resources.
	DependsOn []Resource
	// Protect, when set to true, ensures that this resource cannot be deleted (without first setting it to false).
	Protect bool
	// Provider is an optional provider resource to use for this resource's CRUD operations.
	Provider ProviderResource
	// Providers is an optional map of package to provider resource for a component resource.
	Providers map[string]ProviderResource
	// DeleteBeforeReplace, when set to true, ensures that this resource is deleted prior to replacement.
	DeleteBeforeReplace bool
	// Import, when provided with a resource ID, indicates that this resource's provider should import its state from
	// the cloud resource with the given ID. The inputs to the resource's constructor must align with the resource's
	// current state. Once a resource has been imported, the import property must be removed from the resource's
	// options.
	Import IDInput
	// CustomTimeouts is an optional configuration block used for CRUD operations
	CustomTimeouts *CustomTimeouts
	// Ignore changes to any of the specified properties.
	IgnoreChanges []string
}

type invokeOptions struct {
	// Parent is an optional parent resource to use for default provider options for this invoke.
	Parent Resource
	// Provider is an optional provider resource to use for this invoke.
	Provider ProviderResource
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

type resourceOrInvokeOption func(ro *resourceOptions, io *invokeOptions)

func (o resourceOrInvokeOption) applyResourceOption(opts *resourceOptions) {
	o(opts, nil)
}

func (o resourceOrInvokeOption) applyInvokeOption(opts *invokeOptions) {
	o(nil, opts)
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

// Provider sets the provider resource to use for a resource's CRUD operations or an invoke's call.
func Provider(r ProviderResource) ResourceOrInvokeOption {
	return resourceOrInvokeOption(func(ro *resourceOptions, io *invokeOptions) {
		switch {
		case ro != nil:
			ro.Provider = r
		case io != nil:
			io.Provider = r
		}
	})
}

// DependsOn is an optional array of explicit dependencies on other resources.
func DependsOn(o []Resource) ResourceOption {
	return resourceOption(func(ro *resourceOptions) {
		ro.DependsOn = append(ro.DependsOn, o...)
	})
}

// Protect, when set to true, ensures that this resource cannot be deleted (without first setting it to false).
func Protect(o bool) ResourceOption {
	return resourceOption(func(ro *resourceOptions) {
		ro.Protect = o
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

// DeleteBeforeReplace, when set to true, ensures that this resource is deleted prior to replacement.
func DeleteBeforeReplace(o bool) ResourceOption {
	return resourceOption(func(ro *resourceOptions) {
		ro.DeleteBeforeReplace = o
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

// Timeouts is an optional configuration block used for CRUD operations
func Timeouts(o *CustomTimeouts) ResourceOption {
	return resourceOption(func(ro *resourceOptions) {
		ro.CustomTimeouts = o
	})
}

// Ignore changes to any of the specified properties.
func IgnoreChanges(o []string) ResourceOption {
	return resourceOption(func(ro *resourceOptions) {
		ro.IgnoreChanges = o
	})
}
