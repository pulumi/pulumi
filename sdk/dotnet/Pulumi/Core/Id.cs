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
        internal readonly string Value;

        internal Id(string value)
            => Value = value ?? throw new ArgumentNullException(nameof(value));

        public override string ToString()
            => $"Id({Value})";

        public override int GetHashCode()
            => Value.GetHashCode();

        public override bool Equals(object? obj)
            => obj is Id id && Equals(id);

        public bool Equals(Id id)
            => Value == id.Value;
    }
}
