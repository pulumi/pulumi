// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Immutable;
using System.Linq;
using System.Reflection;
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
                select (attr, memberName: field.Name, memberType: field.FieldType, getValue: (Func<object?, object?>)field.GetValue);

            var propQuery =
                from prop in this.GetType().GetProperties(BindingFlags.NonPublic | BindingFlags.Public | BindingFlags.Instance)
                let attr = prop.GetCustomAttribute<InputAttribute>()
                where attr != null
                select (attr, memberName: prop.Name, memberType: prop.PropertyType, getValue: (Func<object?, object?>)prop.GetValue);

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
                new InputInfo(t.attr.Name, t.attr.Required, t.memberName, t.memberType, t.getValue)).ToImmutableArray();
        }

        internal ImmutableDictionary<string, IInput> ToDictionary()
        {
            var builder = ImmutableDictionary.CreateBuilder<string, IInput>();
            foreach (var info in _inputInfos)
            {
                var value = (IInput)info.GetValue(this);
                if (info.Required && value == null)
                {
                    var fullName = $"[Input] {this.GetType().FullName}.{info.MemberName}";
                    throw new ArgumentNullException(info.MemberName, $"{fullName} is required but was not given a value");
                }

                builder.Add(info.Name, value!);
            }

            return builder.ToImmutable();
        }

        private class EmptyResourceArgs : ResourceArgs
        {
        }

        private struct InputInfo
        {
            public readonly bool Required;
            public readonly string Name;

            public readonly Type MemberType;
            public readonly string MemberName;
            public Func<object, object> GetValue;

            public InputInfo(string name, bool required, string memberName, Type memberType, Func<object, object> getValue) : this()
            {
                Name = name;
                Required = required;
                MemberName = memberName;
                MemberType = memberType;
                GetValue = getValue;
            }
        }
    }
}
