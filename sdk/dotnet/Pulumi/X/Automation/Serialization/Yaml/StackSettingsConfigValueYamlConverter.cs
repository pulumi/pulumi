using System;
using System.Collections.Generic;
using YamlDotNet.Core;
using YamlDotNet.Core.Events;
using YamlDotNet.Serialization;

namespace Pulumi.X.Automation.Serialization.Yaml
{
    internal class StackSettingsConfigValueYamlConverter : IYamlTypeConverter
    {
        private static readonly Type Type = typeof(StackSettingsConfigValue);

        public IValueSerializer? ValueSerializer { get; set; }

        public bool Accepts(Type type)
            => type == Type;

        public object? ReadYaml(IParser parser, Type type)
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
            else
            {
                // complex object, + first property
                var values = new Dictionary<string, object>
                {

                    // parse first property
                    [firstPropertyName.Value] = ParsePropertyValue(parser, type)
                };

                // parse any remaining properties
                while (!parser.Accept<MappingEnd>(out _))
                {
                    var propertyName = parser.Consume<Scalar>().Value;
                    var propertyValue = ParsePropertyValue(parser, type);
                    values[propertyName] = propertyValue;
                }

                // read final mapping end and return
                parser.MoveNext();
                return new StackSettingsConfigValue(values);
            }

            static object ParsePropertyValue(IParser parser, Type type)
            {
                if (parser.Accept<Scalar>(out var scalar))
                {
                    parser.MoveNext();
                    return scalar.Value;
                }
                else if (parser.Accept<MappingStart>(out _))
                {
                    var values = new Dictionary<string, object>();

                    parser.MoveNext();
                    while (!parser.Accept<MappingEnd>(out _))
                    {
                        var propertyName = parser.Consume<Scalar>().Value;
                        var propertyValue = ParsePropertyValue(parser, type);
                        values[propertyName] = propertyValue;
                    }

                    parser.MoveNext();
                    return values;
                }
                else
                {
                    throw new YamlException($"Unable to deserialize [{type.FullName}] as an object. Expecting either scalars or objects for properties.");
                }
            }
        }

        public void WriteYaml(IEmitter emitter, object? value, Type type)
        {
            if (this.ValueSerializer is null)
                throw new YamlException($"{nameof(ValueSerializer)} was not set on converter creation.");

            if (!(value is StackSettingsConfigValue configValue))
                return;

            // object
            if (configValue.IsObject)
            {
                this.ValueSerializer.SerializeValue(emitter, configValue.ValueObject, configValue.ValueObject!.GetType());
            }
            // secure string
            else if (configValue.IsSecure)
            {
                emitter.Emit(new MappingStart(null, null, false, MappingStyle.Block));
                emitter.Emit(new Scalar("secure"));
                emitter.Emit(new Scalar(configValue.ValueString!));
                emitter.Emit(new MappingEnd());
            }
            // plain string
            else
            {
                emitter.Emit(new Scalar(configValue.ValueString!));
            }
        }
    }
}
