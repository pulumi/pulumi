using Pulumi.Serialization;

namespace Pulumi.Azure.AppService
{
    public class Plan : CustomResource
    {
        [Output("name")]
        public Output<string> Name { get; private set; }

        public Plan(string name, PlanArgs args = default, ResourceOptions opts = default)
            : base("azure:appservice/plan:Plan", name, args, opts)
        {
        }
    }

    public class PlanArgs : ResourceArgs
    {
        [Input("kind")]
        public Input<string> Kind { get; set; }
        [Input("location")]
        public Input<string> Location { get; set; }
        [Input("resourceGroupName")]
        public Input<string> ResourceGroupName { get; set; }
        [Input("sku")]
        public Input<PlanSkuArgs> Sku { get; set; }
    }

    public class PlanSkuArgs : ResourceArgs
    {
        [Input("size")]
        public Input<string> Size { get; set; }
        [Input("tier")]
        public Input<string> Tier { get; set; }
    }
}
