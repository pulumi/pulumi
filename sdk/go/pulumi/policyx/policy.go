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

type AnalyzerProviderResource struct {
	// the type token of the resource.
	Type string
	// the full properties to use for validation.
	Properties property.Map
	// the URN of the resource.
	Urn string
	// the name for the resource's URN.
	Name string
}

// AnalyzerResource defines the view of a Pulumi-managed resource as sent to Analyzers. The properties
// of the resource are specific to the type of analysis being performed. See the Analyzer
// service definition for more information.
type AnalyzerResource struct {
	Type                 string
	Properties           property.Map
	Urn                  string
	Name                 string
	Options              pulumi.ResourceOptions
	Provider             AnalyzerProviderResource
	Parent               string
	Dependencies         []string
	PropertyDependencies map[string][]string
}

type ResourceValidationArgs struct {
	Manager  PolicyManager
	Resource AnalyzerResource
	Config   map[string]any
}

type StackValidationArgs struct {
	Manager   PolicyManager
	Resources []AnalyzerResource
}

type Policy interface {
	isPolicy()

	Name() string
	Description() string
	EnforcementLevel() EnforcementLevel
	ConfigSchema() *PolicyConfigSchema
}

type ResourceValidationPolicy interface {
	Policy

	Validate(ctx context.Context, args ResourceValidationArgs) error
}

type StackValidationPolicy interface {
	Policy

	Validate(ctx context.Context, args StackValidationArgs) error
}

type ResourceValidationPolicyArgs struct {
	Description      string
	EnforcementLevel EnforcementLevel
	ValidateResource func(ctx context.Context, args ResourceValidationArgs) error
}

type resourceValidationPolicy struct {
	name             string
	description      string
	enforcementLevel EnforcementLevel
	validateResource func(ctx context.Context, args ResourceValidationArgs) error
}

func (p *resourceValidationPolicy) isPolicy() {}
func (p *resourceValidationPolicy) Name() string {
	return p.name
}

func (p *resourceValidationPolicy) Description() string {
	return p.description
}

func (p *resourceValidationPolicy) EnforcementLevel() EnforcementLevel {
	return p.enforcementLevel
}

func (p *resourceValidationPolicy) ConfigSchema() *PolicyConfigSchema {
	return nil
}

func (p *resourceValidationPolicy) Validate(ctx context.Context, args ResourceValidationArgs) error {
	if p.validateResource == nil {
		return nil
	}

	return p.validateResource(ctx, args)
}

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
