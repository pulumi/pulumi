// Copyright 2026, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/policyx"

	"github.com/blang/semver"
)

func main() {
	if err := policyx.Main(func(pctx *pulumi.Context) (policyx.PolicyPack, error) {
		version := semver.MustParse("1.0.0")
		return policyx.NewPolicyPack(
			"analyze-stack", version, policyx.EnforcementLevelMandatory,
			[]policyx.Policy{
				policyx.NewStackValidationPolicy("stack-size", policyx.StackValidationPolicyArgs{
					Description:      "Stack must contain at most one simple:index:Resource",
					EnforcementLevel: policyx.EnforcementLevelMandatory,
					ValidateStack: func(ctx context.Context, args policyx.StackValidationArgs) error {
						count := 0
						for _, r := range args.Resources {
							if r.Type == "simple:index:Resource" {
								count++
							}
						}
						if count > 1 {
							args.Manager.ReportViolation("Found an extra simple:index:Resource", "")
						}
						return nil
					},
				}),
			})
	}); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
