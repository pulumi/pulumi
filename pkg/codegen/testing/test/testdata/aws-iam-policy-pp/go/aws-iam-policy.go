package main

import (
	"encoding/json"

	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		tmpJSON0, err := json.Marshal(map[string]interface{}{
			"Version": "2012-10-17",
			"Statement": []map[string]interface{}{
				map[string]interface{}{
					"Effect":    "Allow",
					"Principal": "*",
					"Action": []string{
						"s3:GetObject",
					},
					"Resource": []string{
						"arn:aws:s3:::some-aws-bucket/*",
					},
					"Condition": map[string]interface{}{
						"Foo": map[string]interface{}{
							"Bar": []string{
								"iamuser-admin",
								"iamuser2-admin",
							},
						},
						"Baz": map[string]interface{}{
							"Qux": []string{
								"iamuser3-admin",
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
		_, err = iam.NewPolicy(ctx, "policy", &iam.PolicyArgs{
			Path:        pulumi.String("/"),
			Description: pulumi.String("My test policy"),
			Policy:      pulumi.String(json0),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
