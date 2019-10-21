using Pulumi.Rpc;
using System;
using System.Collections.Generic;
using System.Collections.Immutable;

namespace Pulumi.Azure.AppService
{
    public class Plan : CustomResource
    {
        public Output<string> Id1 => Id.Apply(id =>
        {
            var v = id.ToString();
            return v.Substring(3, v.Length-4);
        });

        [ResourceField("name")]
        private readonly StringOutputCompletionSource _name;
        public Output<string> Name1 => _name.Output;


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

            Input<Dictionary<string, Input<string>>> dict = 
                Sku.ToOutput()
                .Apply(sku => 
                    new Dictionary<string, Input<string>>
                    {
                        { "tier",  sku.Tier },
                        { "size",  sku.Size },
                    });
            builder.Add("sku", dict);
        }
    }

    public class PlanSkuArgs
    {
        public Input<string> Tier { get; set; }
        public Input<string> Size { get; set; }
    }
}
