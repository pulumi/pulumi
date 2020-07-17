// Copyright 2016-2020, Pulumi Corporation

namespace Pulumi.Tests.Mocks
{
    public partial class Instance : Pulumi.CustomResource
    {
        [Output("publicIp")]
        public Output<string> PublicIp { get; private set; } = null!;

        public Instance(string name, InstanceArgs args, CustomResourceOptions? options = null)
            : base("aws:ec2/instance:Instance", name, args ?? new InstanceArgs(), options)
        {
        }
    }

    public sealed class InstanceArgs : Pulumi.ResourceArgs
    {
    }

    public class MyStack : Stack
    {
        [Output("publicIp")]
        public Output<string> PublicIp { get; private set; } = null!;

        public MyStack()
        {
            var myInstance = new Instance("instance", new InstanceArgs());
            this.PublicIp = myInstance.PublicIp;
        }
    }
}
