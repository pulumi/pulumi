// Copyright 2025, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"
	"errors"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/policyx"

	"github.com/blang/semver"
)

func main() {
	if err := policyx.Main(func(pctx *pulumi.Context) (policyx.PolicyPack, error) {
		version := semver.MustParse("1.0.0")
		return policyx.NewPolicyPack(
			"invalid-policy", version, policyx.EnforcementLevelAdvisory,
			[]policyx.Policy{
				policyx.NewResourceValidationPolicy("all", policyx.ResourceValidationPolicyArgs{
					Description:      "Invalid policy name",
					EnforcementLevel: policyx.EnforcementLevelAdvisory,
					ValidateResource: func(ctx context.Context, args policyx.ResourceValidationArgs) error {
						return errors.New("Should never run.")
					},
				}),
			})
	}); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
