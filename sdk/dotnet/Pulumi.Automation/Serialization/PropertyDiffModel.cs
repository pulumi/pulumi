// Copyright 2016-2021, Pulumi Corporation

using Pulumi.Automation.Serialization.Json;
using Pulumi.Automation.Events;

// NOTE: The classes in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go
namespace Pulumi.Automation.Serialization
{
    internal class PropertyDiffModel : IJsonModel<PropertyDiff>
    {
        public DiffKind Kind { get; set; }

        public bool InputDiff { get; set; }

        public PropertyDiff Convert() =>
            new PropertyDiff(this.Kind, this.InputDiff);
    }
}
