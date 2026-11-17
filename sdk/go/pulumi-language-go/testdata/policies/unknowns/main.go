// Copyright 2026, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/policyx"

	"github.com/blang/semver"
)

func main() {
	const (
		packName    = "unknowns"
		packVersion = "1.0.0"
		policyName  = "report"
	)

	if err := policyx.Main(func(pctx *pulumi.Context) (policyx.PolicyPack, error) {
		version := semver.MustParse(packVersion)
		return policyx.NewPolicyPack(
			packName, version, policyx.EnforcementLevelAdvisory,
			[]policyx.Policy{
				policyx.NewResourceValidationPolicy(policyName, policyx.ResourceValidationPolicyArgs{
					Description:      "Reports what the analyzer saw",
					EnforcementLevel: policyx.EnforcementLevelAdvisory,
					ValidateResource: func(ctx context.Context, args policyx.ResourceValidationArgs) error {
						if args.Resource.Type != "simple:index:Resource" {
							return nil
						}
						val, ok := args.Resource.Properties.GetOk("value")
						if !ok {
							return nil
						}
						// When the value is unknown during preview we can't evaluate the policy.
						// Mirror the Node/Python runtimes, which auto-report a diagnostic
						// describing why the policy was skipped.
						if val.IsComputed() {
							args.Manager.ReportViolation(fmt.Sprintf(
								"can't run policy '%s' from policy pack '%s@v%s' during preview: "+
									"value at .value can't be known during preview",
								policyName, packName, packVersion), "")
							return nil
						}
						// Secrets must still be inspectable: the underlying bool is accessible
						// regardless of the secret flag.
						if val.AsBool() {
							args.Manager.ReportViolation("value is true", "")
						} else {
							args.Manager.ReportViolation("value is false", "")
						}
						return nil
					},
				}),
			})
	}); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
