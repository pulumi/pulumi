// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Immutable;
using System.Linq;
using System.Reflection;
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
            CheckTargetType(context, targetType);

            var (deserialized, isKnown, isSecret) = Deserializer.Deserialize(value);
            var converted = ConvertObject(context, deserialized, targetType);

            return new OutputData<object?>(converted, isKnown, isSecret);
        }

        private static object? ConvertObject(string context, object? val, System.Type targetType)
        {
            var targetIsNullable = targetType.IsGenericType && targetType.GetGenericTypeDefinition() == typeof(Nullable<>);

            // Note: 'null's can enter the system as the representation of an 'unknown' value.
            // Before calling 'Convert' we will have already lifted the 'IsKnown' bit out, but we
            // will be passing null around as a value.
            if (val == null)
            {
                if (targetIsNullable)
                    // A 'null' value coerces to a nullable null.
                    return null;

                if (targetType.IsValueType)
                    return Activator.CreateInstance(targetType);

                // for all other types, can just return the null value right back out as a legal
                // reference type value.
                return null;
            }

            // We're not null and we're converting to Nullable<T>, just convert our value to be a T.
            if (targetIsNullable)
                return ConvertObject(context, val, targetType.GenericTypeArguments.Single());

            if (targetType == typeof(string))
                return EnsureType<string>(context, val);

            if (targetType == typeof(bool))
                return EnsureType<bool>(context, val);

            if (targetType == typeof(double))
                return EnsureType<double>(context, val);

            if (targetType == typeof(int))
                return (int)EnsureType<double>(context, val);

            if (targetType == typeof(Asset))
                return EnsureType<Asset>(context, val);

            if (targetType == typeof(Archive))
                return EnsureType<Archive>(context, val);

            if (targetType == typeof(AssetOrArchive))
                return EnsureType<AssetOrArchive>(context, val);

            if (targetType.IsConstructedGenericType)
            {
                if (targetType.GetGenericTypeDefinition() == typeof(ImmutableArray<>))
                    return ConvertArray(context, val, targetType);
                
                if (targetType.GetGenericTypeDefinition() == typeof(ImmutableDictionary<,>))
                    return ConvertDictionary(context, val, targetType);
                
                throw new InvalidOperationException(
                    $"Unexpected generic target type {targetType.FullName} when deserializing {context}");
            }

            if (targetType.GetCustomAttribute<OutputTypeAttribute>() == null)
                throw new InvalidOperationException(
                    $"Unexpected target type {targetType.FullName} when deserializing {context}");

            var constructor = GetPropertyConstructor(targetType);
            if (constructor == null)
                throw new InvalidOperationException(
                    $"Expected target type {targetType.FullName} to have [PropertyConstructor] constructor when deserializing {context}");

            var dictionary = EnsureType<ImmutableDictionary<string, object>>(context, val);

            var constructorParameters = constructor.GetParameters();
            var arguments = new object?[constructorParameters.Length];

            for (int i = 0, n = constructorParameters.Length; i < n; i++)
            {
                var parameter = constructorParameters[i];

                // Note: TryGetValue may not find a value here.  That can happen for things like
                // unknown vals.  That's ok.  We'll pass that through to 'Convert' and will get the
                // default value needed for the parameter type.
                dictionary.TryGetValue(parameter.Name!, out var argValue);
                arguments[i] = ConvertObject($"{targetType.FullName}({parameter.Name})", argValue, parameter.ParameterType);
            }

            return constructor.Invoke(arguments);
        }

        private static T EnsureType<T>(string context, object val)
            => val is T t ? t : throw new InvalidOperationException($"Expected {typeof(T).FullName} but got {val.GetType().FullName} deserializing {context}");

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
                builderAdd.Invoke(builder, new[] { ConvertObject(fieldName, element, elementType) });
            }

            return builderToImmutable.Invoke(builder, null)!;
        }

        private static object ConvertDictionary(string fieldName, object val, System.Type targetType)
        {
            if (!(val is ImmutableDictionary<string, object> dictionary))
            {
                throw new InvalidOperationException(
                    $"Expected {typeof(ImmutableDictionary<string, object>).FullName} but got {val.GetType().FullName} deserializing {fieldName}");
            }

            if (targetType == typeof(ImmutableDictionary<string, object>))
            {
                // already in the form we need.  no need to convert anything.
                return val;
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
                builderAdd.Invoke(builder, new[] { key, ConvertObject(fieldName, element, elementType) });
            }

            return builderToImmutable.Invoke(builder, null)!;
        }

        public static void CheckTargetType(string context, System.Type targetType)
        {
            if (targetType == typeof(bool) ||
                targetType == typeof(int) ||
                targetType == typeof(double) ||
                targetType == typeof(string) ||
                targetType == typeof(Asset) ||
                targetType == typeof(Archive) ||
                targetType == typeof(AssetOrArchive))
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
                    CheckTargetType(context, targetType.GenericTypeArguments.Single());
                    return;
                }
                else if (targetType.GetGenericTypeDefinition() == typeof(ImmutableArray<>))
                {
                    CheckTargetType(context, targetType.GenericTypeArguments.Single());
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

                    CheckTargetType(context, dictTypeArgs[1]);
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
    a class explicitly marked with the [PropertyType] attribute.");
            }

            var constructor = GetPropertyConstructor(targetType);
            if (constructor == null)
            {
                throw new InvalidOperationException(
$@"{targetType.FullName} had [PropertyType] attribute, but did not contain constructor marked with [PropertyConstructor].");
            }

            foreach (var param in constructor.GetParameters())
            {
                CheckTargetType($@"{targetType.FullName}({param.Name})", param.ParameterType);
            }
        }

        private static ConstructorInfo GetPropertyConstructor(System.Type outputTypeArg)
            => outputTypeArg.GetConstructors(BindingFlags.NonPublic | BindingFlags.Public | BindingFlags.Instance).FirstOrDefault(
                c => c.GetCustomAttributes<OutputConstructorAttribute>() != null);
    }
}
