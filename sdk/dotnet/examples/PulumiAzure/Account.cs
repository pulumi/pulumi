using Pulumi.Rpc;
using System;
using System.Collections.Immutable;

namespace Pulumi.Azure.Storage
{
    public class Account : CustomResource
    {
        [ResourceField("name")]
        private readonly StringOutputCompletionSource _name;
        public Output<string> Name => _name.Output;

        [ResourceField("primaryAccessKey")]
        private readonly StringOutputCompletionSource _primaryAccessKey;
        public Output<string> PrimaryAccessKey => _primaryAccessKey.Output;

        [ResourceField("primaryConnectionString")]
        private readonly StringOutputCompletionSource _primaryConnectionString;
        public Output<string> PrimaryConnectionString => _primaryConnectionString.Output;

        public Account(string name, AccountArgs args = default, ResourceOptions opts = default)
            : base("azure:storage/account:Account", name, args, opts)
        {
            _name = new StringOutputCompletionSource(this);
            _primaryAccessKey = new StringOutputCompletionSource(this);
            _primaryConnectionString = new StringOutputCompletionSource(this);
            this.OnConstructorCompleted();
        }
    }

    public class AccountArgs : ResourceArgs
    {
        public Input<string> AccessTier { get; set; }
        public Input<string> AccountKind { get; set; }
        public Input<string> AccountTier { get; set; }
        public Input<string> AccountReplicationType { get; set; }
        public Input<string> Location { get; set; }
        public Input<string> ResourceGroupName { get; set; }

        protected override void AddProperties(PropertyBuilder builder)
        {
            builder.Add("accessTier", AccessTier);
            builder.Add("accountKind", AccountKind);
            builder.Add("accountTier", AccountTier);
            builder.Add("accountReplicationType", AccountReplicationType);
            builder.Add("location", Location);
            builder.Add("resourceGroupName", ResourceGroupName);
        }
    }
}
