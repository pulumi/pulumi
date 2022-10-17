using Pulumi;
using Eks = Pulumi.Eks;

class MyStack : Stack
{
    public MyStack()
    {
        var rawkode = new Eks.Cluster("rawkode", new Eks.ClusterArgs
        {
            InstanceType = "t2.medium",
            DesiredCapacity = 2,
            MinSize = 1,
            MaxSize = 2,
        });
        var stack72 = new Eks.Cluster("stack72", new Eks.ClusterArgs
        {
            InstanceType = "t2.medium",
            DesiredCapacity = 4,
            MinSize = 1,
            MaxSize = 8,
        });
    }

}
