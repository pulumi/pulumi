using Pulumi.Serialization;

namespace Pulumi.Azure.CosmosDB
{
    public class Account : CustomResource
    {
        [Output("name")]
        public Output<string> Name { get; private set; }

        public Account(string name, AccountArgs args = default, ResourceOptions opts = default)
            : base("azure:cosmosdb/account:Account", name, args, opts)
        {
        }
    }

    public class AccountArgs : ResourceArgs
    {
        [Input("consistencyPolicy")]
        public Input<AccountConsistencyPolicy> ConsistencyPolicy { get; set; }
        [Input("geoLocations")]
        public InputList<AccountGeoLocation> GeoLocations { get; set; }
        [Input("location")]
        public Input<string> Location { get; set; }
        [Input("offerType")]
        public Input<string> OfferType { get; set; }
        [Input("resourceGroupName")]
        public Input<string> ResourceGroupName { get; set; }
    }

    public class AccountGeoLocation : ResourceArgs
    {
        [Input("location")]
        public Input<string> Location { get; set; }
        [Input("failoverPriority")]
        public Input<int> FailoverPriority { get; set; }
    }

    public class AccountConsistencyPolicy : ResourceArgs
    {
        [Input("consistencyLevel")]
        public Input<string> ConsistencyLevel { get; set; }
    }
}
