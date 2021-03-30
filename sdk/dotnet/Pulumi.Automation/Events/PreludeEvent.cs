// Copyright 2016-2021, Pulumi Corporation
// NOTE: The class in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go

using System.Collections.Generic;
using System.Collections.Immutable;

namespace Pulumi.Automation.Events
{
    /// <summary>
    /// <see cref="PreludeEvent"/> is emitted at the start of an update.
    /// </summary>
    public class PreludeEvent
    {
        /// <summary>
        /// Config contains the keys and values for the update.
        /// Encrypted configuration values may be blinded.
        /// </summary>
        public IImmutableDictionary<string, string> Config { get; }

        internal PreludeEvent(IDictionary<string, string> config)
        {
            Config = config.ToImmutableDictionary();
        }
    }
}
