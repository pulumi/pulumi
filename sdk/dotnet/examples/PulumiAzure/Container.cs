using Pulumi.Serialization;

namespace Pulumi.Azure.Storage
{
    public class Container : CustomResource
    {
        [Property("name")]
        public Output<string> Name { get; private set; }

        public Container(string name, ContainerArgs args = default, ResourceOptions opts = default)
            : base("azure:storage/container:Container", name, args, opts)
        {
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
