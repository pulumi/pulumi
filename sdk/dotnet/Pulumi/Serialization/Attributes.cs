// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System;
using Google.Protobuf.WellKnownTypes;

namespace Pulumi.Serialization
{
    /// <summary>
    /// Attribute used by a Pulumi Cloud Provider Package to mark Resource output properties.
    /// </summary>
    [AttributeUsage(AttributeTargets.Property)]
    public sealed class PropertyAttribute : Attribute 
    {
        public readonly string Name;

        public PropertyAttribute(string name)
        {
            Name = name;
        }
    }

    /// <summary>
    /// Attribute used by a Pulumi Cloud Provider Package to mark complex types used for a Resource
    /// output property.  A complex type must have a single constructor in it marked with the 
    /// <see cref="PropertyConstructorAttribute"/> attribute.
    /// </summary>
    [AttributeUsage(AttributeTargets.Class)]
    public sealed class PropertyTypeAttribute : Attribute
    {
    }

    /// <summary>
    /// Attribute used by a Pulumi Cloud Provider Package to marks the constructor for a complex
    /// property type so that it can be instantiated by the Pulumi runtime.
    /// 
    /// The constructor should contain parameters that map to the resultant <see
    /// cref="Struct.Fields"/> returned by the engine.
    /// </summary>
    [AttributeUsage(AttributeTargets.Constructor)]
    public sealed class PropertyConstructorAttribute : Attribute
    {
    }
}
