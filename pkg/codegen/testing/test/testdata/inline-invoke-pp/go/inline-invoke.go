package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := ec2.NewInstance(ctx, "webServer", &ec2.InstanceArgs{
			Ami: aws.GetAmiOutput(ctx, GetAmiResultOutputArgs{
				Filters: []map[string]interface{}{
					map[string]interface{}{
						"name": "name",
						"values": []string{
							"amzn-ami-hvm-*-x86_64-ebs",
						},
					},
				},
				Owners: []string{
					"137112412989",
				},
				MostRecent: true,
			}, nil).Id,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
