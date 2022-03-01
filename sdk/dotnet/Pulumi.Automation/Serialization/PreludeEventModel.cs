// Copyright 2016-2021, Pulumi Corporation

using System.Collections.Generic;
using Pulumi.Automation.Events;
using Pulumi.Automation.Serialization.Json;

// NOTE: The classes in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go
namespace Pulumi.Automation.Serialization
{
    internal class PreludeEventModel : IJsonModel<PreludeEvent>
    {
        public Dictionary<string, string> Config { get; set; } = null!;

        public PreludeEvent Convert() => new PreludeEvent(this.Config);
    }
}
