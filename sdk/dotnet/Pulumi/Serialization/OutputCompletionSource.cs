// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Immutable;
using System.Linq;
using System.Reflection;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Newtonsoft.Json.Linq;

namespace Pulumi.Serialization
{
    internal interface IOutputCompletionSource
    {
        System.Type TargetType { get; }
        IOutput Output { get; }

        void TrySetException(Exception exception);
        void TrySetDefaultResult(bool isKnown);
        
        void SetStringValue(string value, bool isKnown);
        void SetValue(string context, Value value);
    }

    internal class OutputCompletionSource<T> : IOutputCompletionSource
    {
        private readonly TaskCompletionSource<OutputData<T>> _taskCompletionSource;
        public readonly Output<T> Output;

        public OutputCompletionSource(Resource? resource)
        {
            _taskCompletionSource = new TaskCompletionSource<OutputData<T>>();
            Output = new Output<T>(
                resource == null ? ImmutableHashSet<Resource>.Empty : ImmutableHashSet.Create(resource),
                _taskCompletionSource.Task);
        }

        public System.Type TargetType => typeof(T);

        IOutput IOutputCompletionSource.Output => Output;

        public void SetStringValue(string value, bool isKnown)
            => _taskCompletionSource.SetResult(new OutputData<T>((T)(object)value, isKnown, isSecret: false));

        public void SetValue(string context, Value value)
        {
            var (deserialized, isKnown, isSecret) = Deserializers.GenericDeserializer(value);
            var converted = OutputCompletionSource.Convert(context, deserialized, this.TargetType);
            _taskCompletionSource.SetResult(new OutputData<T>((T)converted!, isKnown, isSecret));
        }

        public void TrySetDefaultResult(bool isKnown)
            => _taskCompletionSource.TrySetResult(new OutputData<T>(default!, isKnown, isSecret: false));

        public void TrySetException(Exception exception)
            => _taskCompletionSource.TrySetException(exception);
    }

