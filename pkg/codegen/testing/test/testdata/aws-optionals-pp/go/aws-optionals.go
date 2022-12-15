package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		policyDocument, err := iam.GetPolicyDocument(ctx, &iam.GetPolicyDocumentArgs{
			Statements: []iam.GetPolicyDocumentStatement{
				{
					Sid: pulumi.StringRef("1"),
					Actions: []string{
						"s3:ListAllMyBuckets",
						"s3:GetBucketLocation",
					},
					Resources: []string{
						"arn:aws:s3:::*",
					},
				},
			},
		}, nil)
		if err != nil {
			return err
		}
		_, err = iam.NewPolicy(ctx, "example", &iam.PolicyArgs{
			Name:   pulumi.String("example_policy"),
			Path:   pulumi.String("/"),
			Policy: pulumi.String(policyDocument.Json),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
