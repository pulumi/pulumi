// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Text.Json;
using YamlDotNet.Core;
using YamlDotNet.Core.Events;
using YamlDotNet.Serialization;

namespace Pulumi.Automation.Serialization.Yaml
{
    internal class StackSettingsConfigValueYamlConverter : IYamlTypeConverter
    {
        private static readonly Type _type = typeof(StackSettingsConfigValue);

        public bool Accepts(Type type) => type == _type;

        public object ReadYaml(IParser parser, Type type)
        {
            // check if plain string
            if (parser.Accept<Scalar>(out var stringValue))
            {
                parser.MoveNext();
                return new StackSettingsConfigValue(stringValue.Value, false);
            }

            var deserializer = new Deserializer();

            // check whether it is an object with a single property called "secure"
            // this means we have a secret value serialized into the value
            if (parser.TryConsume<MappingStart>(out var _))
            {
                var dictionaryFromYaml = new Dictionary<string, object?>();

                // in which case, check whether it is a secure value
                while (parser.TryConsume<Scalar>(out var firstPropertyName))
                {
                    if (string.Equals("Secure", firstPropertyName.Value, StringComparison.OrdinalIgnoreCase))
                    {
                        // secure string
                        if (!parser.TryConsume<Scalar>(out var securePropertyValue))
                            throw new YamlException($"Unable to deserialize [{type.FullName}] as a secure string. Expecting a string secret.");

                         // needs to be 1 mapping end and then return
                        parser.Require<MappingEnd>();
                        parser.MoveNext();
                        return new StackSettingsConfigValue(securePropertyValue.Value, true);
                    }

                    // not a secure string, so we need to add first value to the dictionary
                    dictionaryFromYaml.Add(firstPropertyName.Value, deserializer.Deserialize<object?>(parser));
                }

                parser.Require<MappingEnd>();
                parser.MoveNext();
                // serialize the dictionary back into the value as JSON
                var serializedDictionary = JsonSerializer.Serialize(dictionaryFromYaml);
                return new StackSettingsConfigValue(serializedDictionary, false);
            }

            // for anything else, i.e. arrays, parse the contents as is and serialize it a JSON string
            var deserializedFromYaml = deserializer.Deserialize<object?>(parser);
            var serializedToJson = JsonSerializer.Serialize(deserializedFromYaml);
            return new StackSettingsConfigValue(serializedToJson, false);
        }

        public void WriteYaml(IEmitter emitter, object? value, Type type)
        {
            if (!(value is StackSettingsConfigValue configValue))
                return;

            // secure string
            if (configValue.IsSecure)
            {
                emitter.Emit(new MappingStart(null, null, false, MappingStyle.Block));
                emitter.Emit(new Scalar("secure"));
                emitter.Emit(new Scalar(configValue.Value));
                emitter.Emit(new MappingEnd());
            }
            // plain string
            else
            {
                emitter.Emit(new Scalar(configValue.Value));
            }
        }
    }
}
