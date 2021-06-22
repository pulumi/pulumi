// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using System.Reflection;
using System.Text.Json;
using System.Threading.Tasks;

namespace Pulumi.Serialization
{
    internal interface IOutputCompletionSource
    {
        Type TargetType { get; }
        IOutput Output { get; }

        void TrySetException(Exception exception);
        void TrySetDefaultResult(bool isKnown);
        
        void SetStringValue(string value, bool isKnown);
        void SetValue(OutputData<object?> data);
    }

    internal class OutputCompletionSource<T> : IOutputCompletionSource
    {
        private readonly ImmutableHashSet<Resource> _resources;
        private readonly TaskCompletionSource<OutputData<T>> _taskCompletionSource;
        public readonly Output<T> Output;

        public OutputCompletionSource(Resource? resource)
        {
            _resources = resource == null ? ImmutableHashSet<Resource>.Empty : ImmutableHashSet.Create(resource);
            _taskCompletionSource = new TaskCompletionSource<OutputData<T>>();
            Output = new Output<T>(_taskCompletionSource.Task);
        }

        public Type TargetType => typeof(T);

        IOutput IOutputCompletionSource.Output => Output;

        public void SetStringValue(string value, bool isKnown)
            => _taskCompletionSource.SetResult(new OutputData<T>(
                _resources, (T)(object)value, isKnown, isSecret: false));

        public void SetValue(OutputData<object?> data)
            => _taskCompletionSource.SetResult(new OutputData<T>(
                _resources.Union(data.Resources), (T)data.Value!, data.IsKnown, data.IsSecret));

        public void TrySetDefaultResult(bool isKnown)
            => _taskCompletionSource.TrySetResult(new OutputData<T>(
                _resources, default!, isKnown, isSecret: false));

        public void TrySetException(Exception exception)
            => _taskCompletionSource.TrySetException(exception);
    }

    internal static class OutputCompletionSource
    {
        public static ImmutableDictionary<string, IOutputCompletionSource> InitializeOutputs(Resource resource)
        {
            var query = from property in resource.GetType().GetProperties(BindingFlags.Public | BindingFlags.Instance)
                        let attr = property.GetCustomAttribute<OutputAttribute>()
                        where attr != null
                        select (property, attrName: attr?.Name);

            var result = ImmutableDictionary.CreateBuilder<string, IOutputCompletionSource>();
            foreach (var (prop, attrName) in query.ToList())
            {
                var propType = prop.PropertyType;
                var propFullName = $"[Output] {resource.GetType().FullName}.{prop.Name}";
                if (!propType.IsConstructedGenericType ||
                    propType.GetGenericTypeDefinition() != typeof(Output<>))
                {
                    throw new InvalidOperationException($"{propFullName} was not an Output<T>");
                }

                var setMethod = prop.DeclaringType!.GetMethod("set_" + prop.Name, BindingFlags.NonPublic | BindingFlags.Public | BindingFlags.Instance);
                if (setMethod == null)
                {
                    throw new InvalidOperationException($"{propFullName} did not have a 'set' method");
                }

                var outputTypeArg = propType.GenericTypeArguments.Single();
                Converter.CheckTargetType(propFullName, outputTypeArg, new HashSet<Type>());

                var ocsType = typeof(OutputCompletionSource<>).MakeGenericType(outputTypeArg);
                var ocsContructor = ocsType.GetConstructors().Single();
                var completionSource = (IOutputCompletionSource)ocsContructor.Invoke(new object?[] { resource });

                setMethod.Invoke(resource, new object?[] { completionSource.Output });

                var outputName = attrName ?? prop.Name;
                result.Add(outputName, completionSource);
            }

            Log.Debug("Fields to assign: " + JsonSerializer.Serialize(result.Keys), resource);
            return result.ToImmutable();
        }
    }
}
