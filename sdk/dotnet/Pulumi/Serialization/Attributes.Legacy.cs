// Copyright 2016-2019, Pulumi Corporation

using System;
using Google.Protobuf.WellKnownTypes;

namespace Pulumi.Serialization
{
    /// <summary>
    /// Attribute used by a Pulumi Cloud Provider Package to mark Resource output properties.
    /// </summary>
    [AttributeUsage(AttributeTargets.Property)]
    [Obsolete("Use Pulumi.OutputAttribute instead")]
    public sealed class OutputAttribute : Attribute 
    {
        public string? Name { get; }

        public OutputAttribute(string? name = null)
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
    /// For collection inputs (i.e. <see cref="InputList{T}"/> this should be placed on the
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
    [Obsolete("Use Pulumi.InputAttribute instead")]
    public sealed class InputAttribute : Attribute
    {
        internal string Name { get; }
        internal bool IsRequired { get; }
        internal bool Json { get; }

        public InputAttribute(string name, bool required = false, bool json = false)
        {
            Name = name;
            IsRequired = required;
            Json = json;
        }
    }

    /// <summary>
    /// Attribute used by a Pulumi Cloud Provider Package to mark complex types used for a Resource
    /// output property.  A complex type must have a single constructor in it marked with the 
    /// <see cref="OutputConstructorAttribute"/> attribute.
    /// </summary>
    [AttributeUsage(AttributeTargets.Class)]
    [Obsolete("Use Pulumi.OutputTypeAttribute instead")]
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
    [Obsolete("Use Pulumi.OutputConstructorAttribute instead")]
    public sealed class OutputConstructorAttribute : Attribute
    {
    }
}
