// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Text.Json;

namespace Pulumi.Automation
{
    /// <summary>
    ///
    ///  Represents the state of a stack depoyment ExportStackAsync
    ///  and ImportStackAsync.
    ///
    /// </summary>
    public sealed class StackDeployment
    {
        internal static StackDeployment FromJsonString(string jsonString)
        {
            var json = JsonSerializer.Deserialize<JsonElement>(jsonString);
            var version = json.GetProperty("version").GetString();
            return new StackDeployment(version, json, jsonString);
        }

        /// <summary>
        /// Version indicates the schema of the encoded deployment.
        /// </summary>
        public string Version { get; }

        /// <summary>
        /// JSON repsresentation of the deployment.
        /// </summary>
        public JsonElement Json { get; }

        internal string JsonString { get; }

        private StackDeployment(string version, JsonElement json, string jsonString)
        {
            this.Version = version;
            this.Json = json;
            this.JsonString = jsonString;
        }
    }
}
