using Pulumi;
using Aws = Pulumi.Aws;
using Awsx = Pulumi.Awsx;

class MyStack : Stack
{
    public MyStack()
    {
        var cluster = new Aws.Ecs.Cluster("cluster", new Aws.Ecs.ClusterArgs
        {
        });
        var lb = new Awsx.Lb.ApplicationLoadBalancer("lb", new Awsx.Lb.ApplicationLoadBalancerArgs
        {
        });
        var nginx = new Awsx.Ecs.FargateService("nginx", new Awsx.Ecs.FargateServiceArgs
        {
            Cluster = cluster.Arn,
            TaskDefinitionArgs = new Awsx.Ecs.Inputs.FargateServiceTaskDefinitionArgs
            {
                Container = new Awsx.Ecs.Inputs.TaskDefinitionContainerDefinitionArgs
                {
                    Image = "nginx:latest",
                    Cpu = 512,
                    Memory = 128,
                    PortMappings = 
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
        this.Url = lb.LoadBalancer.Apply(loadBalancer => loadBalancer.DnsName);
    }

    [Output("url")]
    public Output<string> Url { get; set; }
}
