using Pulumi.Serialization;

namespace Pulumi.Azure.Core
{
    public class ResourceGroup : CustomResource
    {
        [OutputProperty("location")]
        public Output<string> Location { get; private set; }

        [OutputProperty("name")]
        public Output<string> Name { get; private set; }

        public ResourceGroup(string name, ResourceGroupArgs args = default, ResourceOptions opts = default) 
            : base("azure:core/resourceGroup:ResourceGroup", name, args, opts)
        {
        }
    }

    public class ResourceGroupArgs : ResourceArgs
    {
        public Input<string> Location;
        public Input<string> Name;

        protected override void AddProperties(PropertyBuilder builder)
        {
            builder.Add("location", Location);
            builder.Add("name", Name);
        }
    }
}
