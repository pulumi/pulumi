using System;
using System.Collections.Immutable;

namespace Pulumi.Azure.Storage
{
    public class Account// : CustomResource
    {
        public Output<string> Name { get; }
        public Output<string> PrimaryAccessKey { get; }
        public Output<string> PrimaryConnectionString { get; }
        
        public Account(string name, AccountArgs args = default, ResourceOptions opts = default)// : base("storage.Account", name, props(args), opts)
        {
            this.Name = Output.Create(name + "abc123de");
            this.PrimaryAccessKey = Output.Create("klsad5jfoaw2iejfaowfoi3wewae==");
            this.PrimaryConnectionString = this.PrimaryAccessKey.Apply(key => $"Blabla={key}");
            Console.WriteLine($"    └─ storage.Account        {name,-11} created");
        }
    }

    public class AccountArgs
    {
        public Input<string> ResourceGroupName { get; set; }
        public Input<string> Location { get; set; }
        public Input<string> AccountReplicationType { get; set; }
        public Input<string> AccountTier { get; set; }
        public Input<string> AccountKind { get; set; }
    }
}
