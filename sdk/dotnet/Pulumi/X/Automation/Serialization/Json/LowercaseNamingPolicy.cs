using System.Text.Json;

namespace Pulumi.X.Automation.Serialization.Json
{
    internal class LowercaseNamingPolicy : JsonNamingPolicy
    {
        public override string ConvertName(string name)
            => name.ToLower();
    }
}
