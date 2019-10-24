using Pulumi.Serialization;

namespace Pulumi.Azure.Core
{
    public class ResourceGroup : CustomResource
    {
        [Output("location")]
        public Output<string> Location { get; private set; }

        [Output("name")]
        public Output<string> Name { get; private set; }

        public ResourceGroup(string name, ResourceGroupArgs args = default, ResourceOptions opts = default) 
            : base("azure:core/resourceGroup:ResourceGroup", name, args, opts)
        {
        }
    }

    public class ResourceGroupArgs : ResourceArgs
    {
        [Input("location")]
        public Input<string> Location;
        [Input("name")]
        public Input<string> Name;
    }
}
