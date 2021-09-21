package main

import (
	"encoding/json"

	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/ecs"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/elasticloadbalancingv2"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		opt0 := true
		vpc, err := ec2.LookupVpc(ctx, &ec2.LookupVpcArgs{
			Default: &opt0,
		}, nil)
		if err != nil {
			return err
		}
		subnets, err := ec2.GetSubnetIds(ctx, &ec2.GetSubnetIdsArgs{
			VpcId: vpc.Id,
		}, nil)
		if err != nil {
			return err
		}
		webSecurityGroup, err := ec2.NewSecurityGroup(ctx, "webSecurityGroup", &ec2.SecurityGroupArgs{
			VpcId: pulumi.String(vpc.Id),
			Egress: ec2.SecurityGroupEgressArray{
				&ec2.SecurityGroupEgressArgs{
					Protocol: pulumi.String("-1"),
					FromPort: pulumi.Int(0),
					ToPort:   pulumi.Int(0),
					CidrBlocks: pulumi.StringArray{
						pulumi.String("0.0.0.0/0"),
					},
				},
			},
			Ingress: ec2.SecurityGroupIngressArray{
				&ec2.SecurityGroupIngressArgs{
					Protocol: pulumi.String("tcp"),
					FromPort: pulumi.Int(80),
					ToPort:   pulumi.Int(80),
					CidrBlocks: pulumi.StringArray{
						pulumi.String("0.0.0.0/0"),
					},
				},
			},
		})
		if err != nil {
			return err
		}
		cluster, err := ecs.NewCluster(ctx, "cluster", nil)
		if err != nil {
			return err
		}
		tmpJSON0, err := json.Marshal(map[string]interface{}{
			"Version": "2008-10-17",
			"Statement": []map[string]interface{}{
				map[string]interface{}{
					"Sid":    "",
					"Effect": "Allow",
					"Principal": map[string]interface{}{
						"Service": "ecs-tasks.amazonaws.com",
					},
					"Action": "sts:AssumeRole",
				},
			},
		})
		if err != nil {
			return err
		}
		json0 := string(tmpJSON0)
		taskExecRole, err := iam.NewRole(ctx, "taskExecRole", &iam.RoleArgs{
			AssumeRolePolicy: pulumi.String(json0),
		})
		if err != nil {
			return err
		}
		_, err = iam.NewRolePolicyAttachment(ctx, "taskExecRolePolicyAttachment", &iam.RolePolicyAttachmentArgs{
			Role:      taskExecRole.Name,
			PolicyArn: pulumi.String("arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"),
		})
		if err != nil {
			return err
		}
		webLoadBalancer, err := elasticloadbalancingv2.NewLoadBalancer(ctx, "webLoadBalancer", &elasticloadbalancingv2.LoadBalancerArgs{
			Subnets: subnets.Ids,
			SecurityGroups: pulumi.StringArray{
				webSecurityGroup.ID(),
			},
		})
		if err != nil {
			return err
		}
		webTargetGroup, err := elasticloadbalancingv2.NewTargetGroup(ctx, "webTargetGroup", &elasticloadbalancingv2.TargetGroupArgs{
			Port:       pulumi.Int(80),
			Protocol:   pulumi.String("HTTP"),
			TargetType: pulumi.String("ip"),
			VpcId:      pulumi.String(vpc.Id),
		})
		if err != nil {
			return err
		}
		webListener, err := elasticloadbalancingv2.NewListener(ctx, "webListener", &elasticloadbalancingv2.ListenerArgs{
			LoadBalancerArn: webLoadBalancer.Arn,
			Port:            pulumi.Int(80),
			DefaultActions: elasticloadbalancingv2.ListenerDefaultActionArray{
				&elasticloadbalancingv2.ListenerDefaultActionArgs{
					Type:           pulumi.String("forward"),
					TargetGroupArn: webTargetGroup.Arn,
				},
			},
		})
		if err != nil {
			return err
		}
		tmpJSON1, err := json.Marshal([]map[string]interface{}{
			map[string]interface{}{
				"name":  "my-app",
				"image": "nginx",
				"portMappings": []map[string]interface{}{
					map[string]interface{}{
						"containerPort": 80,
						"hostPort":      80,
						"protocol":      "tcp",
					},
				},
			},
		})
		if err != nil {
			return err
		}
		json1 := string(tmpJSON1)
		appTask, err := ecs.NewTaskDefinition(ctx, "appTask", &ecs.TaskDefinitionArgs{
			Family:      pulumi.String("fargate-task-definition"),
			Cpu:         pulumi.String("256"),
			Memory:      pulumi.String("512"),
			NetworkMode: pulumi.String("awsvpc"),
			RequiresCompatibilities: pulumi.StringArray{
				pulumi.String("FARGATE"),
			},
			ExecutionRoleArn:     taskExecRole.Arn,
			ContainerDefinitions: pulumi.String(json1),
		})
		if err != nil {
			return err
		}
		_, err = ecs.NewService(ctx, "appService", &ecs.ServiceArgs{
			Cluster:        cluster.Arn,
			DesiredCount:   pulumi.Int(5),
			LaunchType:     pulumi.String("FARGATE"),
			TaskDefinition: appTask.Arn,
			NetworkConfiguration: &ecs.ServiceNetworkConfigurationArgs{
				AssignPublicIp: pulumi.Bool(true),
				Subnets:        subnets.Ids,
				SecurityGroups: pulumi.StringArray{
					webSecurityGroup.ID(),
				},
			},
			LoadBalancers: ecs.ServiceLoadBalancerArray{
				&ecs.ServiceLoadBalancerArgs{
					TargetGroupArn: webTargetGroup.Arn,
					ContainerName:  pulumi.String("my-app"),
					ContainerPort:  pulumi.Int(80),
				},
			},
		}, pulumi.DependsOn([]pulumi.Resource{
			webListener,
		}))
		if err != nil {
			return err
		}
		ctx.Export("url", webLoadBalancer.DnsName)
		return nil
	})
}
