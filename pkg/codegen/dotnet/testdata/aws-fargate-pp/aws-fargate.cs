using System.Collections.Generic;
using System.Linq;
using System.Text.Json;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    // Read the default VPC and public subnets, which we will use.
    var vpc = Aws.Ec2.GetVpc.Invoke(new()
    {
        Default = true,
    });

    var subnets = Aws.Ec2.GetSubnetIds.Invoke(new()
    {
        VpcId = vpc.Apply(getVpcResult => getVpcResult.Id),
    });

    // Create a security group that permits HTTP ingress and unrestricted egress.
    var webSecurityGroup = new Aws.Ec2.SecurityGroup("webSecurityGroup", new()
    {
        VpcId = vpc.Apply(getVpcResult => getVpcResult.Id),
        Egress = new[]
        {
            new Aws.Ec2.Inputs.SecurityGroupEgressArgs
            {
                Protocol = "-1",
                FromPort = 0,
                ToPort = 0,
                CidrBlocks = new[]
                {
                    "0.0.0.0/0",
                },
            },
        },
        Ingress = new[]
        {
            new Aws.Ec2.Inputs.SecurityGroupIngressArgs
            {
                Protocol = "tcp",
                FromPort = 80,
                ToPort = 80,
                CidrBlocks = new[]
                {
                    "0.0.0.0/0",
                },
            },
        },
    });

    // Create an ECS cluster to run a container-based service.
    var cluster = new Aws.Ecs.Cluster("cluster");

    // Create an IAM role that can be used by our service's task.
    var taskExecRole = new Aws.Iam.Role("taskExecRole", new()
    {
        AssumeRolePolicy = JsonSerializer.Serialize(new Dictionary<string, object?>
        {
            ["Version"] = "2008-10-17",
            ["Statement"] = new[]
            {
                new Dictionary<string, object?>
                {
                    ["Sid"] = "",
                    ["Effect"] = "Allow",
                    ["Principal"] = new Dictionary<string, object?>
                    {
                        ["Service"] = "ecs-tasks.amazonaws.com",
                    },
                    ["Action"] = "sts:AssumeRole",
                },
            },
        }),
    });

    var taskExecRolePolicyAttachment = new Aws.Iam.RolePolicyAttachment("taskExecRolePolicyAttachment", new()
    {
        Role = taskExecRole.Name,
        PolicyArn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy",
    });

    // Create a load balancer to listen for HTTP traffic on port 80.
    var webLoadBalancer = new Aws.ElasticLoadBalancingV2.LoadBalancer("webLoadBalancer", new()
    {
        Subnets = subnets.Apply(getSubnetIdsResult => getSubnetIdsResult.Ids),
        SecurityGroups = new[]
        {
            webSecurityGroup.Id,
        },
    });

    var webTargetGroup = new Aws.ElasticLoadBalancingV2.TargetGroup("webTargetGroup", new()
    {
        Port = 80,
        Protocol = "HTTP",
        TargetType = "ip",
        VpcId = vpc.Apply(getVpcResult => getVpcResult.Id),
    });

    var webListener = new Aws.ElasticLoadBalancingV2.Listener("webListener", new()
    {
        LoadBalancerArn = webLoadBalancer.Arn,
        Port = 80,
        DefaultActions = new[]
        {
            new Aws.ElasticLoadBalancingV2.Inputs.ListenerDefaultActionArgs
            {
                Type = "forward",
                TargetGroupArn = webTargetGroup.Arn,
            },
        },
    });

    // Spin up a load balanced service running NGINX
    var appTask = new Aws.Ecs.TaskDefinition("appTask", new()
    {
        Family = "fargate-task-definition",
        Cpu = "256",
        Memory = "512",
        NetworkMode = "awsvpc",
        RequiresCompatibilities = new[]
        {
            "FARGATE",
        },
        ExecutionRoleArn = taskExecRole.Arn,
        ContainerDefinitions = JsonSerializer.Serialize(new[]
        {
            new Dictionary<string, object?>
            {
                ["name"] = "my-app",
                ["image"] = "nginx",
                ["portMappings"] = new[]
                {
                    new Dictionary<string, object?>
                    {
                        ["containerPort"] = 80,
                        ["hostPort"] = 80,
                        ["protocol"] = "tcp",
                    },
                },
            },
        }),
    });

    var appService = new Aws.Ecs.Service("appService", new()
    {
        Cluster = cluster.Arn,
        DesiredCount = 5,
        LaunchType = "FARGATE",
        TaskDefinition = appTask.Arn,
        NetworkConfiguration = new Aws.Ecs.Inputs.ServiceNetworkConfigurationArgs
        {
            AssignPublicIp = true,
            Subnets = subnets.Apply(getSubnetIdsResult => getSubnetIdsResult.Ids),
            SecurityGroups = new[]
            {
                webSecurityGroup.Id,
            },
        },
        LoadBalancers = new[]
        {
            new Aws.Ecs.Inputs.ServiceLoadBalancerArgs
            {
                TargetGroupArn = webTargetGroup.Arn,
                ContainerName = "my-app",
                ContainerPort = 80,
            },
        },
    }, new CustomResourceOptions
    {
        DependsOn =
        {
            webListener,
        },
    });

    return new Dictionary<string, object?>
    {
        ["url"] = webLoadBalancer.DnsName,
    };
});

