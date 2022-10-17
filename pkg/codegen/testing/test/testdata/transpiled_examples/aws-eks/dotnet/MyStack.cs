using Pulumi;
using Aws = Pulumi.Aws;
using Eks = Pulumi.Eks;

class MyStack : Stack
{
    public MyStack()
    {
        var vpcId = Output.Create(Aws.Ec2.GetVpc.InvokeAsync(new Aws.Ec2.GetVpcArgs
        {
            Default = true,
        })).Apply(invoke => invoke.Id);
        var subnetIds = Output.Create(Aws.Ec2.GetSubnetIds.InvokeAsync(new Aws.Ec2.GetSubnetIdsArgs
        {
            VpcId = vpcId,
        })).Apply(invoke => invoke.Ids);
        var cluster = new Eks.Cluster("cluster", new Eks.ClusterArgs
        {
            VpcId = vpcId,
            SubnetIds = subnetIds,
            InstanceType = "t2.medium",
            DesiredCapacity = 2,
            MinSize = 1,
            MaxSize = 2,
        });
        this.Kubeconfig = cluster.Kubeconfig;
    }

    [Output("kubeconfig")]
    public Output<string> Kubeconfig { get; set; }
}
