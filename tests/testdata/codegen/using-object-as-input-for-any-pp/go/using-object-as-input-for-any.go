package main

import (
	"github.com/pulumi/pulumi-aws-native/sdk/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := iam.NewRole(ctx, "role", &iam.RoleArgs{
			RoleName: "ScriptIAMRole",
			AssumeRolePolicyDocument: pulumi.Any(map[string]interface{}{
				"Version": "2012-10-17",
				"Statement": []map[string]interface{}{
					map[string]interface{}{
						"Effect": "Allow",
						"Action": "sts:AssumeRole",
						"Principal": map[string]interface{}{
							"Service": []string{
								"cloudformation.amazonaws.com",
								"gamelift.amazonaws.com",
							},
						},
					},
				},
			}),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
