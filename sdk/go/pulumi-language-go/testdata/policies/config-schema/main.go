// Copyright 2025, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"
	"fmt"
	"slices"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/policyx"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumix"

	"github.com/blang/semver"
)

func main() {
	if err := policyx.Main(func(host pulumix.Engine) (policyx.PolicyPack, error) {
		version := semver.MustParse("3.0.0")
		return policyx.NewPolicyPack(
			"config-schema", version, policyx.EnforcementLevelAdvisory, nil,
			[]policyx.Policy{
				policyx.NewResourceValidationPolicy("validator", policyx.ResourceValidationPolicyArgs{
					Description:      "Verifies property matches config",
					EnforcementLevel: policyx.EnforcementLevelAdvisory,
					ConfigSchema: &policyx.ConfigSchema{
						Properties: map[string]map[string]any{
							"value": {
								"type": "boolean",
							},
							"names": {
								"type": "array",
								"items": map[string]any{
									"type": "string",
								},
								"minItems": 1,
							},
						},
						Required: []string{"value", "names"},
					},
					ValidateResource: func(ctx context.Context, args policyx.ResourceValidationArgs) error {
						if args.Resource.Type != "simple:index:Resource" {
							return nil
						}

						value := args.Config["value"].(bool)
						names := args.Config["names"].([]any)
						strNames := make([]string, len(names))
						for i, n := range names {
							strNames[i] = n.(string)
						}

						if slices.Contains(strNames, args.Resource.Name) {
							if val, ok := args.Resource.Properties.GetOk("value"); ok && val.AsBool() != value {
								args.Manager.ReportViolation(fmt.Sprintf("Property was %t", val.AsBool()), "")
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
