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

import { OpMap, OpType } from "./stack";

/**
 * CancelEvent is emitted when the user initiates a cancellation of the update in progress, or
 * the update successfully completes.
 */
export type CancelEvent = {};

/**
 * An event emitted whenever a generic message is written, for example warnings
 * from the pulumi CLI itself. Less common than {@link DiagnosticEvent}
 */
export interface StdoutEngineEvent {
    message: string;
    color: string;
}

/**
 * An event emitted whenever a diagnostic message is provided, for example errors from
 * a cloud resource provider while trying to create or update a resource.
 */
export interface DiagnosticEvent {
    urn?: string;
    prefix?: string;
    message: string;
    color: string;
    severity: "info" | "info#err" | "warning" | "error";
    streamID?: number;
    ephemeral?: boolean;
}

/**
 * An event emitted whenever there is a policy violation.
 */
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

/**
 * An event emitted at the start of an update.
 */
export interface PreludeEvent {
    /**
     * Configuration values that will be used during the update.
     */
    config: Record<string, string>;
}

/**
 * An event emitted at the end of an update, with a summary of the changes made.
 */
export interface SummaryEvent {
    /**
     * True if one or more of the resources are in an invalid state.
     */
    maybeCorrupt: boolean;

    /**
     * The number of seconds the update took to execute.
     */
    durationSeconds: number;

    /**
     * The count for resource changes by type.
     */
    resourceChanges: OpMap;

    /**
     * The policy packs that were run during the update. Maps PolicyPackName -> version.
     *
     * Note: When this field was initially added, we forgot to add the JSON tag
     * and are now locked into using PascalCase for this field to maintain backwards
     * compatibility. For older clients this will map to the version, while for newer ones
     * it will be the version tag prepended with "v".
     */
    policyPacks: Record<string, string>;
}

/**
 * A {@link DiffKind} describes the kind of difference between two values
 * reported in a diff.
 */
export enum DiffKind {
    /**
     * Indicates that the property was added.
     */
    add = "add",

    /**
     * Indicates that the property was added and requires that the resource be replaced.
     */
    addReplace = "add-replace",

    /**
     * Indicates that the property was deleted.
     */
    delete = "delete",

    /**
     * Indicates that the property was deleted and requires that the resource be replaced.
     */
    deleteReplace = "delete-replace",

    /**
     * Indicates that the property was updated.
     */
    update = "update",

    /**
     * Indicates that the property was updated and requires that the resource be replaced.
     */
    updateReplace = "update-replace",
}

/**
 * A {@link PropertyDiff} describes the difference between a single property's old and new values.
 */
export interface PropertyDiff {
    /**
     * diffKind is the kind of difference.
     */
    diffKind: DiffKind;

    /**
     * inputDiff is true if this is a difference between old and new inputs
     * rather than old state and new inputs.
     */
    inputDiff: boolean;
}

/**
 * {@link StepEventMetadata} describes a "step" within the Pulumi engine, which
 * is any concrete action to migrate a set of cloud resources from one state to
 * another.
 */
export interface StepEventMetadata {
    /**
     * The type of operation being performed.
     */
    op: OpType;

    /**
     * The URN of the resource being operated on.
     */
    urn: string;

    /**
     * The type of the resource being operated on.
     */
    type: string;

    /**
     * Old is the state of the resource before performing the step.
     */
    old?: StepEventStateMetadata;

    /**
     * New is the state of the resource after performing the step.
     */
    new?: StepEventStateMetadata;

    /**
     * Keys causing a replacement (only applicable for "create" and "replace" Ops).
     */
    keys?: string[];

    /**
     * Keys that changed with this step.
     */
    diffs?: string[];

    /**
     * The diff for this step as a list of property paths and difference types.
     */
    detailedDiff?: Record<string, PropertyDiff>;

    /**
     * Logical is set if the step is a logical operation in the program.
     */
    logical?: boolean;

    /**
     * Provider actually performing the step.
     */
    provider: string;
}

/**
 * {@link StepEventStateMetadata} is the more detailed state information for a resource as it relates to
 * a step(s) being performed.
 */
export interface StepEventStateMetadata {
    /**
     * The type of the resource being operated on.
     */
    type: string;

