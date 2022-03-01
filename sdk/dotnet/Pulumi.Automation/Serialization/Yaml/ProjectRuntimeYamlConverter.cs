// Copyright 2016-2021, Pulumi Corporation

using System;
using YamlDotNet.Core;
using YamlDotNet.Core.Events;
using YamlDotNet.Serialization;

namespace Pulumi.Automation.Serialization.Yaml
{
    internal class ProjectRuntimeYamlConverter : IYamlTypeConverter
    {
        private static readonly Type _type = typeof(ProjectRuntime);
        private static readonly Type _optionsType = typeof(ProjectRuntimeOptions);

        private readonly IYamlTypeConverter _optionsConverter = new ProjectRuntimeOptionsYamlConverter();

        public bool Accepts(Type type) => type == _type;

        public object ReadYaml(IParser parser, Type type)
        {
            if (parser.TryConsume<Scalar>(out var nameValueScalar))
            {
                var runtimeName = DeserializeName(nameValueScalar, type);
                return new ProjectRuntime(runtimeName);
            }

            if (!parser.TryConsume<MappingStart>(out _))
                throw new YamlException($"Unable to deserialize [{type.FullName}]. Expecting string or object.");

            if (!parser.TryConsume<Scalar>(out var namePropertyScalar)
                || !string.Equals(nameof(ProjectRuntime.Name), namePropertyScalar.Value, StringComparison.OrdinalIgnoreCase))
                throw new YamlException($"Unable to deserialize [{type.FullName}]. Expecting runtime name property.");

            if (!parser.TryConsume<Scalar>(out var nameValueScalar2))
                throw new YamlException($"Unable to deserialize [{type.FullName}]. Runtime name property should be a string.");

            var name = DeserializeName(nameValueScalar2, type);

            // early mapping end is ok
            if (parser.Accept<MappingEnd>(out _))
            {
                parser.MoveNext(); // read final MappingEnd since Accept doesn't call MoveNext
                return new ProjectRuntime(name);
            }

            if (!parser.TryConsume<Scalar>(out var optionsPropertyScalar)
                || !string.Equals(nameof(ProjectRuntime.Options), optionsPropertyScalar.Value, StringComparison.OrdinalIgnoreCase))
                throw new YamlException($"Unable to deserialize [{type.FullName}]. Expecting runtime options property.");

            if (!parser.Accept<MappingStart>(out _))
                throw new YamlException($"Unable to deserialize [{type.FullName}]. Runtime options property should be an object.");

            var runtimeOptionsObj = this._optionsConverter.ReadYaml(parser, _optionsType);
            if (!(runtimeOptionsObj is ProjectRuntimeOptions runtimeOptions))
                throw new YamlException("There was an issue deserializing the runtime options object.");

            parser.MoveNext(); // read final MappingEnd event
            return new ProjectRuntime(name) { Options = runtimeOptions };

            static ProjectRuntimeName DeserializeName(Scalar nameValueScalar, Type type)
            {
                if (string.IsNullOrWhiteSpace(nameValueScalar.Value))
                    throw new YamlException($"A valid runtime name was not provided when deserializing [{type.FullName}].");

                if (Enum.TryParse<ProjectRuntimeName>(nameValueScalar.Value, true, out var runtimeName))
                    return runtimeName;

                throw new YamlException($"Unexpected runtime name of \"{nameValueScalar.Value}\" provided when deserializing [{type.FullName}].");
            }
        }

        public void WriteYaml(IEmitter emitter, object? value, Type type)
        {
            if (!(value is ProjectRuntime runtime))
                return;

            if (runtime.Options is null)
            {
                emitter.Emit(new Scalar(runtime.Name.ToString().ToLower()));
            }
            else
            {
                emitter.Emit(new MappingStart(null, null, false, MappingStyle.Block));

                emitter.Emit(new Scalar("name"));
                emitter.Emit(new Scalar(runtime.Name.ToString().ToLower()));

                emitter.Emit(new Scalar("options"));
                this._optionsConverter.WriteYaml(emitter, runtime.Options, _optionsType);

                emitter.Emit(new MappingEnd());
            }
        }
    }
}
