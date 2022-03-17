// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Linq;
using YamlDotNet.Core;
using YamlDotNet.Core.Events;
using YamlDotNet.Serialization;

namespace Pulumi.Automation.Serialization.Yaml
{
    internal class ProjectRuntimeOptionsYamlConverter : IYamlTypeConverter
    {
        private static readonly Type _type = typeof(ProjectRuntimeOptions);
        private static readonly List<string> _propertyNames = typeof(ProjectRuntimeOptions).GetProperties().Select(x => x.Name).ToList();
        private static readonly Dictionary<string, Func<Scalar, string, Type, object?>> _readers =
            new Dictionary<string, Func<Scalar, string, Type, object?>>(StringComparer.OrdinalIgnoreCase)
            {
                [nameof(ProjectRuntimeOptions.Binary)] = (x, p, t) => x.Value,
                [nameof(ProjectRuntimeOptions.TypeScript)] = (x, p, t) => x.ReadBoolean(p, t),
                [nameof(ProjectRuntimeOptions.VirtualEnv)] = (x, p, t) => x.Value,
            };

        public bool Accepts(Type type) => type == _type;

        public object ReadYaml(IParser parser, Type type)
        {
            if (!parser.TryConsume<MappingStart>(out _))
                throw new YamlException($"Unable to deserialize [{type.FullName}]. Expecting object.");

            var values = _propertyNames.ToDictionary(x => x, x => (object?)null, StringComparer.OrdinalIgnoreCase);

            do
            {
                if (!parser.TryConsume<Scalar>(out var propertyNameScalar))
                    throw new YamlException($"Unable to deserialize [{type.FullName}]. Expecting a property name.");

                if (!_readers.TryGetValue(propertyNameScalar.Value, out var readerFunc))
                    throw new YamlException($"Unable to deserialize [{type.FullName}]. Invalid property [{propertyNameScalar.Value}].");

                if (!parser.TryConsume<Scalar>(out var propertyValueScalar))
                    throw new YamlException($"Unable to deserialize [{type.FullName}]. Expecting a scalar value for [{propertyNameScalar.Value}].");

                values[propertyNameScalar.Value] = readerFunc(propertyValueScalar, propertyNameScalar.Value, type);
            }
            while (!parser.Accept<MappingEnd>(out _));

            parser.MoveNext(); // read final MappingEnd event
            return new ProjectRuntimeOptions
            {
                Binary = (string?)values[nameof(ProjectRuntimeOptions.Binary)],
                TypeScript = (bool?)values[nameof(ProjectRuntimeOptions.TypeScript)],
                VirtualEnv = (string?)values[nameof(ProjectRuntimeOptions.VirtualEnv)],
            };
        }

        public void WriteYaml(IEmitter emitter, object? value, Type type)
        {
            if (!(value is ProjectRuntimeOptions options))
                return;

            if (string.IsNullOrWhiteSpace(options.Binary)
                && options.TypeScript is null
                && string.IsNullOrWhiteSpace(options.VirtualEnv))
                return;

            emitter.Emit(new MappingStart(null, null, false, MappingStyle.Block));

            if (!string.IsNullOrWhiteSpace(options.Binary))
            {
                emitter.Emit(new Scalar("binary"));
                emitter.Emit(new Scalar(options.Binary));
            }

            if (options.TypeScript != null)
            {
                emitter.Emit(new Scalar("typescript"));
                emitter.Emit(new Scalar(options.TypeScript.ToString()!.ToLower()));
            }

            if (!string.IsNullOrWhiteSpace(options.VirtualEnv))
            {
                emitter.Emit(new Scalar("virtualenv"));
                emitter.Emit(new Scalar(options.VirtualEnv));
            }

            emitter.Emit(new MappingEnd());
        }
    }
}
