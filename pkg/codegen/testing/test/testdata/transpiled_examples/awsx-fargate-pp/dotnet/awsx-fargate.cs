using System.Collections.Generic;
using Pulumi;
using Aws = Pulumi.Aws;
using Awsx = Pulumi.Awsx;

return await Deployment.RunAsync(() => 
{
    var cluster = new Aws.Ecs.Cluster("cluster");

    var lb = new Awsx.Lb.ApplicationLoadBalancer("lb");

    var nginx = new Awsx.Ecs.FargateService("nginx", new()
    {
        Cluster = cluster.Arn,
        TaskDefinitionArgs = new Awsx.Ecs.Inputs.FargateServiceTaskDefinitionArgs
        {
            Container = new Awsx.Ecs.Inputs.TaskDefinitionContainerDefinitionArgs
            {
                Image = "nginx:latest",
                Cpu = 512,
                Memory = 128,
                PortMappings = new[]
                {
                    new Awsx.Ecs.Inputs.TaskDefinitionPortMappingArgs
                    {
                        ContainerPort = 80,
                        TargetGroup = lb.DefaultTargetGroup,
                    },
                },
            },
        },
    });

    return new Dictionary<string, object?>
    {
        ["url"] = lb.LoadBalancer.Apply(loadBalancer => loadBalancer.DnsName),
    };
});

