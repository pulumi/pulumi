package main

import (
	"encoding/json"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		tmpJSON0, err := json.Marshal(map[string]interface{}{
			"Version": "2012-10-17",
			"Statement": []map[string]interface{}{
				map[string]interface{}{
					"Effect":   "Allow",
					"Action":   "lambda:*",
					"Resource": "arn:aws:lambda:*:*:function:*",
					"Condition": map[string]interface{}{
						"StringEquals": map[string]interface{}{
							"aws:RequestTag/Team": []string{
								"iamuser-admin",
								"iamuser2-admin",
							},
						},
						"ForAllValues:StringEquals": map[string]interface{}{
							"aws:TagKeys": []string{
								"Team",
							},
						},
					},
				},
			},
		})
		if err != nil {
			return err
		}
		json0 := string(tmpJSON0)
		// Create a policy with multiple Condition keys
		policy, err := iam.NewPolicy(ctx, "policy", &iam.PolicyArgs{
			Path:        pulumi.String("/"),
			Description: pulumi.String("My test policy"),
			Policy:      pulumi.String(json0),
		})
		if err != nil {
			return err
		}
		ctx.Export("policyName", policy.Name)
		return nil
	})
}
