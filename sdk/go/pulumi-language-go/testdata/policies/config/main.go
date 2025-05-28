// Copyright 2025, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/policyx"

	"github.com/blang/semver"
)

func main() {
	if err := policyx.Main(func(host engine.Host) (policyx.PolicyPack, error) {
		version := semver.MustParse("2.0.0")
		return policyx.NewPolicyPack(
			"config", version, policyx.EnforcementLevel_Mandatory, nil,
			[]policyx.Policy{
				policyx.NewResourceValidationPolicy("allowed", policyx.ResourceValidationPolicyArgs{
					Description:      "Verifies properties",
					EnforcementLevel: policyx.EnforcementLevel_Mandatory,
					ValidateResource: func(ctx context.Context, args policyx.ResourceValidationArgs) error {
						if args.Resource.Type != "simple:index:Resource" {
							return nil
						}

						value := args.Config["value"].(bool)
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
