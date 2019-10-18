using System;
using System.Collections.Generic;
using System.Collections.Immutable;

namespace Pulumi.Azure.AppService
{
    public class AppService //: CustomResource
    {
        public Output<string> Name { get; }
        public Output<string> DefaultSiteHostname { get; }        

        public AppService(string name, AppServiceArgs args = default, ResourceOptions opts = default)// : base("appservice.AppService", name, props(args), opts)
        {
            this.Name = Output.Create(name + "abc123de");
            this.DefaultSiteHostname = this.Name.Apply(name => $"{name}.azurewebsites.net");
            Console.WriteLine($"    └─ appservice.AppService  {name, -11} created");
        }
    }

    public class AppServiceArgs
    {
        public Input<string> Location { get; set; }
        public Input<string> ResourceGroupName { get; set; }
        public Input<string> AppServicePlanId { get; set; }
        private InputMap<string, string> _appSettings;
        public InputMap<string, string> AppSettings
        {
            get => _appSettings ?? (_appSettings = new Dictionary<string, string>());
            set => _appSettings = value;
        }
        private InputList<ConnectionStringArgs>? _connectionStrings;
        public InputList<ConnectionStringArgs> ConnectionStrings
        {
            get => _connectionStrings ?? (_connectionStrings = new List<ConnectionStringArgs>());
            set => _connectionStrings = value;
        }
    }

    public class ConnectionStringArgs
    {
        public Input<string> Name { get; set; }
        public Input<string> Type { get; set; }
        public Input<string> Value { get; set; }
    }
}
