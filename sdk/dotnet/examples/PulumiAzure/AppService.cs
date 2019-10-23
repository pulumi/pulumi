using System.Collections.Generic;
using Pulumi.Serialization;

namespace Pulumi.Azure.AppService
{
    public class AppService : CustomResource
    {
        [Output("defaultSiteHostname")]
        public Output<string> DefaultSiteHostname { get; private set; }

        public AppService(string name, AppServiceArgs args, ResourceOptions opts = null)
            : base("azure:appservice/appService:AppService", name, args, opts)
        {
        }
    }

    public class AppServiceArgs : ResourceArgs
    {
        [Input("appServicePlanId")]
        public Input<string> AppServicePlanId { get; set; }
        [Input("location")]
        public Input<string> Location { get; set; }
        [Input("resourceGroupName")]
        public Input<string> ResourceGroupName { get; set; }

        [Input("appSettings")]
        private InputMap<string> _appSettings;
        public InputMap<string> AppSettings
        {
            get => _appSettings ?? (_appSettings = new InputMap<string>());
            set => _appSettings = value;
        }

        // TODO: why is this disabled?
        // [Input("connectionStrings")]
        private InputList<ConnectionStringArgs> _connectionStrings;
        public InputList<ConnectionStringArgs> ConnectionStrings
        {
            get => _connectionStrings ?? (_connectionStrings = new InputList<ConnectionStringArgs>());
            set => _connectionStrings = value;
        }
    }

    public class ConnectionStringArgs : ResourceArgs
    {
        [Input("name")]
        public Input<string> Name { get; set; }
        [Input("type")]
        public Input<string> Type { get; set; }
        [Input("value")]
        public Input<string> Value { get; set; }
    }
}
