// Copyright 2016-2021, Pulumi Corporation

using System.Text.Json;

namespace Pulumi.Automation
{
    /// <summary>
    /// Represents the state of a stack deployment as used by
    /// ExportStackAsync and ImportStackAsync.
    /// <para/>
    /// There is no strongly typed model for the contents yet, but you
    /// can access the raw representation via the Json property.
    /// <para/>
    /// NOTE: instances may contain sensitive data (secrets).
    /// </summary>
    public sealed class StackDeployment
    {
        public static StackDeployment FromJsonString(string jsonString)
        {
            var json = JsonSerializer.Deserialize<JsonElement>(jsonString);
            var version = json.GetProperty("version").GetInt32();
            return new StackDeployment(version, json);
        }

        /// <summary>
        /// Version indicates the schema of the encoded deployment.
        /// </summary>
        public int Version { get; }

        /// <summary>
        /// JSON representation of the deployment.
        /// </summary>
        public JsonElement Json { get; }

        private StackDeployment(int version, JsonElement json)
        {
            this.Version = version;
            this.Json = json;
        }
    }
}
