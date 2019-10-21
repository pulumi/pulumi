using Pulumi.Rpc;
using System;
using System.Collections.Immutable;

namespace Pulumi.Azure.Storage
{
    public class Container : CustomResource
    {
        [ResourceField("name")]
        private readonly StringOutputCompletionSource _name;
        public Output<string> Name1 => _name.Output;

        public Container(string name, ContainerArgs args = default, ResourceOptions opts = default)
            : base("azure:storage/container:Container", name, args, opts)
        {
            _name = new StringOutputCompletionSource(this);
            this.OnConstructorCompleted();
        }
    }

    public class ContainerArgs : ResourceArgs
    {
        public Input<string> ContainerAccessType { get; set; }
        public Input<string> StorageAccountName { get; set; }

        protected override void AddProperties(PropertyBuilder builder)
        {
            builder.Add("containerAccessType", ContainerAccessType);
            builder.Add("storageAccountName", StorageAccountName);
        }
    }
}
