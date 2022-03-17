// Copyright 2016-2021, Pulumi Corporation

using System;
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

            // confirm it is an object
            if (!parser.TryConsume<MappingStart>(out _))
                throw new YamlException($"Unable to deserialize [{type.FullName}]. Expecting object if not plain string.");

            // get first property name
            if (!parser.TryConsume<Scalar>(out var firstPropertyName))
                throw new YamlException($"Unable to deserialize [{type.FullName}]. Expecting first property name inside object.");

            // check if secure string
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
            throw new NotSupportedException("Automation API does not currently support deserializing complex objects from stack settings.");
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
