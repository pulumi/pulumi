// Copyright 2025, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/policyx"

	"github.com/blang/semver"
)

func main() {
	if err := policyx.Main(func(pctx *pulumi.Context) (policyx.PolicyPack, error) {
		version := semver.MustParse("2.0.0")
		return policyx.NewPolicyPack(
			"stack-tags", version, policyx.EnforcementLevelMandatory,
			[]policyx.Policy{
				policyx.NewResourceValidationPolicy("allowed", policyx.ResourceValidationPolicyArgs{
					Description:      fmt.Sprintf("Verifies property equals the stack tag value"),
					EnforcementLevel: policyx.EnforcementLevelMandatory,
					ValidateResource: func(ctx context.Context, args policyx.ResourceValidationArgs) error {
						if args.Resource.Type != "simple:index:Resource" {
							return nil
						}

						tag, ok := args.StackTags["value"]
						if !ok {
							args.Manager.ReportViolation("Stack tag 'value' is required", "")
							return nil
						}

						var expected bool
						if err := json.Unmarshal([]byte(tag), &expected); err != nil {
							args.Manager.ReportViolation(fmt.Sprintf("Stack tag 'value' must be a boolean, got '%s'", tag), "")
							return nil
						}

						if val, ok := args.Resource.Properties.GetOk("value"); ok && val.AsBool() != expected {
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
