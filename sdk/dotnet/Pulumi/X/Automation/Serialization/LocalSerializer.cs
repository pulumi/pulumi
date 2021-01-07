using System;
using Newtonsoft.Json;
using Newtonsoft.Json.Converters;
using Newtonsoft.Json.Serialization;
using Pulumi.X.Automation.Serialization.Json;
using Pulumi.X.Automation.Serialization.Yaml;
using YamlDotNet.Serialization;
using YamlDotNet.Serialization.NamingConventions;

namespace Pulumi.X.Automation.Serialization
{
    internal class LocalSerializer
    {
        private readonly JsonSerializerSettings _jsonSettings;
        private readonly IDeserializer _yamlDeserializer;
        private readonly ISerializer _yamlSerializer;

        public LocalSerializer()
        {
            // configure json
            this._jsonSettings = BuildJsonSerializerSettings();

            // configure yaml
            this._yamlDeserializer = BuildYamlDeserializer();
            this._yamlSerializer = BuildYamlSerializer();
        }

        public T DeserializeJson<T>(string content)
        {
            var result = JsonConvert.DeserializeObject<T>(content, this._jsonSettings);
            return result ?? throw new InvalidOperationException("JSON result is null.");
        }

        public T DeserializeYaml<T>(string content)
            where T : class
            => this._yamlDeserializer.Deserialize<T>(content);

        public string SerializeJson<T>(T @object)
            => JsonConvert.SerializeObject(@object, this._jsonSettings);

        public string SerializeYaml<T>(T @object)
            where T : class
            => this._yamlSerializer.Serialize(@object);

        public static JsonSerializerSettings BuildJsonSerializerSettings()
        {
            var lowercaseNamingStrategy = new LowercaseNamingStrategy();
            var settings = new JsonSerializerSettings
            {
                ConstructorHandling = ConstructorHandling.AllowNonPublicDefaultConstructor,
                ContractResolver = new DefaultContractResolver
                {
                    NamingStrategy = lowercaseNamingStrategy,
                },
                NullValueHandling = NullValueHandling.Ignore,
            };

            settings.Converters.Add(new StringEnumConverter(lowercaseNamingStrategy, allowIntegerValues: false));
            settings.Converters.Add(new MapToModelJsonConverter<ConfigValue, ConfigValueModel>());
            settings.Converters.Add(new MapToModelJsonConverter<PluginInfo, PluginInfoModel>());
            settings.Converters.Add(new MapToModelJsonConverter<ProjectSettings, ProjectSettingsModel>());
            settings.Converters.Add(new MapToModelJsonConverter<StackSummary, StackSummaryModel>());
            settings.Converters.Add(new MapToModelJsonConverter<UpdateSummary, UpdateSummaryModel>());
            settings.Converters.Add(new ProjectRuntimeJsonConverter());
            settings.Converters.Add(new StackSettingsConfigValueJsonConverter());

            return settings;
        }

        public static IDeserializer BuildYamlDeserializer()
            => new DeserializerBuilder()
            .WithNamingConvention(LowerCaseNamingConvention.Instance)
            .IgnoreUnmatchedProperties()
            .WithTypeConverter(new ProjectRuntimeYamlConverter())
            .WithTypeConverter(new StackSettingsConfigValueYamlConverter())
            .Build();

        public static ISerializer BuildYamlSerializer()
            => new SerializerBuilder()
            .WithNamingConvention(LowerCaseNamingConvention.Instance)
            .ConfigureDefaultValuesHandling(DefaultValuesHandling.OmitNull)
            .WithTypeConverter(new ProjectRuntimeYamlConverter())
            .WithTypeConverter(new StackSettingsConfigValueYamlConverter())
            .Build();
    }
}
