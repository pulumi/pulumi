// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Immutable;
using System.Linq;
using System.Reflection;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Newtonsoft.Json.Linq;
using Pulumi.Serialization;

namespace Pulumi
{
    public partial class Deployment
    {
        internal interface IOutputCompletionSource
        {
            System.Type TargetType { get; }
            IOutput Output { get; }
            void TrySetException(Exception exception);
            void SetDefaultResult(bool isKnown);
            void SetResult(object? value, bool isKnown, bool isSecret);
        }

        internal class OutputCompletionSource<T> : IOutputCompletionSource
        {
            private readonly TaskCompletionSource<OutputData<T>> _taskCompletionSource;
            private readonly Output<T> _output;

            public OutputCompletionSource(Resource resource)
            {
                _taskCompletionSource = new TaskCompletionSource<OutputData<T>>();
                _output = new Output<T>(ImmutableHashSet.Create(resource), _taskCompletionSource.Task);
            }

            public System.Type TargetType => typeof(T);

            public IOutput Output => _output;

            public void SetDefaultResult(bool isKnown)
                => _taskCompletionSource.SetResult(new OutputData<T>(default!, isKnown, isSecret: false));

            public void SetResult(object? value, bool isKnown, bool isSecret)
                => _taskCompletionSource.SetResult(new OutputData<T>((T)value!, isKnown, isSecret));

            public void TrySetException(Exception exception)
                => _taskCompletionSource.TrySetException(exception);
        }

        private static ImmutableDictionary<string, IOutputCompletionSource> GetOutputCompletionSources(
            Resource resource)
        {
            var name = resource.GetResourceName();
            var type = resource.GetResourceType();

            var query = from property in resource.GetType().GetProperties(BindingFlags.Public | BindingFlags.Instance)
                        let attr = property.GetCustomAttribute<PropertyAttribute>()
                        where attr != null
                        select (property, attr);

            var result = ImmutableDictionary.CreateBuilder<string, IOutputCompletionSource>();
            foreach (var (prop, attr) in query.ToList())
            {
                var propType = prop.PropertyType;
                var propFullName = $"[Property] {resource.GetType().FullName}.{prop.Name}";
                if (!propType.IsConstructedGenericType &&
                    propType.GetGenericTypeDefinition() != typeof(Output<>))
                {
                    throw new InvalidOperationException($"{propFullName} was not an Output<T>");
                }

                if (prop.SetMethod == null)
                {
                    throw new InvalidOperationException($"{propFullName} did not have a 'set' method");
                }

                var outputTypeArg = propType.GenericTypeArguments.Single();
                CheckOutputTypeArg(propFullName, outputTypeArg);

                var ocsType = typeof(OutputCompletionSource<>).MakeGenericType(outputTypeArg);
                var ocsContructor = ocsType.GetConstructors().Single();
                var completionSource = (IOutputCompletionSource)ocsContructor.Invoke(new[] { resource });

                prop.SetValue(resource, completionSource.Output);
                result.Add(attr.Name, completionSource);
            }

            Log.Debug("Fields to assign: " + new JArray(result.Keys), resource);
            return result.ToImmutable();
        }

        private static void CheckOutputTypeArg(string fullName, System.Type outputTypeArg)
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
                    CheckOutputTypeArg(fullName, outputTypeArg.GenericTypeArguments.Single());
                    return;
                }
                else if (outputTypeArg.GetGenericTypeDefinition() == typeof(ImmutableArray<>))
                {
                    CheckOutputTypeArg(fullName, outputTypeArg.GenericTypeArguments.Single());
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

                    CheckOutputTypeArg(fullName, dictTypeArgs[1]);
                    return;
                }
                else
                {
                    throw new InvalidOperationException(
    $@"{fullName} contains invalid type {outputTypeArg.FullName}:
    The only generic types allowed are ImmutableArray<...> and ImmutableDictionary<string, ...>");
                }
            }

            var propertyTypeAttribute = outputTypeArg.GetCustomAttribute<PropertyTypeAttribute>();
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
                CheckOutputTypeArg($@"{outputTypeArg.FullName}(${param.Name})", param.ParameterType);
            }
        }

        private static ConstructorInfo GetPropertyConstructor(System.Type outputTypeArg)
        {
            return outputTypeArg.GetConstructors(BindingFlags.NonPublic).FirstOrDefault(
                            c => c.GetCustomAttributes<PropertyConstructorAttribute>() != null);
        }

        /// <summary>
        /// Executes the provided <paramref name="action"/> and then completes all the 
        /// <see cref="IOutputCompletionSource"/> sources on the <paramref name="resource"/> with
        /// the results of it.
        /// </summary>
        private async Task CompleteResourceAsync(
            Resource resource, Func<Task<(string urn, string id, Struct data)>> action)
        {
            var completionSources = GetOutputCompletionSources(resource);

            // Run in a try/catch/finally so that we always resolve all the outputs of the resource
            // regardless of whether we encounter an errors computing the action.
            try
            {
                var response = await action().ConfigureAwait(false);
                completionSources["urn"].SetResult(response.urn, isKnown: true, isSecret: false);
                if (resource is CustomResource customResource)
                {
                    var id = response.id;
                    if (string.IsNullOrEmpty(id))
                    {
                        completionSources["id"].SetResult("", isKnown: false, isSecret: false);
                    }
                    else
                    {
                        completionSources["id"].SetResult(id, isKnown: true, isSecret: false);
                    }
                }

                // Go through all our output fields and lookup a corresponding value in the response
                // object.  Allow the output field to deserialize the response.
                foreach (var (fieldName, completionSource) in completionSources)
                {
                    // We process and deserialize each field (instead of bulk processing
                    // 'response.data' so that each field can have independent isKnown/isSecret
                    // values.  We do not want to bubble up isKnown/isSecret from one field to the
                    // rest.
                    if (response.data.Fields.TryGetValue(fieldName, out var value))
                    {
                        var (deserialized, isKnown, isSecret) = Deserializers.GenericDeserializer(value);

                        var converted = Convert(
                            $"{resource.GetType().FullName}.{fieldName}", deserialized, completionSource.TargetType);
                        completionSource.SetResult(converted, isKnown, isSecret);
                    }
                }
            }
            catch (Exception e)
            {
                // Mark any unresolved output properties with this exception.  That way we don't
                // leave any outstanding tasks sitting around which might cause hangs.
                foreach (var source in completionSources.Values)
                {
                    source.TrySetException(e);
                }
            }
            finally
            {
                // ensure that we've at least resolved all our completion sources.  That way we
                // don't leave any outstanding tasks sitting around which might cause hangs.
                foreach (var source in completionSources.Values)
                {
                    // Didn't get a value for this field.  Resolve it with a default value.
                    // If we're in preview, we'll consider this unknown and in a normal
                    // update we'll consider it known.
                    source.SetDefaultResult(isKnown: !this.IsDryRun);
                }
            }
        }

        private static object? Convert(string fieldName, object? val, System.Type targetType)
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

            if (targetType.GetCustomAttribute<PropertyTypeAttribute>() == null)
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
                typeof(ImmutableArray).GetMethod(nameof(ImmutableArray.CreateBuilder))!
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
                typeof(ImmutableDictionary).GetMethod(nameof(ImmutableDictionary.CreateBuilder))!
                                           .MakeGenericMethod(targetType.GenericTypeArguments)
                                           .Invoke(obj: null, parameters: null)!;

            var builderAdd = builder.GetType().GetMethod(nameof(ImmutableDictionary<string, object>.Builder.Add))!;
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