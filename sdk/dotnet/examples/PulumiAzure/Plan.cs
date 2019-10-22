using Pulumi.Rpc;
using System;
using System.Collections.Generic;
using System.Collections.Immutable;

namespace Pulumi.Azure.AppService
{
    public class Plan : CustomResource
    {
        [ResourceField("name")]
        private readonly StringOutputCompletionSource _name;
        public Output<string> Name => _name.Output;


        public Plan(string name, PlanArgs args = default, ResourceOptions opts = default)
            : base("azure:appservice/plan:Plan", name, args, opts)
        {
            _name = new StringOutputCompletionSource(this);
            this.OnConstructorCompleted();
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
