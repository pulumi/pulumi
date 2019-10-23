using Pulumi.Serialization;

namespace Pulumi.Azure.AppService
{
    public class Plan : CustomResource
    {
        [OutputProperty("name")]
        public Output<string> Name { get; private set; }

        public Plan(string name, PlanArgs args = default, ResourceOptions opts = default)
            : base("azure:appservice/plan:Plan", name, args, opts)
        {
        }
    }

    public class PlanArgs : ResourceArgs
    {
        public Input<string> Kind { get; set; }
        public Input<string> Location { get; set; }
        public Input<string> ResourceGroupName { get; set; }
        public Input<PlanSkuArgs> Sku { get; set; }

        protected override void AddProperties(PropertyBuilder builder)
        {
            builder.Add("kind", Kind);
            builder.Add("location", Location);
            builder.Add("resourceGroupName", ResourceGroupName);
            builder.Add("sku", Sku);
        }
    }

    public class PlanSkuArgs : ResourceArgs
    {
        public Input<string> Size { get; set; }
        public Input<string> Tier { get; set; }

        protected override void AddProperties(PropertyBuilder builder)
        {
            builder.Add("size", Size);
            builder.Add("tier", Tier);
        }
    }
}
