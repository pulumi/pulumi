using System.Collections.Generic;
using Pulumi.Serialization;

namespace Pulumi.Azure.AppService
{
    public class AppService : CustomResource
    {
        [ResourceField("defaultSiteHostname")]
        private readonly StringOutputCompletionSource _defaultSiteHostname;
        public Output<string> DefaultSiteHostname => _defaultSiteHostname.Output;
        
        public AppService(string name, AppServiceArgs args, ResourceOptions opts = null)
            : base("azure:appservice/appService:AppService", name, args, opts)
        {
            _defaultSiteHostname = new StringOutputCompletionSource(this);
            this.OnConstructorCompleted();
        }
    }

    public class AppServiceArgs : ResourceArgs
    {
        public Input<string> AppServicePlanId { get; set; }
        public Input<string> Location { get; set; }
        public Input<string> ResourceGroupName { get; set; }


        private InputMap<string> _appSettings;
        public InputMap<string> AppSettings
        {
            get => _appSettings ?? (_appSettings = new Dictionary<string, string>());
            set => _appSettings = value;
        }
        private InputList<ConnectionStringArgs> _connectionStrings;
        public InputList<ConnectionStringArgs> ConnectionStrings
        {
            get => _connectionStrings ?? (_connectionStrings = new List<ConnectionStringArgs>());
            set => _connectionStrings = value;
        }

        protected override void AddProperties(PropertyBuilder builder)
        {
            builder.Add("appServicePlanId", AppServicePlanId);
            builder.Add("location", Location);
            builder.Add("resourceGroupName", ResourceGroupName);
            builder.Add("appSettings", AppSettings);
            //builder.Add("connectionStrings", ConnectionStrings);
        }
    }

    public class ConnectionStringArgs
    {
        public Input<string> Name { get; set; }
        public Input<string> Type { get; set; }
        public Input<string> Value { get; set; }
    }
}