    /**
     * The URN of the resource being operated on.
     */
    urn: string;

    /**
     * Custom indicates if the resource is managed by a plugin.
     */
    custom?: boolean;

    /**
     * Delete is true when the resource is pending deletion due to a replacement.
     */
    delete?: boolean;

    /**
     * ID is the resource's unique ID, assigned by the resource provider (or blank if none/uncreated).
     */
    id: string;

    /**
     * Parent is an optional parent URN that this resource belongs to.
     */
    parent: string;

    /**
     * Protect is true to "protect" this resource (protected resources cannot be deleted).
     */
    protect?: boolean;

    /**
     * RetainOnDelete is true if the resource is not physically deleted when it is logically deleted.
     */
    retainOnDelete?: boolean;

    /**
     * Inputs contains the resource's input properties (as specified by the program). Secrets have
     * filtered out, and large assets have been replaced by hashes as applicable.
     */
    inputs: Record<string, any>;

    /**
     * Outputs contains the resource's complete output state (as returned by the resource provider).
     */
    outputs: Record<string, any>;

    /**
     * Provider is the resource's provider reference
     */
    provider: string;

    /**
     * InitErrors is the set of errors encountered in the process of initializing resource.
     */
    initErrors?: string[];
}

/**
 * An event emitted before a resource is modified.
 */
export interface ResourcePreEvent {
    /**
     * Metadata for the event.
     */
    metadata: StepEventMetadata;

    planning?: boolean;
}

/**
 * An event emitted when a resource is finished being provisioned.
 */
export interface ResOutputsEvent {
    /**
     * Metadata for the event.
     */
    metadata: StepEventMetadata;

    planning?: boolean;
}

/**
 * An event emitted when a resource operation fails. Typically a
 * {@link DiagnosticEvent} is emitted before this event, indicating the root
 * cause of the error.
 */
export interface ResOpFailedEvent {
    /**
     * Metadata for the event.
     */
    metadata: StepEventMetadata;

    status: number;

    steps: number;
}

/**
 * An event emitted when a debugger has been started and is waiting for the user
 * to attach to it using the DAP protocol.
 */
export interface StartDebuggingEvent {
    config: Record<string, any>;
}

/**
 * A Pulumi engine event, such as a change to a resource or diagnostic message.
 * This is intended to capture a discriminated union -- exactly one event field
 * will be non-nil.
 */
export interface EngineEvent {
    /**
     * A unique, and monotonically increasing number for each engine event sent
     * to the Pulumi Service. Since events may be sent concurrently, and/or
     * delayed via network routing, the sequence number is to ensure events can
     * be placed into a total ordering.
     *
     * - No two events can have the same sequence number.
     * - Events with a lower sequence number must have been emitted before those with a higher
     *   sequence number.
     */
    sequence: number;

    /**
     * Timestamp is a Unix timestamp (seconds) of when the event was emitted.
     */
    timestamp: number;

    /**
     * A cancellation event, if this engine event represents a cancellation.
     */
    cancelEvent?: CancelEvent;

    /**
     * A stdout event, if this engine event represents a message written to stdout.
     */
    stdoutEvent?: StdoutEngineEvent;

    /**
     * A diagnostic event, if this engine event represents a diagnostic message.
     */
    diagnosticEvent?: DiagnosticEvent;

    /**
     * A prelude event, if this engine event represents the start of an
     * operation.
     */
    preludeEvent?: PreludeEvent;

    /**
     * A summary event, if this engine event represents the end of an operation.
     */
    summaryEvent?: SummaryEvent;

    /**
     * A resource pre-event, if this engine event represents a resource
     * about to be modified.
     */
    resourcePreEvent?: ResourcePreEvent;

    /**
     * A resource outputs event, if this engine event represents a resource
     * that has been modified.
     */
    resOutputsEvent?: ResOutputsEvent;

    /**
     * A resource operation failed event, if this engine event represents a resource
     * operation that failed.
     */
    resOpFailedEvent?: ResOpFailedEvent;

    /**
     * A policy event, if this engine event represents a policy violation.
     */
    policyEvent?: PolicyEvent;

    /**
     * A debugging event, if the engine event represents a debugging message.
     */
    startDebuggingEvent?: StartDebuggingEvent;
}
