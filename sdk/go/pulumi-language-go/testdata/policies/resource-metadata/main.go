// Copyright 2026, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"
	"strings"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/policyx"
)

func resourceNamed(args policyx.ResourceValidationArgs, name string) bool {
	return args.Resource.Type == "simple:index:Resource" && args.Resource.Name == name
}

func stackResourceNamed(resources []policyx.AnalyzerResource, name string) *policyx.AnalyzerResource {
	for i := range resources {
		if resources[i].Type == "simple:index:Resource" && resources[i].Name == name {
			return &resources[i]
		}
	}
	return nil
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func containsURN(values []string, name string) bool {
	for _, value := range values {
		if strings.Contains(value, "::"+name) {
			return true
		}
	}
	return false
}

func reportIfResource(ctx context.Context, args policyx.ResourceValidationArgs, name string, ok bool) error {
	if resourceNamed(args, name) && ok {
		args.Manager.ReportViolation("metadata matched", "")
	}
	return nil
}

func main() {
	if err := policyx.Main(func(pctx *pulumi.Context) (policyx.PolicyPack, error) {
		version := semver.MustParse("1.0.0")
		return policyx.NewPolicyPack(
			"resource-metadata", version, policyx.EnforcementLevelAdvisory,
			[]policyx.Policy{
				policyx.NewResourceValidationPolicy("name-type-urn-props", policyx.ResourceValidationPolicyArgs{
					Description:      "Resource identity metadata is available",
					EnforcementLevel: policyx.EnforcementLevelAdvisory,
					ValidateResource: func(ctx context.Context, args policyx.ResourceValidationArgs) error {
						value, ok := args.Resource.Properties.GetOk("value")
						return reportIfResource(ctx, args, "identity",
							ok && value.AsBool() && strings.Contains(args.Resource.URN, "::identity"))
					},
				}),
				policyx.NewResourceValidationPolicy("protect", policyx.ResourceValidationPolicyArgs{
					Description:      "Protect option metadata is available",
					EnforcementLevel: policyx.EnforcementLevelAdvisory,
					ValidateResource: func(ctx context.Context, args policyx.ResourceValidationArgs) error {
						return reportIfResource(ctx, args, "protected", args.Resource.Options.Protect)
					},
				}),
				policyx.NewResourceValidationPolicy("ignore-changes", policyx.ResourceValidationPolicyArgs{
					Description:      "IgnoreChanges option metadata is available",
					EnforcementLevel: policyx.EnforcementLevelAdvisory,
					ValidateResource: func(ctx context.Context, args policyx.ResourceValidationArgs) error {
						return reportIfResource(ctx, args, "ignoreChanges",
							contains(args.Resource.Options.IgnoreChanges, "value"))
					},
				}),
				policyx.NewResourceValidationPolicy("delete-before-replace", policyx.ResourceValidationPolicyArgs{
					Description:      "DeleteBeforeReplace option metadata is available",
					EnforcementLevel: policyx.EnforcementLevelAdvisory,
					ValidateResource: func(ctx context.Context, args policyx.ResourceValidationArgs) error {
						return reportIfResource(ctx, args, "deleteBeforeReplace",
							args.Resource.Options.DeleteBeforeReplace)
					},
				}),
				policyx.NewResourceValidationPolicy("additional-secret-outputs", policyx.ResourceValidationPolicyArgs{
					Description:      "AdditionalSecretOutputs option metadata is available",
					EnforcementLevel: policyx.EnforcementLevelAdvisory,
					ValidateResource: func(ctx context.Context, args policyx.ResourceValidationArgs) error {
						return reportIfResource(ctx, args, "secretOutput",
							contains(args.Resource.Options.AdditionalSecretOutputs, "value"))
					},
				}),
				policyx.NewResourceValidationPolicy("custom-timeouts", policyx.ResourceValidationPolicyArgs{
					Description:      "CustomTimeouts option metadata is available",
					EnforcementLevel: policyx.EnforcementLevelAdvisory,
					ValidateResource: func(ctx context.Context, args policyx.ResourceValidationArgs) error {
						timeouts := args.Resource.Options.CustomTimeouts
						return reportIfResource(ctx, args, "customTimeouts", timeouts != nil && timeouts.Create == "5m0s")
					},
				}),
				policyx.NewResourceValidationPolicy("provider", policyx.ResourceValidationPolicyArgs{
					Description:      "Provider metadata is available",
					EnforcementLevel: policyx.EnforcementLevelAdvisory,
					ValidateResource: func(ctx context.Context, args policyx.ResourceValidationArgs) error {
						return reportIfResource(ctx, args, "explicitProvider",
							args.Resource.Provider.Type == "pulumi:providers:simple" && args.Resource.Provider.Name == "prov")
					},
				}),
				policyx.NewResourceValidationPolicy("parent", policyx.ResourceValidationPolicyArgs{
					Description:      "Parent metadata is available",
					EnforcementLevel: policyx.EnforcementLevelAdvisory,
					ValidateResource: func(ctx context.Context, args policyx.ResourceValidationArgs) error {
						return reportIfResource(ctx, args, "child", strings.Contains(args.Resource.Parent, "::parent"))
					},
				}),
				policyx.NewStackValidationPolicy("dependencies", policyx.StackValidationPolicyArgs{
					Description:      "Dependency metadata is available",
					EnforcementLevel: policyx.EnforcementLevelAdvisory,
					ValidateStack: func(ctx context.Context, args policyx.StackValidationArgs) error {
						if resource := stackResourceNamed(args.Resources, "dependsOn"); resource != nil &&
							containsURN(resource.Dependencies, "dependency") {
							args.Manager.ReportViolation("metadata matched", resource.URN)
						}
						return nil
					},
				}),
				policyx.NewStackValidationPolicy("property-dependencies", policyx.StackValidationPolicyArgs{
					Description:      "Property dependency metadata is available",
					EnforcementLevel: policyx.EnforcementLevelAdvisory,
					ValidateStack: func(ctx context.Context, args policyx.StackValidationArgs) error {
						if resource := stackResourceNamed(args.Resources, "propertyDependency"); resource != nil &&
							containsURN(resource.PropertyDependencies["value"], "dependency") {
							args.Manager.ReportViolation("metadata matched", resource.URN)
						}
						return nil
					},
				}),
			})
	}); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
