using Pulumi.Serialization;

namespace Pulumi.Azure.Storage
{
    public class Container : CustomResource
    {
        [Output("name")]
        public Output<string> Name { get; private set; }

        public Container(string name, ContainerArgs args = default, ResourceOptions opts = default)
            : base("azure:storage/container:Container", name, args, opts)
        {
        }
    }

    public class ContainerArgs : ResourceArgs
    {
        [Input("containerAccessType")]
        public Input<string> ContainerAccessType { get; set; }
        [Input("storageAccountName")]
        public Input<string> StorageAccountName { get; set; }
    }
}
