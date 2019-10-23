using Pulumi.Serialization;

namespace Pulumi.Azure.CosmosDB
{
    public class Account : CustomResource
    {
        [Property("name")]
        public Output<string> Name { get; private set; }

        [Property("primaryAccessKey")]
        public Output<string> PrimaryAccessKey { get; private set; }

        [Property("primaryConnectionString")]
        public Output<string> PrimaryConnectionString { get; private set; }

        public Account(string name, AccountArgs args = default, ResourceOptions opts = default)
            : base("azure:storage/account:Account", name, args, opts)
        {
        }
    }

    public class AccountArgs : ResourceArgs
    {
        public Input<string> AccessTier { get; set; }
        public Input<string> AccountKind { get; set; }
        public Input<string> AccountTier { get; set; }
        public Input<string> Location { get; set; }
        public Input<string> OfferType { get; set; }
        public Input<string> ResourceGroupName { get; set; }

        protected override void AddProperties(PropertyBuilder builder)
        {
            builder.Add("accessTier", AccessTier);
            builder.Add("accountKind", AccountKind);
            builder.Add("accountTier", AccountTier);
            builder.Add("location", Location);
            builder.Add("offerType", OfferType);
            builder.Add("resourceGroupName", ResourceGroupName);
        }
    }
}
