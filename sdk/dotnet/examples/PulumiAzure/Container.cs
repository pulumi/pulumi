using System;
using System.Collections.Immutable;

namespace Pulumi.Azure.Storage
{
    public class Container// : CustomResource
    {
        public Output<string> Name { get; }

        public Container(string name, ContainerArgs args = default, ResourceOptions opts = default)// : base("storage.Container", name, props(args), opts)
        {
            this.Name = Output.Create(name + "abc123de");
            Console.WriteLine($"    └─ storage.Container      {name,-11} created");
        }
    }

    public class ContainerArgs
    {
        public Input<string> StorageAccountName { get; set; }
        public Input<string> ContainerAccessType { get; set; }
    }
}
