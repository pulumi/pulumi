using Pulumi.Serialization;

namespace Pulumi.Azure.CosmosDB
{
    public class Account : CustomResource
    {
        [ResourceField("name")]
        private readonly StringOutputCompletionSource _name;
        public Output<string> Name => _name.Output;

        public Account(string name, AccountArgs args = default, ResourceOptions opts = default)
            : base("azure:cosmosdb/account:Account", name, args, opts)
        {
            _name = new StringOutputCompletionSource(this);
            this.OnConstructorCompleted();
        }
    }

    public class AccountArgs : ResourceArgs
    {
        public Input<AccountConsistencyPolicy> ConsistencyPolicy { get; set; }
        public InputList<AccountGeoLocation> GeoLocations { get; set; }
        public Input<string> Location { get; set; }
        public Input<string> OfferType { get; set; }
        public Input<string> ResourceGroupName { get; set; }

        protected override void AddProperties(PropertyBuilder builder)
        {
            builder.Add("consistencyPolicy", ConsistencyPolicy);
            builder.Add("geoLocations", GeoLocations);
            builder.Add("location", Location);
            builder.Add("offerType", OfferType);
            builder.Add("resourceGroupName", ResourceGroupName);
        }
    }

    public class AccountGeoLocation : ResourceArgs
    {
        public Input<string> Location { get; set; }
        public Input<int> FailoverPriority { get; set; }
        
        protected override void AddProperties(PropertyBuilder builder)
        {
            builder.Add("location", Location);
            builder.Add("failoverPriority", FailoverPriority);
        }
    }

    public class AccountConsistencyPolicy : ResourceArgs
    {
        public Input<string> ConsistencyLevel { get; set; }

        protected override void AddProperties(PropertyBuilder builder)
        {
            builder.Add("consistencyLevel", ConsistencyLevel);
        }
    }
}