    internal static class OutputCompletionSource
    {
        public static ImmutableDictionary<string, IOutputCompletionSource> GetSources(Resource resource)
        {
            var name = resource.GetResourceName();
            var type = resource.GetResourceType();

            var query = from property in resource.GetType().GetProperties(BindingFlags.Public | BindingFlags.Instance)
                        let attr = property.GetCustomAttribute<OutputAttribute>()
                        where attr != null
                        select (property, attr);

            var result = ImmutableDictionary.CreateBuilder<string, IOutputCompletionSource>();
            foreach (var (prop, attr) in query.ToList())
            {
                var propType = prop.PropertyType;
                var propFullName = $"[Output] {resource.GetType().FullName}.{prop.Name}";
                if (!propType.IsConstructedGenericType &&
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
                CheckTypeIsDeserializable(propFullName, outputTypeArg);

                var ocsType = typeof(OutputCompletionSource<>).MakeGenericType(outputTypeArg);
                var ocsContructor = ocsType.GetConstructors().Single();
                var completionSource = (IOutputCompletionSource)ocsContructor.Invoke(new[] { resource });

                setMethod.Invoke(resource, new[] { completionSource.Output });
                result.Add(attr.Name, completionSource);
            }

            Log.Debug("Fields to assign: " + new JArray(result.Keys), resource);
            return result.ToImmutable();
        }

        public static void CheckTypeIsDeserializable(string fullName, System.Type outputTypeArg)
        {
            if (outputTypeArg == typeof(bool) ||
                outputTypeArg == typeof(int) ||
                outputTypeArg == typeof(double) ||
                outputTypeArg == typeof(string))
            {
                return;
            }

            if (outputTypeArg.IsConstructedGenericType)
            {
                if (outputTypeArg.GetGenericTypeDefinition() == typeof(Nullable<>))
                {
                    CheckTypeIsDeserializable(fullName, outputTypeArg.GenericTypeArguments.Single());
                    return;
                }
                else if (outputTypeArg.GetGenericTypeDefinition() == typeof(ImmutableArray<>))
                {
                    CheckTypeIsDeserializable(fullName, outputTypeArg.GenericTypeArguments.Single());
                    return;
                }
                else if (outputTypeArg.GetGenericTypeDefinition() == typeof(ImmutableDictionary<,>))
                {
                    var dictTypeArgs = outputTypeArg.GenericTypeArguments;
                    if (dictTypeArgs[0] != typeof(string))
                    {
                        throw new InvalidOperationException(
    $@"{fullName} contains invalid type {outputTypeArg.FullName}:
    The only allowed ImmutableDictionary 'TKey' type is 'String'.");
                    }

                    CheckTypeIsDeserializable(fullName, dictTypeArgs[1]);
                    return;
                }
                else
                {
                    throw new InvalidOperationException(
    $@"{fullName} contains invalid type {outputTypeArg.FullName}:
    The only generic types allowed are ImmutableArray<...> and ImmutableDictionary<string, ...>");
                }
            }

            var propertyTypeAttribute = outputTypeArg.GetCustomAttribute<OutputTypeAttribute>();
            if (propertyTypeAttribute == null)
            {
                throw new InvalidOperationException(
    $@"{fullName} contains invalid type {outputTypeArg.FullName}. Allowed types are:
    String, Boolean, Int32, Double,
    Nullable<...>, ImmutableArray<...> and ImmutableDictionary<string, ...> or
    a class explicitly marked with the [PropertyType] attribute.");
            }

            var constructor = GetPropertyConstructor(outputTypeArg);
            if (constructor == null)
            {
                throw new InvalidOperationException(
    $@"{outputTypeArg.FullName} had [PropertyType] attribute, but did not contain constructor marked with [PropertyConstructor].");
            }

            foreach (var param in constructor.GetParameters())
            {
                CheckTypeIsDeserializable($@"{outputTypeArg.FullName}(${param.Name})", param.ParameterType);
            }
        }

        private static ConstructorInfo GetPropertyConstructor(System.Type outputTypeArg)
            => outputTypeArg.GetConstructors(BindingFlags.NonPublic | BindingFlags.Public | BindingFlags.Instance).FirstOrDefault(
                c => c.GetCustomAttributes<OutputConstructorAttribute>() != null);

        public static object? Convert(string fieldName, object? val, System.Type targetType)
        {
            var targetIsNullable = targetType.IsGenericType && targetType.GetGenericTypeDefinition() == typeof(Nullable<>);

            // Note: 'null's can enter the system as the representation of an 'unknown' value.
            // Before calling 'Convert' we will have already lifted the 'IsKnown' bit out, but we
            // will be passing null around as a value.
            if (val == null)
            {
                if (targetIsNullable)
                {
                    // A 'null' value coerces to a nullable null.
                    return null;
                }

                if (targetType.IsValueType)
                {
                    return Activator.CreateInstance(targetType);
                }

                // for all other types, can just return the null value right back out as a legal
                // reference type value.
                return null;
            }

            // We're not null and we're converting to Nullable<T>, just convert our value to be a T.
            if (targetIsNullable)
            {
                return Convert(fieldName, val, targetType.GenericTypeArguments.Single());
            }

            if (targetType == typeof(string))
            {
                if (!(val is string))
                {
                    throw new InvalidOperationException(
                        $"Expected {typeof(string).FullName} but got {val.GetType().FullName} deserializing {fieldName}");
                }

                return val;
            }

            if (targetType == typeof(bool))
            {
                if (!(val is bool))
                {
                    throw new InvalidOperationException(
                        $"Expected {typeof(bool).FullName} but got {val.GetType().FullName} deserializing {fieldName}");
                }

                return val;
            }

            if (targetType == typeof(double))
            {
                if (!(val is double))
                {
                    throw new InvalidOperationException(
                        $"Expected {typeof(double).FullName} but got {val.GetType().FullName} deserializing {fieldName}");
                }

                return val;
            }

            if (targetType == typeof(int))
            {
                if (!(val is double d))
                {
                    throw new InvalidOperationException(
                        $"Expected {typeof(double).FullName} but got {val.GetType().FullName} deserializing {fieldName}");
                }

                return (int)d;
            }

            if (targetType.IsConstructedGenericType)
            {
                if (targetType.GetGenericTypeDefinition() == typeof(ImmutableArray<>))
                {
                    return ConvertArray(fieldName, val, targetType);
                }
                else if (targetType.GetGenericTypeDefinition() == typeof(ImmutableDictionary<,>))
                {
                    return ConvertDictionary(fieldName, val, targetType);
                }
                else
                {
                    throw new InvalidOperationException(
                        $"Unexpected generic target type {targetType.FullName} when deserializing {fieldName}");
                }
            }

            if (targetType.GetCustomAttribute<OutputTypeAttribute>() == null)
            {
                throw new InvalidOperationException(
                    $"Unexpected target type {targetType.FullName} when deserializing {fieldName}");
            }

            var constructor = GetPropertyConstructor(targetType);
            if (constructor == null)
            {
                throw new InvalidOperationException(
                    $"Expected target type {targetType.FullName} to have [PropertyConstructor] constructor when deserializing {fieldName}");
            }

            if (!(val is ImmutableDictionary<string, object> dictionary))
            {
                throw new InvalidOperationException(
    $"Expected {typeof(ImmutableDictionary<string, object>).FullName} but got {val.GetType().FullName} deserializing {fieldName}");
            }

            var constructorParameters = constructor.GetParameters();
            var arguments = new object?[constructorParameters.Length];

            for (int i = 0, n = constructorParameters.Length; i < n; i++)
            {
                var parameter = constructorParameters[i];

                // Note: TryGetValue may not find a value here.  That can happen for things like
                // unknown vals.  That's ok.  We'll pass that through to 'Convert' and will get the
                // default value needed for the parameter type.
                dictionary.TryGetValue(parameter.Name!, out var argValue);
                arguments[i] = Convert($"{targetType.FullName}({parameter.Name})", argValue, parameter.ParameterType);
            }

            return constructor.Invoke(arguments);
        }

        private static object ConvertArray(string fieldName, object val, System.Type targetType)
        {
            if (!(val is ImmutableArray<object> array))
            {
                throw new InvalidOperationException(
                    $"Expected {typeof(ImmutableArray<object>).FullName} but got {val.GetType().FullName} deserializing {fieldName}");
            }

            var builder =
                typeof(ImmutableArray).GetMethod(nameof(ImmutableArray.CreateBuilder), Array.Empty<System.Type>())!
                                      .MakeGenericMethod(targetType.GenericTypeArguments)
                                      .Invoke(obj: null, parameters: null)!;

            var builderAdd = builder.GetType().GetMethod(nameof(ImmutableArray<int>.Builder.Add))!;
            var builderToImmutable = builder.GetType().GetMethod(nameof(ImmutableArray<int>.Builder.ToImmutable))!;

            var elementType = targetType.GenericTypeArguments.Single();
            foreach (var element in array)
            {
                builderAdd.Invoke(builder, new[] { Convert(fieldName, element, elementType) });
            }

            return builderToImmutable.Invoke(builder, null)!;
        }

        private static object ConvertDictionary(
            string fieldName, object val, System.Type targetType)
        {
            if (!(val is ImmutableDictionary<string, object> dictionary))
            {
                throw new InvalidOperationException(
                    $"Expected {typeof(ImmutableDictionary<string, object>).FullName} but got {val.GetType().FullName} deserializing {fieldName}");
            }

            var keyType = targetType.GenericTypeArguments[0];
            if (keyType != typeof(string))
            {
                throw new InvalidOperationException(
                    $"Unexpected type {targetType.FullName} when deserializing {fieldName}. ImmutableDictionary's TKey type was not {typeof(string).FullName}");
            }

            var builder =
                typeof(ImmutableDictionary).GetMethod(nameof(ImmutableDictionary.CreateBuilder), Array.Empty<System.Type>())!
                                           .MakeGenericMethod(targetType.GenericTypeArguments)
                                           .Invoke(obj: null, parameters: null)!;

            // var b = ImmutableDictionary.CreateBuilder<string, object>().Add()

            var builderAdd = builder.GetType().GetMethod(nameof(ImmutableDictionary<string, object>.Builder.Add), targetType.GenericTypeArguments)!;
            var builderToImmutable = builder.GetType().GetMethod(nameof(ImmutableDictionary<string, object>.Builder.ToImmutable))!;

            var elementType = targetType.GenericTypeArguments[1];
            foreach (var (key, element) in dictionary)
            {
                builderAdd.Invoke(builder, new[] { key, Convert(fieldName, element, elementType) });
            }

            return builderToImmutable.Invoke(builder, null)!;
        }
    }
}
