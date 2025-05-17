// Copyright 2025, Pulumi Corporation.
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

package policyx

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// AnalyzerProviderResource represents a Pulumi resource as seen by an analyzer provider.
// It contains the type, properties, URN, and name of the resource.
type AnalyzerProviderResource struct {
	// Type is the type token of the resource.
	Type string
	// Properties are the full properties to use for validation.
	Properties property.Map
	// URN is the URN of the resource.
	URN string
	// Name is the name for the resource's URN.
	Name string
}

// AnalyzerResource defines the view of a Pulumi-managed resource as sent to Analyzers.
// The properties of the resource are specific to the type of analysis being performed.
// See the Analyzer service definition for more information.
type AnalyzerResource struct {
	// Type is the type token of the resource.
	Type string
	// Properties are the full properties to use for validation.
	Properties property.Map
	// URN is the URN of the resource.
	URN string
	// Name is the name for the resource's URN.
	Name string
	// Options are the resource options for the resource.
	Options pulumi.ResourceOptions
	// Provider is the provider for the resource.
	Provider AnalyzerProviderResource
	// Parent is the URN of the parent resource, if any.
	Parent string
	// Dependencies is a list of URNs of resources this resource depends on.
	Dependencies []string
	// PropertyDependencies maps property names to the list of URNs they depend on.
	PropertyDependencies map[string][]string
}

// ResourceValidationArgs contains the arguments passed to a resource validation policy.
type ResourceValidationArgs struct {
	// Manager is the policy manager.
	Manager PolicyManager
	// Resource is the resource being validated.
	Resource AnalyzerResource
	// Config is the policy configuration.
	Config map[string]any
}

// StackValidationArgs contains the arguments passed to a stack validation policy.
type StackValidationArgs struct {
	// Manager is the policy manager.
	Manager PolicyManager
	// Resources is the list of resources in the stack.
	Resources []AnalyzerResource
}

// Policy is the interface implemented by all policies.
type Policy interface {
	isPolicy()
	// Name returns the name of the policy.
	Name() string
	// Description returns the description of the policy.
	Description() string
	// EnforcementLevel returns the enforcement level of the policy.
	EnforcementLevel() EnforcementLevel
	// ConfigSchema returns the configuration schema for the policy, if any.
	ConfigSchema() *PolicyConfigSchema
}

// ResourceValidationPolicy is a policy that validates individual resources.
type ResourceValidationPolicy interface {
	Policy
	// Validate validates a resource.
	Validate(ctx context.Context, args ResourceValidationArgs) error
}

// StackValidationPolicy is a policy that validates the entire stack.
type StackValidationPolicy interface {
	Policy
	// Validate validates the stack.
	Validate(ctx context.Context, args StackValidationArgs) error
}

// ResourceValidationPolicyArgs contains the arguments for creating a resource validation policy.
type ResourceValidationPolicyArgs struct {
	// Description is the description of the policy.
	Description string
	// EnforcementLevel is the enforcement level of the policy.
	EnforcementLevel EnforcementLevel
	// ValidateResource is the validation function for the policy.
	ValidateResource func(ctx context.Context, args ResourceValidationArgs) error
}

// resourceValidationPolicy is an implementation of ResourceValidationPolicy.
type resourceValidationPolicy struct {
	name             string
	description      string
	enforcementLevel EnforcementLevel
	validateResource func(ctx context.Context, args ResourceValidationArgs) error
}

// isPolicy marks resourceValidationPolicy as a Policy.
func (p *resourceValidationPolicy) isPolicy() {}

// Name returns the name of the policy.
func (p *resourceValidationPolicy) Name() string {
	return p.name
}

// Description returns the description of the policy.
func (p *resourceValidationPolicy) Description() string {
	return p.description
}

// EnforcementLevel returns the enforcement level of the policy.
func (p *resourceValidationPolicy) EnforcementLevel() EnforcementLevel {
	return p.enforcementLevel
}

// ConfigSchema returns the configuration schema for the policy, if any.
func (p *resourceValidationPolicy) ConfigSchema() *PolicyConfigSchema {
	return nil
}

// Validate validates a resource using the policy's validation function.
func (p *resourceValidationPolicy) Validate(ctx context.Context, args ResourceValidationArgs) error {
	if p.validateResource == nil {
		return nil
	}
	return p.validateResource(ctx, args)
}

// NewResourceValidationPolicy creates a new ResourceValidationPolicy with the given name and arguments.
func NewResourceValidationPolicy(
	name string,
	args ResourceValidationPolicyArgs,
) ResourceValidationPolicy {
	return &resourceValidationPolicy{
		name:             name,
		description:      args.Description,
		enforcementLevel: args.EnforcementLevel,
		validateResource: args.ValidateResource,
	}
}
