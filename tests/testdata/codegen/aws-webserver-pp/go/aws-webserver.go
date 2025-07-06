package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Create a new security group for port 80.
		securityGroup, err := ec2.NewSecurityGroup(ctx, "securityGroup", &ec2.SecurityGroupArgs{
			Ingress: []ec2.SecurityGroupIngressArgs{
				{
					Protocol: pulumi.String("tcp"),
					FromPort: pulumi.Int(0),
					ToPort:   pulumi.Int(0),
					CidrBlocks: []pulumi.String{
						pulumi.String("0.0.0.0/0"),
					},
				},
			},
		})
		if err != nil {
			return err
		}
		// Get the ID for the latest Amazon Linux AMI.
		ami, err := aws.GetAmi(ctx, &aws.GetAmiArgs{
			Filters: []aws.GetAmiFilter{
				{
					Name: "name",
					Values: []string{
						"amzn-ami-hvm-*-x86_64-ebs",
					},
				},
			},
			Owners: []string{
				"137112412989",
			},
			MostRecent: pulumi.BoolRef(true),
		}, nil)
		if err != nil {
			return err
		}
		// Create a simple web server using the startup script for the instance.
		server, err := ec2.NewInstance(ctx, "server", &ec2.InstanceArgs{
			Tags: map[string]pulumi.String{
				"Name": pulumi.String("web-server-www"),
			},
			InstanceType: ec2.InstanceType_T2_Micro,
			SecurityGroups: []pulumi.String{
				securityGroup.Name,
			},
			Ami:      pulumi.String(ami.Id),
			UserData: "#!/bin/bash\necho \"Hello, World!\" > index.html\nnohup python -m SimpleHTTPServer 80 &\n",
		})
		if err != nil {
			return err
		}
		ctx.Export("publicIp", server.PublicIp)
		ctx.Export("publicHostName", server.PublicDns)
		return nil
	})
}
