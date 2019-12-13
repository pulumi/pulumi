// Copyright 2016-2019, Pulumi Corporation

using System;

namespace Pulumi
{
    /// <summary>
    /// Optional timeouts to supply in <see cref="ResourceOptions.CustomTimeouts"/>.
    /// </summary>
    public sealed class CustomTimeouts
    {
        /// <summary>
        /// The optional create timeout.
        /// </summary>
        public TimeSpan? Create { get; set; }

        /// <summary>
        /// The optional update timeout.
        /// </summary>
        public TimeSpan? Update { get; set; }

        /// <summary>
        /// The optional delete timeout.
        /// </summary>
        public TimeSpan? Delete { get; set; }

        internal static CustomTimeouts? Clone(CustomTimeouts? timeouts)
            => timeouts == null ? null : new CustomTimeouts
            {
                Create = timeouts.Create,
                Delete = timeouts.Delete,
                Update = timeouts.Update,
            };
    }
}
