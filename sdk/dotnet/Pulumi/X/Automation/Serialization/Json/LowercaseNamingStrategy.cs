using Newtonsoft.Json.Serialization;

namespace Pulumi.X.Automation.Serialization.Json
{
    internal class LowercaseNamingStrategy : NamingStrategy
    {
        protected override string ResolvePropertyName(string name)
            => name.ToLower();
    }
}
