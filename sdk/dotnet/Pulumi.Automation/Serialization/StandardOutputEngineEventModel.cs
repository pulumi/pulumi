// Copyright 2016-2021, Pulumi Corporation

using Pulumi.Automation.Serialization.Json;
using Pulumi.Automation.Events;

// NOTE: The classes in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go
namespace Pulumi.Automation.Serialization
{
    internal class StandardOutputEngineEventModel : IJsonModel<StandardOutputEngineEvent>
    {
        public string Message { get; set; } = null!;

        public string Color { get; set; } = null!;

        public StandardOutputEngineEvent Convert() =>
            new StandardOutputEngineEvent(this.Message, this.Color);
    }
}
