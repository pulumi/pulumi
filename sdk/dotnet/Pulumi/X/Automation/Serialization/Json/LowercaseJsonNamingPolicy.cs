using System.Text.Json;

namespace Pulumi.X.Automation.Serialization.Json
{
    internal class LowercaseJsonNamingPolicy : JsonNamingPolicy
    {
        public override string ConvertName(string name)
            => name.ToLower();
    }
}
