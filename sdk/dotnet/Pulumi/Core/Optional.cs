// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Generic;
using System.Diagnostics.CodeAnalysis;

namespace Pulumi
{
    /// <summary>
    /// Combines a value, <see cref="Value"/>, and a flag, <see cref="HasValue"/>, indicating
    /// whether or not that value is meaningful.
    /// </summary>
    /// <typeparam name="T">The type of the value.</typeparam>
    public readonly struct Optional<T> : IEquatable<Optional<T>>
    {
        /// <summary>
        /// Constructs an <see cref="Optional{T}"/> with a meaningful value.
        /// </summary>
        public Optional(T value)
        {
            HasValue = true;
            Value = value;
        }

        /// <summary>
        /// Returns <see langword="true"/> if the <see cref="Value"/> will return a meaningful
        /// value.
        /// </summary>
        public bool HasValue { get; }

        /// <summary>
        /// Gets the value of the current object.  Not meaningful unless <see cref="HasValue"/>
        /// returns <see langword="true"/>.
        /// </summary>
        /// <remarks>
        /// <para>Unlike <see cref="Nullable{T}.Value"/>, this property does not throw an exception when
        /// <see cref="HasValue"/> is <see langword="false"/>.</para>
        /// </remarks>
        /// <returns>
        /// <para>The value if <see cref="HasValue"/> is <see langword="true"/>; otherwise, the default value for type
        /// <typeparamref name="T"/>.</para>
        /// </returns>
        public T Value { get; }

        /// <summary>
        /// Creates a new object initialized to a meaningful value. 
        /// </summary>
        public static implicit operator Optional<T>(T value)
            => new Optional<T>(value);

        /// <summary>
        /// Returns a string representation of this object.
        /// </summary>
        public override string ToString()
        {
            // Note: For nullable types, it's possible to have _hasValue true and _value null.
            return HasValue
                ? Value?.ToString() ?? "null"
                : "unspecified";
        }

        public override bool Equals(object? obj)
            => obj is Optional<T> optional && Equals(optional);

        public override int GetHashCode()
            => HashCode.Combine(HasValue, Value);

        public bool Equals(Optional<T> other)
            => HasValue == other.HasValue && EqualityComparer<T>.Default.Equals(Value, other.Value);

        public static bool operator ==(Optional<T> left, Optional<T> right)
            => left.Equals(right);

        public static bool operator !=(Optional<T> left, Optional<T> right)
            => !(left == right);
    }
}
