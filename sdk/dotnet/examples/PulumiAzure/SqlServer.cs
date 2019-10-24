using System;
using System.Collections.Immutable;

namespace Pulumi.Azure.Sql
{
    public class SqlServer //: CustomResource
    {
        public Output<string> Name { get; }

        public SqlServer(string name, SqlServerArgs args = default, ResourceOptions opts = default)// : base("sql.SqlServer", name, props(args), opts)
        {
            this.Name = Output.Create(name + "abc123de");
            Console.WriteLine($"    └─ core.SqlServer         {name,-11} created");
        }
    }

    public class SqlServerArgs
    {
        public Input<string> ResourceGroupName { get; set; }
        public Input<string> Location { get; set; }
        public Input<string> AdministratorLogin { get; set; }
        public Input<string> AdministratorLoginPassword { get; set; }
        public Input<string> Version { get; set; }
    }
}
