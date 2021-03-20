// Copyright 2016-2021, Pulumi Corporation
// NOTE: The class in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go

namespace Pulumi.Automation.Events
{
    /// <summary>
    /// EngineEvent describes a Pulumi engine event, such as a change to a resource or diagnostic
    /// message. EngineEvent is a discriminated union of all possible event types, and exactly one
    /// field will be non-null.
    /// </summary>
    public class EngineEvent
    {
        /// <summary>
        /// Sequence is a unique, and monotonically increasing number for each engine event sent to the
        /// Pulumi Service. Since events may be sent concurrently, and/or delayed via network routing,
        /// the sequence number is to ensure events can be placed into a total ordering.
        /// <para>
        /// - No two events can have the same sequence number.
        /// </para>
        /// <para>
        /// - Events with a lower sequence number must have been emitted before those with a higher sequence number.
        /// </para>
        /// </summary>
        public int Sequence { get; }

        /// <summary>
        /// Timestamp is a Unix timestamp (seconds) of when the event was emitted.
        /// </summary>
        public int Timestamp { get; }

        public CancelEvent? CancelEvent { get; }
        public StdoutEngineEvent? StdoutEvent { get; }
        public DiagnosticEvent? DiagnosticEvent { get; }
        public PreludeEvent? PreludeEvent { get; }
        public SummaryEvent? SummaryEvent { get; }
        public ResourcePreEvent? ResourcePreEvent { get; }
        public ResOutputsEvent? ResOutputsEvent { get; }
        public ResOpFailedEvent? ResOpFailedEvent { get; }
        public PolicyEvent? PolicyEvent { get; }

        internal EngineEvent(
            int sequence,
            int timestamp,
            CancelEvent? cancelEvent,
            StdoutEngineEvent? stdoutEvent,
            DiagnosticEvent? diagnosticEvent,
            PreludeEvent? preludeEvent,
            SummaryEvent? summaryEvent,
            ResourcePreEvent? resourcePreEvent,
            ResOutputsEvent? resOutputsEvent,
            ResOpFailedEvent? resOpFailedEvent,
            PolicyEvent? policyEvent)
        {
            Sequence = sequence;
            Timestamp = timestamp;
            CancelEvent = cancelEvent;
            StdoutEvent = stdoutEvent;
            DiagnosticEvent = diagnosticEvent;
            PreludeEvent = preludeEvent;
            SummaryEvent = summaryEvent;
            ResourcePreEvent = resourcePreEvent;
            ResOutputsEvent = resOutputsEvent;
            ResOpFailedEvent = resOpFailedEvent;
            PolicyEvent = policyEvent;
        }
    }
}
