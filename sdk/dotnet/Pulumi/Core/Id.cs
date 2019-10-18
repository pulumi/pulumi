// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;

namespace Pulumi
{
    /// <summary>
    /// A provider-assigned ID.
    /// </summary>
    public readonly struct Id : IEquatable<Id>
    {
        private readonly string _value;

        internal Id(string value)
            => _value = value ?? throw new ArgumentNullException(nameof(value));

        public override string ToString()
            => $"Id({_value})";

        public override int GetHashCode()
            => _value.GetHashCode();

        public override bool Equals(object obj)
            => obj is Id id && Equals(id);

        public bool Equals(Id id)
            => _value == id._value;
    }
}
