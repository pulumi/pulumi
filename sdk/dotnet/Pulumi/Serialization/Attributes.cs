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
    public sealed class OutputAttribute : Attribute 
    {
        public readonly string Name;

        public OutputAttribute(string name)
        {
            Name = name;
        }
    }

    /// <summary>
    /// Attribute used by a Pulumi Cloud Provider Package to mark Resource input fields and
    /// properties.
    /// <para/>
    /// Note: for simple inputs (i.e. <see cref="Input{T}"/> this should just be placed on the
    /// property itself.  i.e. <c>[Input] Input&lt;string&gt; Acl</c>.
    /// 
    /// For collection inputs (i.e. <see cref="InputList{T}"/> this shuld be placed on the
    /// backing field for the property.  i.e.
    /// 
    /// <code>
    ///     [Input] private InputList&lt;string&gt; _acls;
    ///     public InputList&lt;string&gt; Acls
    ///     {
    ///         get => _acls ?? (_acls = new InputList&lt;string&gt;());
    ///         set => _acls = value;
    ///     }
    /// </code>
    /// </summary>
    [AttributeUsage(AttributeTargets.Field | AttributeTargets.Property)]
    public sealed class InputAttribute : Attribute
    {
        public readonly string Name;
        public readonly bool Required;

        public InputAttribute(string name, bool required = false)
        {
            Name = name;
            Required = required;
        }
    }

    /// <summary>
    /// Attribute used by a Pulumi Cloud Provider Package to mark complex types used for a Resource
    /// output property.  A complex type must have a single constructor in it marked with the 
    /// <see cref="OutputConstructorAttribute"/> attribute.
    /// </summary>
    [AttributeUsage(AttributeTargets.Class)]
    public sealed class OutputTypeAttribute : Attribute
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
    public sealed class OutputConstructorAttribute : Attribute
    {
    }
}
