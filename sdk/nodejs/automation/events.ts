// Copyright 2016-2020, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// NOTE: The interfaces in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go

// CancelEvent is emitted when the user initiates a cancellation of the update in progress, or
// the update successfully completes.
import { OpMap, OpType } from "./stack";

export interface CancelEvent {
}

// StdoutEngineEvent is emitted whenever a generic message is written, for example warnings
// from the pulumi CLI itself. Less common than DiagnosticEvent
export interface StdoutEngineEvent {
    message: string;
    color: string;
}

// DiagnosticEvent is emitted whenever a diagnostic message is provided, for example errors from
// a cloud resource provider while trying to create or update a resource.
export interface DiagnosticEvent {
    urn?: string;
    prefix?: string;
    message: string;
    color: string;
    severity: "info" | "info#err" | "warning" | "error";
    streamID?: number;
    ephemeral?: boolean;
}

// PolicyEvent is emitted whenever there is a Policy violation.
export interface PolicyEvent {
    resourceUrn?: string;
    message: string;
    color: string;
    policyName: string;
    policyPackName: string;
    policyPackVersion: string;
    policyPackVersionTag: string;
    enforcementLevel: "warning" | "mandatory";
}

// PreludeEvent is emitted at the start of an update.
export interface PreludeEvent {
    // config contains the keys and values for the update.
    // Encrypted configuration values may be blinded.
    config: Record<string, string>;
}

// SummaryEvent is emitted at the end of an update, with a summary of the changes made.
export interface SummaryEvent {
    // maybeCorrupt is set if one or more of the resources is in an invalid state.
    maybeCorrupt: boolean;
    // duration is the number of seconds the update was executing.
    durationSeconds: number;
    // resourceChanges contains the count for resource change by type. The keys are deploy.StepOp,
    // which is not exported in this package.
    resourceChanges: OpMap;
    // policyPacks run during update. Maps PolicyPackName -> version.
    // Note: When this field was initially added, we forgot to add the JSON tag
    // and are now locked into using PascalCase for this field to maintain backwards
    // compatibility. For older clients this will map to the version, while for newer ones
    // it will be the version tag prepended with "v".
    policyPacks: Record<string, string>;
}

export enum DiffKind {
    // add indicates that the property was added.
    add = "add",
    // addReplace indicates that the property was added and requires that the resource be replaced.
    addReplace = "add-replace",
    // delete indicates that the property was deleted.
    delete = "delete",
    // deleteReplace indicates that the property was deleted and requires that the resource be replaced.
    deleteReplace = "delete-replace",
    // update indicates that the property was updated.
    update = "update",
    // updateReplace indicates that the property was updated and requires that the resource be replaced.
    updateReplace = "update-replace",
}

// PropertyDiff describes the difference between a single property's old and new values.
export interface PropertyDiff {
    // diffKind is the kind of difference.
    diffKind: DiffKind;
    // inputDiff is true if this is a difference between old and new inputs rather than old state and new inputs.
    inputDiff: boolean;
}

// StepEventMetadata describes a "step" within the Pulumi engine, which is any concrete action
// to migrate a set of cloud resources from one state to another.
export interface StepEventMetadata {
    // Op is the operation being performed.
    op: OpType;
    urn: string;
    type: string;

    // Old is the state of the resource before performing the step.
    old?: StepEventStateMetadata;
    // New is the state of the resource after performing the step.
    new?: StepEventStateMetadata;

    // Keys causing a replacement (only applicable for "create" and "replace" Ops).
    keys?: string[];
    // Keys that changed with this step.
    diffs?: string[];
    // The diff for this step as a list of property paths and difference types.
    detailedDiff?: Record<string, PropertyDiff>;
    // Logical is set if the step is a logical operation in the program.
    logical?: boolean;
    // Provider actually performing the step.
    provider: string;
}

// StepEventStateMetadata is the more detailed state information for a resource as it relates to
// a step(s) being performed.
export interface StepEventStateMetadata {
    type: string;
    urn: string;

    // Custom indicates if the resource is managed by a plugin.
    custom?: boolean;
    // Delete is true when the resource is pending deletion due to a replacement.
    delete?: boolean;
    // ID is the resource's unique ID, assigned by the resource provider (or blank if none/uncreated).
    id: string;
    // Parent is an optional parent URN that this resource belongs to.
    parent: string;
    // Protect is true to "protect" this resource (protected resources cannot be deleted).
    protect?: boolean;
    // Inputs contains the resource's input properties (as specified by the program). Secrets have
    // filtered out, and large assets have been replaced by hashes as applicable.
    inputs: Record<string, any>;
    // Outputs contains the resource's complete output state (as returned by the resource provider).
    outputs: Record<string, any>;
    // Provider is the resource's provider reference
    provider: string;
    // InitErrors is the set of errors encountered in the process of initializing resource.
    initErrors?: string[];
}

// ResourcePreEvent is emitted before a resource is modified.
export interface ResourcePreEvent {
    metadata: StepEventMetadata;
    planning?: boolean;
}

// ResOutputsEvent is emitted when a resource is finished being provisioned.
export interface ResOutputsEvent {
    metadata: StepEventMetadata;
    planning?: boolean;
}

// ResOpFailedEvent is emitted when a resource operation fails. Typically a DiagnosticEvent is
// emitted before this event, indicating the root cause of the error.
export interface ResOpFailedEvent {
    metadata: StepEventMetadata;
    status: number;
    steps: number;
}

// EngineEvent describes a Pulumi engine event, such as a change to a resource or diagnostic
// message. EngineEvent is a discriminated union of all possible event types, and exactly one
// field will be non-nil.
export interface EngineEvent {
    // Sequence is a unique, and monotonically increasing number for each engine event sent to the
    // Pulumi Service. Since events may be sent concurrently, and/or delayed via network routing,
    // the sequence number is to ensure events can be placed into a total ordering.
    //
    // - No two events can have the same sequence number.
    // - Events with a lower sequence number must have been emitted before those with a higher
    //   sequence number.
    sequence: number;

    // Timestamp is a Unix timestamp (seconds) of when the event was emitted.
    timestamp: number;

    cancelEvent?: CancelEvent;
    stdoutEvent?: StdoutEngineEvent;
    diagnosticEvent?: DiagnosticEvent;
    preludeEvent?: PreludeEvent;
    summaryEvent?: SummaryEvent;
    resourcePreEvent?: ResourcePreEvent;
    resOutputsEvent?: ResOutputsEvent;
    resOpFailedEvent?: ResOpFailedEvent;
    policyEvent?: PolicyEvent;
}
