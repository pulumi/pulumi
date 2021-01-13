// Copyright 2016-2020, Pulumi Corporation

namespace Pulumi.Tests.Mocks
{
    [ResourceType("aws:ec2/instance:Instance", null)]
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

    public partial class MyCustom : Pulumi.CustomResource
    {
        [Output("instance")]
        public Output<Instance> Instance { get; private set; } = null!;

        public MyCustom(string name, MyCustomArgs args, CustomResourceOptions? options = null)
            : base("pkg:index:MyCustom", name, args ?? new MyCustomArgs(), options)
        {
        }
    }

    public sealed class MyCustomArgs : Pulumi.ResourceArgs
    {
        [Input("instance")]
        public Input<Instance>? Instance { get; set; }
    }

    public class MyStack : Stack
    {
        [Output("publicIp")]
        public Output<string> PublicIp { get; private set; } = null!;

        public MyStack()
        {
            var myInstance = new Instance("instance", new InstanceArgs());
            var myCustom = new MyCustom("mycustom", new MyCustomArgs
            {
                Instance = myInstance,
            });
            this.PublicIp = myInstance.PublicIp;
        }
    }
}
