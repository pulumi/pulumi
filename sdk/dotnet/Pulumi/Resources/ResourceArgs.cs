// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Immutable;
using System.Linq;
using System.Reflection;
using System.Threading.Tasks;
using Google.Protobuf;
using Pulumi.Serialization;

namespace Pulumi
{
    /// <summary>
    /// Base type for all resource argument classes.
    /// </summary>
    public abstract class ResourceArgs
    {
        public static readonly ResourceArgs Empty = new EmptyResourceArgs();

        private readonly ImmutableArray<InputInfo> _inputInfos;

        protected ResourceArgs()
        {
            var fieldQuery =
                from field in this.GetType().GetFields(BindingFlags.NonPublic | BindingFlags.Public | BindingFlags.Instance)
                let attr = field.GetCustomAttribute<InputAttribute>()
                where attr != null
                select (attr, memberName: field.Name, memberType: field.FieldType, getValue: (Func<object, object?>)field.GetValue);

            var propQuery =
                from prop in this.GetType().GetProperties(BindingFlags.NonPublic | BindingFlags.Public | BindingFlags.Instance)
                let attr = prop.GetCustomAttribute<InputAttribute>()
                where attr != null
                select (attr, memberName: prop.Name, memberType: prop.PropertyType, getValue: (Func<object, object?>)prop.GetValue);

            var all = fieldQuery.Concat(propQuery).ToList();

            foreach (var (attr, memberName, memberType, getValue) in all)
            {
                var fullName = $"[Input] {this.GetType().FullName}.{memberName}";

                if (!typeof(IInput).IsAssignableFrom(memberType))
                {
                    throw new InvalidOperationException($"{fullName} was not an Input<T>");
                }
            }

            _inputInfos = all.Select(t =>
                new InputInfo(t.attr, t.memberName, t.memberType, t.getValue)).ToImmutableArray();
        }

        internal async Task<ImmutableDictionary<string, IInput?>> ToDictionaryAsync()
        {
            var builder = ImmutableDictionary.CreateBuilder<string, IInput?>();
            foreach (var info in _inputInfos)
            {
                var fullName = $"[Input] {this.GetType().FullName}.{info.MemberName}";

                var value = (IInput?)info.GetValue(this);
                if (info.Attribute.IsRequired && value == null)
                {
                    throw new ArgumentNullException(info.MemberName, $"{fullName} is required but was not given a value");
                }

                if (info.Attribute.Json)
                {
                    value = await ConvertToJsonAsync(fullName, value).ConfigureAwait(false);
                }

                builder.Add(info.Attribute.Name, value);
            }

            return builder.ToImmutable();
        }

        private async Task<IInput?> ConvertToJsonAsync(string context, IInput? input)
        {
            if (input == null)
                return null;

            var serializer = new Serializer(excessiveDebugOutput: false);
            var obj = await serializer.SerializeAsync(context, input).ConfigureAwait(false);
            var value = Serializer.CreateValue(obj);
            var valueString = JsonFormatter.Default.Format(value);
            return (Input<string>)valueString;
        }

        private class EmptyResourceArgs : ResourceArgs
        {
        }

        private struct InputInfo
        {
            public readonly InputAttribute Attribute;
            public readonly Type MemberType;
            public readonly string MemberName;
            public Func<object, object?> GetValue;

            public InputInfo(InputAttribute attribute, string memberName, Type memberType, Func<object, object> getValue) : this()
            {
                Attribute = attribute;
                MemberName = memberName;
                MemberType = memberType;
                GetValue = getValue;
            }
        }
    }
}
