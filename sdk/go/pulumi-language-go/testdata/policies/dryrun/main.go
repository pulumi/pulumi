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
			"dryrun", version, policyx.EnforcementLevelAdvisory, nil,
			[]policyx.Policy{
				policyx.NewResourceValidationPolicy("dry", policyx.ResourceValidationPolicyArgs{
					Description:      "Verifies properties are true on dryrun",
					EnforcementLevel: policyx.EnforcementLevelMandatory,
					ValidateResource: func(ctx context.Context, args policyx.ResourceValidationArgs) error {
						if args.Resource.Type != "simple:index:Resource" {
							return nil
						}

						if args.DryRun {
							if val, ok := args.Resource.Properties.GetOk("value"); ok && val.AsBool() == false {
								args.Manager.ReportViolation("This is a test error", "")
							}
						}
						return nil
					},
				}),
			})
	}); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
