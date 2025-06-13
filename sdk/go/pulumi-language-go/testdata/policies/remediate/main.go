// Copyright 2025, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/policyx"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumix"

	"github.com/blang/semver"
)

func main() {
	if err := policyx.Main(func(host pulumix.Engine) (policyx.PolicyPack, error) {
		version := semver.MustParse("3.0.0")
		return policyx.NewPolicyPack(
			"remediate", version, policyx.EnforcementLevelAdvisory, nil,
			[]policyx.Policy{
				policyx.NewResourceRemediationPolicy("fixup", policyx.ResourceRemediationPolicyArgs{
					Description: "Sets property to config",
					RemediateResource: func(ctx context.Context, args policyx.ResourceRemediationArgs) (*property.Map, error) {
						if args.Resource.Type != "simple:index:Resource" {
							return nil, nil
						}

						value := args.Config["value"].(bool)
						if val, ok := args.Resource.Properties.GetOk("value"); ok && val.AsBool() != value {
							result := property.NewMap(map[string]property.Value{
								"value": property.New(value),
							})
							return &result, nil
						}
						return nil, nil
					},
				}),
			})
	}); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
