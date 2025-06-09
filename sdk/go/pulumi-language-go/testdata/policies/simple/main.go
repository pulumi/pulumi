// Copyright 2025, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/policyx"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumix"

	"github.com/blang/semver"
)

func main() {
	if err := policyx.Main(func(host pulumix.Engine) (policyx.PolicyPack, error) {
		version := semver.MustParse("1.0.0")
		return policyx.NewPolicyPack(
			"simple", version, policyx.EnforcementLevelAdvisory, nil,
			[]policyx.Policy{
				policyx.NewResourceValidationPolicy("truthiness", policyx.ResourceValidationPolicyArgs{
					Description:      "Verifies properties are true",
					EnforcementLevel: policyx.EnforcementLevelAdvisory,
					ValidateResource: func(ctx context.Context, args policyx.ResourceValidationArgs) error {
						if args.Resource.Type != "simple:index:Resource" {
							return nil
						}
						if val, ok := args.Resource.Properties.GetOk("value"); ok && val.AsBool() == true {
							args.Manager.ReportViolation("This is a test warning", "")
						}
						return nil
					},
				}),
				policyx.NewResourceValidationPolicy("falsiness", policyx.ResourceValidationPolicyArgs{
					Description:      "Verifies properties are false",
					EnforcementLevel: policyx.EnforcementLevelMandatory,
					ValidateResource: func(ctx context.Context, args policyx.ResourceValidationArgs) error {
						if args.Resource.Type != "simple:index:Resource" {
							return nil
						}
						if val, ok := args.Resource.Properties.GetOk("value"); ok && val.AsBool() == false {
							args.Manager.ReportViolation("This is a test error", "")
						}
						return nil
					},
				}),
			})
	}); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
