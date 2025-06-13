// Copyright 2025, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/policyx"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumix"

	"github.com/blang/semver"
)

func main() {
	if err := policyx.Main(func(host pulumix.Engine) (policyx.PolicyPack, error) {
		version := semver.MustParse("3.0.0")
		return policyx.NewPolicyPack(
			"enforcement-config", version, policyx.EnforcementLevelAdvisory, nil,
			[]policyx.Policy{
				policyx.NewResourceValidationPolicy("false", policyx.ResourceValidationPolicyArgs{
					Description:      "Verifies property is false",
					EnforcementLevel: policyx.EnforcementLevelAdvisory,
					ValidateResource: func(ctx context.Context, args policyx.ResourceValidationArgs) error {
						if args.Resource.Type != "simple:index:Resource" {
							return nil
						}

						if val, ok := args.Resource.Properties.GetOk("value"); ok && val.AsBool() {
							args.Manager.ReportViolation(fmt.Sprintf("Property was %t", val.AsBool()), "")
						}
						return nil
					},
				}),
			})
	}); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
