package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		securityGroup, err := ec2.NewSecurityGroup(ctx, "securityGroup", &ec2.SecurityGroupArgs{
			Ingress: ec2.SecurityGroupIngressArray{
				&ec2.SecurityGroupIngressArgs{
					Protocol: pulumi.String("tcp"),
					FromPort: pulumi.Int(0),
					ToPort:   pulumi.Int(0),
					CidrBlocks: pulumi.StringArray{
						pulumi.String("0.0.0.0/0"),
					},
				},
			},
		})
		if err != nil {
			return err
		}
		ami := aws.GetAmiOutput(ctx, aws.GetAmiOutputArgs{
			Filters: aws.GetAmiFilterArray{
				&aws.GetAmiFilterArgs{
					Name: pulumi.String("name"),
					Values: pulumi.StringArray{
						pulumi.String("amzn-ami-hvm-*-x86_64-ebs"),
					},
				},
			},
			Owners: pulumi.StringArray{
				pulumi.String("137112412989"),
			},
			MostRecent: pulumi.Bool(true),
		}, nil)
		server, err := ec2.NewInstance(ctx, "server", &ec2.InstanceArgs{
			Tags: pulumi.StringMap{
				"Name": pulumi.String("web-server-www"),
			},
			InstanceType: pulumi.String("t2.micro"),
			SecurityGroups: pulumi.StringArray{
				securityGroup.Name,
			},
			Ami: ami.ApplyT(func(ami aws.GetAmiResult) (string, error) {
				return ami.Id, nil
			}).(pulumi.StringOutput),
			UserData: pulumi.String("#!/bin/bash\necho \"Hello, World!\" > index.html\nnohup python -m SimpleHTTPServer 80 &\n"),
		})
		if err != nil {
			return err
		}
		ctx.Export("publicIp", server.PublicIp)
		ctx.Export("publicHostName", server.PublicDns)
		return nil
	})
}
