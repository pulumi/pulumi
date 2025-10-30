// Copyright 2025, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/policyx"

	"github.com/blang/semver"
)

func main() {
	if err := policyx.Main(func(pctx *pulumi.Context) (policyx.PolicyPack, error) {
		cfg := config.New(pctx, "")
		value := cfg.RequireBool("value")
		version := semver.MustParse("2.0.0")
		return policyx.NewPolicyPack(
			"stack-config", version, policyx.EnforcementLevelMandatory,
			[]policyx.Policy{
				policyx.NewResourceValidationPolicy(fmt.Sprintf("validate-%t", value), policyx.ResourceValidationPolicyArgs{
					Description:		fmt.Sprintf("Verifies property is %t", value),
					EnforcementLevel:	policyx.EnforcementLevelMandatory,
					ValidateResource: func(ctx context.Context, args policyx.ResourceValidationArgs) error {
						if args.Resource.Type != "simple:index:Resource" {
							return nil
						}

						if val, ok := args.Resource.Properties.GetOk("value"); ok && val.AsBool() != value {
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
