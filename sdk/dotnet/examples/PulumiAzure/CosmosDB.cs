using Pulumi.Serialization;

namespace Pulumi.Azure.CosmosDB
{
    public class Account : CustomResource
    {
        [OutputProperty("name")]
        public Output<string> Name { get; private set; }

        public Account(string name, AccountArgs args = default, ResourceOptions opts = default)
            : base("azure:cosmosdb/account:Account", name, args, opts)
        {
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
