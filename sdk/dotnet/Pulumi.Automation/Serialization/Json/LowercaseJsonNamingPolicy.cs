// Copyright 2016-2021, Pulumi Corporation

using System.Text.Json;

namespace Pulumi.Automation.Serialization.Json
{
    internal class LowercaseJsonNamingPolicy : JsonNamingPolicy
    {
        public override string ConvertName(string name)
            => name.ToLower();
    }
}
