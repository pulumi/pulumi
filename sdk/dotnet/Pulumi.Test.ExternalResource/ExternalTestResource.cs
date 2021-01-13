namespace Pulumi.Test.ExternalResource
{
    [ResourceType("test:index/ExternalResource", "1.0.0")]
    public class ExternalTestResource : CustomResource
    {
        public ExternalTestResource(string type, string name, ResourceArgs args, CustomResourceOptions options = null)
            : base(type, name, args, options)
        {
        }
    }
}
