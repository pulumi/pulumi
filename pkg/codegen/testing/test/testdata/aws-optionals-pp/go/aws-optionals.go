package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		policyDocument := iam.GetPolicyDocumentOutput(ctx, iam.GetPolicyDocumentOutputArgs{
			Statements: iam.GetPolicyDocumentStatementArray{
				&iam.GetPolicyDocumentStatementArgs{
					Sid: pulumi.String("1"),
					Actions: pulumi.StringArray{
						pulumi.String("s3:ListAllMyBuckets"),
						pulumi.String("s3:GetBucketLocation"),
					},
					Resources: pulumi.StringArray{
						pulumi.String("arn:aws:s3:::*"),
					},
				},
			},
		}, nil)
		_, err := iam.NewPolicy(ctx, "example", &iam.PolicyArgs{
			Name: pulumi.String("example_policy"),
			Path: pulumi.String("/"),
			Policy: policyDocument.ApplyT(func(policyDocument iam.GetPolicyDocumentResult) (string, error) {
				return policyDocument.Json, nil
			}).(pulumi.StringOutput),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
