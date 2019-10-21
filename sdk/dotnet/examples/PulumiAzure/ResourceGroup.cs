using Pulumi.Rpc;

namespace Pulumi.Azure.Core
{
    public class ResourceGroup : CustomResource
    {
        [ResourceField("location")]
        private readonly StringOutputCompletionSource _location;
        public Output<string> Location => _location.Output;

        [ResourceField("name")]
        private readonly StringOutputCompletionSource _name;
        public new Output<string> Name => _name.Output;

        public ResourceGroup(string name, ResourceGroupArgs args = default, ResourceOptions opts = default) 
            : base("azure:core/resourceGroup:ResourceGroup", name, args, opts)
        {
            _location = new StringOutputCompletionSource(this);
            _name = new StringOutputCompletionSource(this);
            this.OnConstructorCompleted();
        }
    }

    public class ResourceGroupArgs : ResourceArgs
    {
        public Input<string> Location;
        public Input<string> Name;

        protected override void AddProperties(PropertyBuilder builder)
        {
            builder.Add("location", Location);
            builder.Add("name", Name);
        }
    }
}
