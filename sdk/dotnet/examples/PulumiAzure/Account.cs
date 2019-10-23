using Pulumi.Serialization;

namespace Pulumi.Azure.Storage
{
    public class Account : CustomResource
    {
        [Output("name")]
        public Output<string> Name { get; private set; }

        [Output("primaryAccessKey")]
        public Output<string> PrimaryAccessKey { get; private set; }

        [Output("primaryConnectionString")]
        public Output<string> PrimaryConnectionString { get; private set; }

        public Account(string name, AccountArgs args = default, ResourceOptions opts = default)
            : base("azure:storage/account:Account", name, args, opts)
        {
        }
    }

    public class AccountArgs : ResourceArgs
    {
        [Input("accessTier")]
        public Input<string> AccessTier { get; set; }
        [Input("accountKind")]
        public Input<string> AccountKind { get; set; }
        [Input("accountTier")]
        public Input<string> AccountTier { get; set; }
        [Input("accountReplicationType")]
        public Input<string> AccountReplicationType { get; set; }
        [Input("location")]
        public Input<string> Location { get; set; }
        [Input("resourceGroupName")]
        public Input<string> ResourceGroupName { get; set; }
    }
}
