// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Text.Json;
using Google.Protobuf.WellKnownTypes;

namespace Pulumi.Serialization
{
    internal static class Converter
    {
        public static OutputData<T> ConvertValue<T>(string context, Value value)
        {
            var (data, isKnown, isSecret) = ConvertValue(context, value, typeof(T));
            return new OutputData<T>((T)data!, isKnown, isSecret);
        }

        public static OutputData<object?> ConvertValue(string context, Value value, System.Type targetType)
        {
            CheckTargetType(context, targetType, new HashSet<System.Type>());

            var (deserialized, isKnown, isSecret) = Deserializer.Deserialize(value);
            var converted = ConvertObject(context, deserialized, targetType);

            return new OutputData<object?>(converted, isKnown, isSecret);
        }

        private static object? ConvertObject(string context, object? val, System.Type targetType)
        {
            var (result, exception) = TryConvertObject(context, val, targetType);
            if (exception != null)
                throw exception;

            return result;
        }

        private static (object?, InvalidOperationException?) TryConvertObject(string context, object? val, System.Type targetType)
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
                    return (null, null);
                }

                if (targetType.IsValueType)
                {
                    return (Activator.CreateInstance(targetType), null);
                }

                // for all other types, can just return the null value right back out as a legal
                // reference type value.
                return (null, null);
            }

            // We're not null and we're converting to Nullable<T>, just convert our value to be a T.
            if (targetIsNullable)
                return TryConvertObject(context, val, targetType.GenericTypeArguments.Single());

            if (targetType == typeof(string))
                return TryEnsureType<string>(context, val);

            if (targetType == typeof(bool))
                return TryEnsureType<bool>(context, val);

            if (targetType == typeof(double))
                return TryEnsureType<double>(context, val);

            if (targetType == typeof(int))
            {
                var (d, exception) = TryEnsureType<double>(context, val);
                if (exception != null)
                    return (null, exception);

                return ((int)d, exception);
            }

            if (targetType == typeof(Asset))
                return TryEnsureType<Asset>(context, val);

            if (targetType == typeof(Archive))
                return TryEnsureType<Archive>(context, val);

            if (targetType == typeof(AssetOrArchive))
                return TryEnsureType<AssetOrArchive>(context, val);

            if (targetType == typeof(JsonElement))
                return TryConvertJsonElement(context, val);

            if (targetType.IsConstructedGenericType)
            {
                if (targetType.GetGenericTypeDefinition() == typeof(Union<,>))
                    return TryConvertOneOf(context, val, targetType);

                if (targetType.GetGenericTypeDefinition() == typeof(ImmutableArray<>))
                    return TryConvertArray(context, val, targetType);
                
                if (targetType.GetGenericTypeDefinition() == typeof(ImmutableDictionary<,>))
                    return TryConvertDictionary(context, val, targetType);
                
                throw new InvalidOperationException(
                    $"Unexpected generic target type {targetType.FullName} when deserializing {context}");
            }

            if (targetType.GetCustomAttribute<OutputTypeAttribute>() == null)
                return (null, new InvalidOperationException(
                    $"Unexpected target type {targetType.FullName} when deserializing {context}"));

            var constructor = GetPropertyConstructor(targetType);
            if (constructor == null)
                return (null, new InvalidOperationException(
                    $"Expected target type {targetType.FullName} to have [{nameof(OutputConstructorAttribute)}] constructor when deserializing {context}"));

            var (dictionary, tempException) = TryEnsureType<ImmutableDictionary<string, object>>(context, val);
            if (tempException != null)
                return (null, tempException);

            var constructorParameters = constructor.GetParameters();
            var arguments = new object?[constructorParameters.Length];

            for (int i = 0, n = constructorParameters.Length; i < n; i++)
            {
                var parameter = constructorParameters[i];

                // Note: TryGetValue may not find a value here.  That can happen for things like
                // unknown vals.  That's ok.  We'll pass that through to 'Convert' and will get the
                // default value needed for the parameter type.
                dictionary!.TryGetValue(parameter.Name!, out var argValue);
                var (temp, tempException1) = TryConvertObject($"{targetType.FullName}({parameter.Name})", argValue, parameter.ParameterType);
                if (tempException1 != null)
                    return (null, tempException1);

                arguments[i] = temp;
            }

            return (constructor.Invoke(arguments), null);
        }

        private static (object?, InvalidOperationException?) TryConvertJsonElement(
            string context, object val)
        {
            using (var stream = new MemoryStream())
            {
                using (var writer = new Utf8JsonWriter(stream))
                {
                    var exception = WriteJson(context, writer, val);
                    if (exception != null)
                        return (null, exception);
                }

                stream.Position = 0;
                var document = JsonDocument.Parse(stream);
                var element = document.RootElement;
                return (element, null);
            }
        }

        private static InvalidOperationException? WriteJson(string context, Utf8JsonWriter writer, object? val)
        {
            switch (val)
            {
                case string v:
                    writer.WriteStringValue(v);
                    return null;
                case double v:
                    writer.WriteNumberValue(v);
                    return null;
                case bool v:
                    writer.WriteBooleanValue(v);
                    return null;
                case null:
                    writer.WriteNullValue();
                    return null;
                case ImmutableArray<object?> v:
                    writer.WriteStartArray();
                    foreach (var element in v)
                    {
                        var exception = WriteJson(context, writer, element);
                        if (exception != null)
                            return exception;
                    }
                    writer.WriteEndArray();
                    return null;
                case ImmutableDictionary<string, object?> v:
                    writer.WriteStartObject();
                    foreach (var (key, element) in v)
                    {
                        writer.WritePropertyName(key);
                        var exception = WriteJson(context, writer, element);
                        if (exception != null)
                            return exception;
                    }
                    writer.WriteEndObject();
                    return null;
                default:
                    return new InvalidOperationException($"Unexpected type {val.GetType().FullName} when converting {context} to {nameof(JsonElement)}");
            }
        }

        private static (T, InvalidOperationException?) TryEnsureType<T>(string context, object val)
            => val is T t ? (t, null) : (default(T)!, new InvalidOperationException($"Expected {typeof(T).FullName} but got {val.GetType().FullName} deserializing {context}"));

        private static (object?, InvalidOperationException?) TryConvertOneOf(string context, object val, System.Type oneOfType)
        {
            var firstType = oneOfType.GenericTypeArguments[0];
            var secondType = oneOfType.GenericTypeArguments[1];

            var (val1, exception1) = TryConvertObject($"{context}.AsT0", val, firstType);
            if (exception1 == null)
            {
                var fromT0Method = oneOfType.GetMethod(nameof(Union<int, int>.FromT0), BindingFlags.Public | BindingFlags.Static);
                return (fromT0Method.Invoke(null, new[] { val1 }), null);
            }

            var (val2, exception2) = TryConvertObject($"{context}.AsT1", val, secondType);
            if (exception2 == null)
            {
                var fromT1Method = oneOfType.GetMethod(nameof(Union<int, int>.FromT1), BindingFlags.Public | BindingFlags.Static);
                return (fromT1Method.Invoke(null, new[] { val2 }), null);
            }

            return (null, new InvalidOperationException($"Expected {firstType.FullName} or {secondType.FullName} but got {val.GetType().FullName} deserializing {context}"));
        }

        private static (object?, InvalidOperationException?) TryConvertArray(
            string fieldName, object val, System.Type targetType)
        {
            if (!(val is ImmutableArray<object> array))
                return (null, new InvalidOperationException(
                    $"Expected {typeof(ImmutableArray<object>).FullName} but got {val.GetType().FullName} deserializing {fieldName}"));

            var builder =
                typeof(ImmutableArray).GetMethod(nameof(ImmutableArray.CreateBuilder), Array.Empty<System.Type>())!
                                      .MakeGenericMethod(targetType.GenericTypeArguments)
                                      .Invoke(obj: null, parameters: null)!;

            var builderAdd = builder.GetType().GetMethod(nameof(ImmutableArray<int>.Builder.Add))!;
            var builderToImmutable = builder.GetType().GetMethod(nameof(ImmutableArray<int>.Builder.ToImmutable))!;

            var elementType = targetType.GenericTypeArguments.Single();
            foreach (var element in array)
            {
                var (e, exception) = TryConvertObject(fieldName, element, elementType);
                if (exception != null)
                    return (null, exception);

                builderAdd.Invoke(builder, new[] { e });
            }

            return (builderToImmutable.Invoke(builder, null), null);
        }

        private static (object?, InvalidOperationException?) TryConvertDictionary(
            string fieldName, object val, System.Type targetType)
        {
            if (!(val is ImmutableDictionary<string, object> dictionary))
                return (null, new InvalidOperationException(
                    $"Expected {typeof(ImmutableDictionary<string, object>).FullName} but got {val.GetType().FullName} deserializing {fieldName}"));

            // check if already in the form we need.  no need to convert anything.
            if (targetType == typeof(ImmutableDictionary<string, object>))
                return (val, null);

            var keyType = targetType.GenericTypeArguments[0];
            if (keyType != typeof(string))
                return (null, new InvalidOperationException(
                    $"Unexpected type {targetType.FullName} when deserializing {fieldName}. ImmutableDictionary's TKey type was not {typeof(string).FullName}"));

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
                var (e, exception) = TryConvertObject(fieldName, element, elementType);
                if (exception != null)
                    return (null, exception);

                builderAdd.Invoke(builder, new[] { key, e });
            }

            return (builderToImmutable.Invoke(builder, null), null);
        }

        public static void CheckTargetType(string context, System.Type targetType, HashSet<System.Type> seenTypes)
        {
            // types can be recursive.  So only dive into a type if it's the first time we're seeing it.
            if (!seenTypes.Add(targetType))
                return;

            if (targetType == typeof(bool) ||
                targetType == typeof(int) ||
                targetType == typeof(double) ||
                targetType == typeof(string) ||
                targetType == typeof(Asset) ||
                targetType == typeof(Archive) ||
                targetType == typeof(AssetOrArchive) ||
                targetType == typeof(JsonElement))
            {
                return;
            }

            if (targetType == typeof(ImmutableDictionary<string, object>))
            {
                // This type is what is generated for things like azure/aws tags.  It's an untyped
                // map in our original schema.  This is the only place that `object` should appear
                // as a legal value.
                return;
            }

            if (targetType.IsConstructedGenericType)
            {
                if (targetType.GetGenericTypeDefinition() == typeof(Nullable<>))
                {
                    CheckTargetType(context, targetType.GenericTypeArguments.Single(), seenTypes);
                    return;
                }
                else if (targetType.GetGenericTypeDefinition() == typeof(Union<,>))
                {
                    CheckTargetType(context, targetType.GenericTypeArguments[0], seenTypes);
                    CheckTargetType(context, targetType.GenericTypeArguments[1], seenTypes);
                    return;
                }
                else if (targetType.GetGenericTypeDefinition() == typeof(ImmutableArray<>))
                {
                    CheckTargetType(context, targetType.GenericTypeArguments.Single(), seenTypes);
                    return;
                }
                else if (targetType.GetGenericTypeDefinition() == typeof(ImmutableDictionary<,>))
                {
                    var dictTypeArgs = targetType.GenericTypeArguments;
                    if (dictTypeArgs[0] != typeof(string))
                    {
                        throw new InvalidOperationException(
$@"{context} contains invalid type {targetType.FullName}:
    The only allowed ImmutableDictionary 'TKey' type is 'String'.");
                    }

                    CheckTargetType(context, dictTypeArgs[1], seenTypes);
                    return;
                }
                else
                {
                    throw new InvalidOperationException(
$@"{context} contains invalid type {targetType.FullName}:
    The only generic types allowed are ImmutableArray<...> and ImmutableDictionary<string, ...>");
                }
            }

            var propertyTypeAttribute = targetType.GetCustomAttribute<OutputTypeAttribute>();
            if (propertyTypeAttribute == null)
            {
                throw new InvalidOperationException(
$@"{context} contains invalid type {targetType.FullName}. Allowed types are:
    String, Boolean, Int32, Double,
    Nullable<...>, ImmutableArray<...> and ImmutableDictionary<string, ...> or
    a class explicitly marked with the [{nameof(OutputTypeAttribute)}].");
            }

            var constructor = GetPropertyConstructor(targetType);
            if (constructor == null)
            {
                throw new InvalidOperationException(
$@"{targetType.FullName} had [{nameof(OutputTypeAttribute)}], but did not contain constructor marked with [{nameof(OutputConstructorAttribute)}].");
            }

            foreach (var param in constructor.GetParameters())
            {
                CheckTargetType($@"{targetType.FullName}({param.Name})", param.ParameterType, seenTypes);
            }
        }

        private static ConstructorInfo GetPropertyConstructor(System.Type outputTypeArg)
            => outputTypeArg.GetConstructors(BindingFlags.NonPublic | BindingFlags.Public | BindingFlags.Instance).FirstOrDefault(
                c => c.GetCustomAttributes<OutputConstructorAttribute>() != null);
    }
}
