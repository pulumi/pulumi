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

        protected ResourceArgs()
        {
        }

        internal ImmutableDictionary<string, IInput> ToDictionary()
        {
            var builder = ImmutableDictionary.CreateBuilder<string, IInput>();

            var fieldQuery =
                from field in this.GetType().GetFields(BindingFlags.NonPublic | BindingFlags.Public | BindingFlags.Instance)
                let attr = field.GetCustomAttribute<InputAttribute>()
                where attr != null
                select (attr, field.Name, memberType: field.FieldType, getValue: (Func<object?,object?>)field.GetValue);

            var propQuery =
                from prop in this.GetType().GetProperties(BindingFlags.NonPublic | BindingFlags.Public | BindingFlags.Instance)
                let attr = prop.GetCustomAttribute<InputAttribute>()
                where attr != null
                select (attr, prop.Name, memberType: prop.PropertyType, getValue: (Func<object?, object?>)prop.GetValue);

            var all = fieldQuery.Concat(propQuery).ToList();

            foreach (var (attr, memberName, memberType, getValue) in all)
            {
                var fullName = $"[Input] {this.GetType().FullName}.{memberName}";

                if (!typeof(IInput).IsAssignableFrom(memberType))
                {
                    throw new InvalidOperationException($"{fullName} was not an Input<T>");
                }

                var value = (IInput)getValue(this);
                if (attr.Required && value == null)
                {
                    throw new ArgumentNullException(memberName, $"{fullName} is required but was not given a value");
                }

                builder.Add(attr.Name, value!);
            }

            return builder.ToImmutable();
        }

        private class EmptyResourceArgs : ResourceArgs
        {
        }
    }
}
