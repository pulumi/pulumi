using System;
using System.Collections.Immutable;

namespace Pulumi.Azure.Sql
{
    public class Database //: CustomResource
    {
        public Output<string> Name { get; }

        public Database(string name, DatabaseArgs args = default, ResourceOptions opts = default)// : base("sql.Database", name, props(args), opts)
        {
            this.Name = Output.Create(name + "abc123de");
            Console.WriteLine($"    └─ core.Database          {name,-11} created");
        }
    }

    public class DatabaseArgs
    {
        public Input<string> ResourceGroupName { get; set; }
        public Input<string> ServerName { get; set; }
        public Input<string> RequestedServiceObjectiveName { get; set; }
    }
}
