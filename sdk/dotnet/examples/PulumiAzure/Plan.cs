using System;
using System.Collections.Immutable;

namespace Pulumi.Azure.AppService
{
    public class Plan //: CustomResource
    {
        public Output<string> Name { get; }
        public Output<string> Id { get; }

        public Plan(string name, PlanArgs args = default, ResourceOptions opts = default)// : base("appservice.Plan", name, props(args), opts)
        {
            this.Name = Output.Create(name + "abc123de");
            this.Id = this.Name.Apply(name => $"/subscription/123/resourceGroup/456/web.farm/{name}");
            Console.WriteLine($"    └─ appservice.Plan        {name, -11} created");
        }
    }

    public class PlanArgs
    {
        public Input<string> Location { get; set; }
        public Input<string> ResourceGroupName { get; set; }
        public Input<string> Kind { get; set; }
        public Input<PlanSkuArgs> Sku { get; set; }
    }

    public class PlanSkuArgs
    {
        public Input<string> Tier { get; set; }
        public Input<string> Size { get; set; }
    }
}
