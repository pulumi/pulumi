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
    /// Base type for all input argument classes.
    /// </summary>
    public abstract class InputArgs
    {
        private readonly ImmutableArray<InputInfo> _inputInfos;

        private protected abstract void ValidateMember(Type memberType, string fullName);

        protected InputArgs()
        {
            var fieldQuery =
                from field in this.GetType().GetFields(BindingFlags.NonPublic | BindingFlags.Public | BindingFlags.Instance)
                let attr1 = field.GetCustomAttribute<Pulumi.InputAttribute>()
#pragma warning disable 618
                let attr2 = field.GetCustomAttribute<Pulumi.Serialization.InputAttribute>()
#pragma warning restore 618
                where attr1 != null || attr2 != null
                let attr = attr1 ?? new Pulumi.InputAttribute(attr2.Name, attr2.IsRequired, attr2.Json)
                select (attr, memberName: field.Name, memberType: field.FieldType, getValue: (Func<object, object?>)field.GetValue);

            var propQuery =
                from prop in this.GetType().GetProperties(BindingFlags.NonPublic | BindingFlags.Public | BindingFlags.Instance)
                let attr1 = prop.GetCustomAttribute<Pulumi.InputAttribute>()
#pragma warning disable 618
                let attr2 = prop.GetCustomAttribute<Pulumi.Serialization.InputAttribute>()
#pragma warning restore 618
                where attr1 != null || attr2 != null
                let attr = attr1 ?? new Pulumi.InputAttribute(attr2.Name, attr2.IsRequired, attr2.Json)
                select (attr, memberName: prop.Name, memberType: prop.PropertyType, getValue: (Func<object, object?>)prop.GetValue);

            var all = fieldQuery.Concat(propQuery).ToList();

            foreach (var (attr, memberName, memberType, getValue) in all)
            {
                var fullName = $"[Input] {this.GetType().FullName}.{memberName}";
                ValidateMember(memberType, fullName);
            }

            _inputInfos = all.Select(t =>
                new InputInfo(t.attr, t.memberName, t.memberType, t.getValue)).ToImmutableArray();
        }

        internal virtual async Task<ImmutableDictionary<string, object?>> ToDictionaryAsync()
        {
            var builder = ImmutableDictionary.CreateBuilder<string, object?>();
            foreach (var info in _inputInfos)
            {
                var fullName = $"[Input] {this.GetType().FullName}.{info.MemberName}";

                var value = info.GetValue(this);
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

        private async Task<object?> ConvertToJsonAsync(string context, object? input)
        {
            if (input == null)
                return null;

            var serializer = new Serializer(excessiveDebugOutput: false);
            var obj = await serializer.SerializeAsync(context, input).ConfigureAwait(false);
            var value = Serializer.CreateValue(obj);
            return JsonFormatter.Default.Format(value);
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
