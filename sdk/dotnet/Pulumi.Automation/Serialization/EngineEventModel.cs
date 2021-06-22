// Copyright 2016-2021, Pulumi Corporation

using Pulumi.Automation.Events;
using Pulumi.Automation.Serialization.Json;

// NOTE: The classes in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go
namespace Pulumi.Automation.Serialization
{
    internal class EngineEventModel : IJsonModel<EngineEvent>
    {
        public int Sequence { get; set; }

        public int Timestamp { get; set; }

        public CancelEventModel? CancelEvent { get; set; }
        public StandardOutputEngineEventModel? StdoutEvent { get; set; }
        public DiagnosticEventModel? DiagnosticEvent { get; set; }
        public PreludeEventModel? PreludeEvent { get; set; }
        public SummaryEventModel? SummaryEvent { get; set; }
        public ResourcePreEventModel? ResourcePreEvent { get; set; }
        public ResourceOutputsEventModel? ResOutputsEvent { get; set; }
        public ResourceOperationFailedEventModel? ResOpFailedEvent { get; set; }
        public PolicyEventModel? PolicyEvent { get; set; }

        public EngineEvent Convert() =>
            new EngineEvent(
                this.Sequence,
                this.Timestamp,
                this.CancelEvent?.Convert(),
                this.StdoutEvent?.Convert(),
                this.DiagnosticEvent?.Convert(),
                this.PreludeEvent?.Convert(),
                this.SummaryEvent?.Convert(),
                this.ResourcePreEvent?.Convert(),
                this.ResOutputsEvent?.Convert(),
                this.ResOpFailedEvent?.Convert(),
                this.PolicyEvent?.Convert());
    }
}
