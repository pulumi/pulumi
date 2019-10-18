using System;
using System.Collections.Immutable;

namespace Pulumi.Azure.Core
{
    public class ResourceGroup //: CustomResource
    {
        public Output<string> Name { get; }

        public ResourceGroup(string name, ResourceGroupArgs args = default, ResourceOptions opts = default)// : base("core.ResourceGroup", name, props(args), opts)
        {
            this.Name = Output.Create(name + "abc123de");
            Console.WriteLine($"    └─ core.ResourceGroup     {name,-11} created");
        }
    }

    public class ResourceGroupArgs
    {
        public Input<string> Location { get; set; }
    }
}
