// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Diagnostics.CodeAnalysis;
using System.Linq;
using System.Threading.Tasks;

namespace Pulumi
{
    /// <summary>
    /// Optional timeouts to supply in <see cref="ResourceOptions.CustomTimeouts"/>.
    /// </summary>
    public class CustomTimeouts
    {
        /// <summary>
        /// The optional create timeout represented as a string e.g. 5m, 40s, 1d.
        /// </summary>
        public string? Create { get; set; }

        /// <summary>
        /// The optional update timeout represented as a string e.g. 5m, 40s, 1d.
        /// </summary>
        public string? Update { get; set; }

        /// <summary>
        /// The optional delete timeout represented as a string e.g. 5m, 40s, 1d.
        /// </summary>
        public string? Delete { get; set; }
    }
}
