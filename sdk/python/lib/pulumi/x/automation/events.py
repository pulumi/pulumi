# Copyright 2016-2021, Pulumi Corporation.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# NOTE: The classes in this file are intended to align with the serialized
# JSON types defined and versioned in sdk/go/common/apitype/events.go

from enum import Enum
from typing import Optional, List, Mapping, Any, MutableMapping


class OpType(str, Enum):
    """
    The granular CRUD operation performed on a particular resource during an update.
    """
    SAME = "same"
    CREATE = "create"
    UPDATE = "update"
    DELETE = "delete"
    REPLACE = "replace"
    CREATE_REPLACEMENT = "create-replacement"
    DELETE_REPLACED = "delete-replaced"
    READ = "read"
    READ_REPLACEMENT = "read-replacement"
    REFRESH = "refresh"
    DISCARD = "discard"
    DISCARD_REPLACED = "discard-replaced"
    REMOVE_PENDING_REPLACE = "remove-pending-replace"
    IMPORT = "import"
    IMPORT_REPLACEMENT = "import-replacement"


OpMap = MutableMapping[OpType, int]


class BaseEvent:
    def __repr__(self):
        # pylint: disable=duplicate-code
        inputs = self.__dict__
        fields = [f"{key}={inputs[key]!r}" for key in inputs]
        fields = ", ".join(fields)
        return f"{self.__class__.__name__}({fields})"


class CancelEvent(BaseEvent):
    """
    CancelEvent is emitted when the user initiates a cancellation of the update in progress, or
    the update successfully completes.
    """
    def __init__(self) -> None:
        pass

    @classmethod
    def from_json(cls) -> 'CancelEvent':
        return cls()


class StdoutEngineEvent(BaseEvent):
    """
    StdoutEngineEvent is emitted whenever a generic message is written, for example warnings
    from the pulumi CLI itself. Less common than DiagnosticEvent.

    Attributes
    ----------
    message: str
        The message
    color: str
        The color to render the message
    """
    def __init__(self, message: str, color: str) -> None:
        self.message = message
        self.color = color

    @classmethod
    def from_json(cls, data: dict) -> 'StdoutEngineEvent':
        return cls(**data)


class DiagnosticEvent(BaseEvent):
    """
    DiagnosticEvent is emitted whenever a diagnostic message is provided, for example errors from
    a cloud resource provider while trying to create or update a resource.

    Attributes
    ----------
    message: str
        The message
    color: str
        The color to render the message
    severity: str
        The severity of the message. One of "info", "info#err", "warning", "error"
    stream_id: Optional[str]
        The stream id
    ephemeral: Optional[bool]
        Signifies whether the message should be rendered ephemerally in the progress display
    urn: Optional[str]
        The urn of the resource
    prefix: Optional[str]
        An optional prefix
    """
    def __init__(self,
                 message: str,
                 color: str,
                 severity: str,
                 stream_id: Optional[int] = None,
                 ephemeral: Optional[bool] = None,
                 urn: Optional[str] = None,
                 prefix: Optional[str] = None) -> None:
        self.message = message
        self.color = color
        self.severity = severity
        self.stream_id = stream_id
        self.ephemeral = ephemeral
        self.urn = urn
        self.prefix = prefix

    @classmethod
    def from_json(cls, data: dict) -> 'DiagnosticEvent':
        return cls(message=data.get("message", ""),
                   color=data.get("color", ""),
                   severity=data.get("severity", ""),
                   stream_id=data.get("streamId"),
                   ephemeral=data.get("ephemeral"),
                   urn=data.get("urn"),
                   prefix=data.get("prefix"))


class PolicyEvent(BaseEvent):
    """
    PolicyEvent is emitted whenever there is a Policy violation.

    Attributes
    ----------
    message: str
        The message
    color: str
        The color to render the message
    policy_name: str
        The name of the policy
    policy_pack_name: str
        The name of the policy pack
    policy_pack_version: str
        The version of the policy pack
    policy_pack_version_tag: str
        The policy pack version tag
    enforcement_level: str
        The enforcement level of the policy. One of "warning or "mandatory"
    resource_urn: Optional[str]
        The urn of the resource associated with this event
    """
    def __init__(self,
                 message: str,
                 color: str,
                 policy_name: str,
                 policy_pack_name: str,
                 policy_pack_version: str,
                 policy_pack_version_tag: str,
                 enforcement_level: str,
                 resource_urn: Optional[str] = None) -> None:
        self.message = message
        self.color = color
        self.policy_name = policy_name
        self.policy_pack_name = policy_pack_name
        self.policy_pack_version = policy_pack_version
        self.policy_pack_version_tag = policy_pack_version_tag
        self.enforcement_level = enforcement_level
        self.resource_urn = resource_urn

    @classmethod
    def from_json(cls, data: dict) -> 'PolicyEvent':
        return cls(message=data.get("message", ""),
                   color=data.get("color", ""),
                   policy_name=data.get("policyName", ""),
                   policy_pack_name=data.get("policyPackName", ""),
                   policy_pack_version=data.get("policyPackVersion", ""),
                   policy_pack_version_tag=data.get("policyPackVersionTag", ""),
                   enforcement_level=data.get("enforcementLevel", ""),
                   resource_urn=data.get("resource_urn"))


class PreludeEvent(BaseEvent):
    """
    PreludeEvent is emitted at the start of an update.

    Attributes
    ----------
    config: Mapping[str, str]
        config contains the keys and values for the update.
        Encrypted configuration values may be blinded.
    """
    def __init__(self, config: Mapping[str, str]) -> None:
        self.config = config

    @classmethod
    def from_json(cls, data: dict) -> 'PreludeEvent':
        return cls(**data)


class SummaryEvent(BaseEvent):
    """
    SummaryEvent is emitted at the end of an update, with a summary of the changes made.

    Attributes
    ----------
    maybe_corrupt: bool
        maybeCorrupt is set if one or more of the resources is in an invalid state.
    duration_seconds: int
        duration is the number of seconds the update was executing.
    resource_changes: OpMap
        resourceChanges contains the count for resource change by type. The keys are deploy.StepOp,
        which is not exported in this package.
    policy_packs: Mapping[str, str]
        policyPacks run during update. Maps PolicyPackName -> version.
        Note: When this field was initially added, we forgot to add the JSON tag
        and are now locked into using PascalCase for this field to maintain backwards
        compatibility. For older clients this will map to the version, while for newer ones
        it will be the version tag prepended with "v".
    """
    def __init__(self,
                 maybe_corrupt: bool,
                 duration_seconds: int,
                 resource_changes: OpMap,
                 policy_packs: Mapping[str, str]) -> None:
        self.maybe_corrupt = maybe_corrupt
        self.duration_seconds = duration_seconds
        self.resource_changes = resource_changes
        self.policy_packs = policy_packs

    @classmethod
    def from_json(cls, data: dict) -> 'SummaryEvent':
        return cls(maybe_corrupt=data.get("maybeCorrupt", False),
                   duration_seconds=data.get("durationSeconds", 0),
                   resource_changes=data.get("resourceChanges", {}),
                   policy_packs=data.get("PolicyPacks", {}))


class DiffKind(str, Enum):
    """
    DiffKind enumerates the possible kinds of diffs.

    Values
    ------
    * ADD: indicates that the property was added.
    * ADD_REPLACE: indicates that the property was added and requires that the resource be replaced.
    * DELETE: delete indicates that the property was deleted.
    * DELETE_REPLACE: indicates that the property was deleted and requires that the resource be replaced.
    * UPDATE: update indicates that the property was updated.
    * UPDATE_REPLACE: indicates that the property was updated and requires that the resource be replaced.
    """
    ADD = "add"
    ADD_REPLACE = "add-replace"
    DELETE = "delete"
    DELETE_REPLACE = "delete-replace"
    UPDATE = "update"
    UPDATE_REPLACE = "update-replace"


class PropertyDiff(BaseEvent):
    """
    PropertyDiff describes the difference between a single property's old and new values.

    Attributes
    ----------
    diff_kind: DiffKind
        diff_kind is the kind of difference.
    input_diff: bool
        input_diff is true if this is a difference between old and new inputs rather than old state and new inputs.
    """
    def __init__(self, diff_kind: DiffKind, input_diff: bool) -> None:
        self.diff_kind = diff_kind
        self.input_diff = input_diff

    @classmethod
    def from_json(cls, data: dict) -> 'PropertyDiff':
        return cls(diff_kind=DiffKind(data.get("diffKind")),
                   input_diff=data.get("inputDiff", False))


class StepEventStateMetadata(BaseEvent):
    """
    StepEventStateMetadata is the more detailed state information for a resource as it relates to
    a step(s) being performed.

    Attributes
    ----------
    type: str
        The type of the resource
    urn: str
        The URN of the resource
    id: str
        The resource's id
    parent: str
        The URN of the parent resource
    provider: str
        The URN of the resource provider
    custom: bool
        Indicates if the resource is managed by a plugin
    delete: bool
        True when the resource is pending deletion due to replacement.
    protect: bool
        Protect is true to "protect" this resource (protected resources cannot be deleted).
    inputs: Mapping[str, Any]
        Inputs contains the resource's input properties (as specified by the program). Secrets have
        filtered out, and large assets have been replaced by hashes as applicable.
    outputs: Mapping[str, Any]
        Outputs contains the resource's complete output state (as returned by the resource provider).
    init_errors: Optional[List[str]]
        init_errors is the set of errors encountered in the process of initializing resource.
    """
    def __init__(self,
                 type: str,  # pylint: disable=redefined-builtin
                 urn: str,
                 id: str,  # pylint: disable=redefined-builtin
                 parent: str,
                 provider: str,
                 custom: Optional[bool] = None,
                 delete: Optional[bool] = None,
                 protect: Optional[bool] = None,
                 inputs: Mapping[str, Any] = None,
                 outputs: Mapping[str, Any] = None,
                 init_errors: Optional[List[str]] = None):
        self.type = type
        self.urn = urn
        self.id = id
        self.parent = parent
        self.provider = provider
        self.custom = custom
        self.delete = delete
        self.protect = protect
        self.inputs = inputs
        self.outputs = outputs
        self.init_errors = init_errors

    @classmethod
    def from_json(cls, data: dict) -> 'StepEventStateMetadata':
        return cls(type=data.get("type", ""),
                   urn=data.get("urn", ""),
                   id=data.get("id", ""),
                   parent=data.get("parent", ""),
                   provider=data.get("provider", ""),
                   custom=data.get("custom"),
                   delete=data.get("delete"),
                   protect=data.get("protect"),
                   inputs=data.get("inputs"),
                   outputs=data.get("outputs"),
                   init_errors=data.get("initErrors"))


class StepEventMetadata(BaseEvent):
    """
    StepEventMetadata describes a "step" within the Pulumi engine, which is any concrete action
    to migrate a set of cloud resources from one state to another.

    Attributes
    ----------
    op: OpType
        The operation being performed.
    urn: str
        The urn of the resource.
    type: str
        The type of resource.
    provider: str
        The provider actually performing the step.
    old: Optional[StepEventStateMetadata]
        old is the state of the resource before performing the step.
    new: Optional[StepEventStateMetadata]
        new is the state of the resource after performing the step.
    keys: Optional[List[str]]
        keys causing a replacement (only applicable for "create" and "replace" Ops)
    diffs: Optional[List[str]]
        Keys that changed with this step.
    detailed_diff: Optional[Mapping[str, PropertyDiff]]
        The diff for this step as a list of property paths and difference types.
    logical: Optional[bool]
        Logical is set if the step is a logical operation in the program.
    """
    def __init__(self,
                 op: OpType,
                 urn: str,
                 type: str,  # pylint: disable=redefined-builtin
                 provider: str,
                 old: Optional[StepEventStateMetadata] = None,
                 new: Optional[StepEventStateMetadata] = None,
                 keys: Optional[List[str]] = None,
                 diffs: Optional[List[str]] = None,
                 detailed_diff: Optional[Mapping[str, PropertyDiff]] = None,
                 logical: Optional[bool] = None):
        self.op = op
        self.urn = urn
        self.type = type
        self.provider = provider
        self.old = old
        self.new = new
        self.keys = keys
        self.diffs = diffs
        self.detailed_diff = detailed_diff
        self.logical = logical

    @classmethod
    def from_json(cls, data: dict) -> 'StepEventMetadata':
        old = data.get("old")
        new = data.get("new")

        return cls(op=OpType(data.get("op", "")),
                   urn=data.get("urn", ""),
                   type=data.get("type", ""),
                   provider=data.get("provider", ""),
                   old=StepEventStateMetadata.from_json(old) if old else None,
                   new=StepEventStateMetadata.from_json(new) if new else None,
                   keys=data.get("keys"),
                   diffs=data.get("diffs"),
                   detailed_diff=data.get("detailed_diff"),
                   logical=data.get("logical"))


class ResourcePreEvent(BaseEvent):
    """
    ResourcePreEvent is emitted before a resource is modified.
    """
    def __init__(self,
                 metadata: StepEventMetadata,
                 planning: Optional[bool] = None):
        self.metadata = metadata
        self.planning = planning

    @classmethod
    def from_json(cls, data: dict) -> 'ResourcePreEvent':
        return cls(**data)


class ResOutputsEvent(BaseEvent):
    """
    ResOutputsEvent is emitted when a resource is finished being provisioned.
    """
    def __init__(self,
                 metadata: StepEventMetadata,
                 planning: Optional[bool] = None):
        self.metadata = metadata
        self.planning = planning

    @classmethod
    def from_json(cls, data: dict) -> 'ResOutputsEvent':
        return cls(**data)


class ResOpFailedEvent(BaseEvent):
    """
    ResOpFailedEvent is emitted when a resource operation fails. Typically a DiagnosticEvent is
    emitted before this event, indicating the root cause of the error.
    """
    def __init__(self,
                 metadata: StepEventMetadata,
                 status: int,
                 steps: int):
        self.metadata = metadata
        self.status = status
        self.steps = steps

    @classmethod
    def from_json(cls, data: dict) -> 'ResOpFailedEvent':
        return cls(**data)


class EngineEvent(BaseEvent):
    """
    EngineEvent describes a Pulumi engine event, such as a change to a resource or diagnostic
    message. EngineEvent is a discriminated union of all possible event types, and exactly one
    field will be non-nil.

    Attributes
    ----------
    sequence: str
        Sequence is a unique, and monotonically increasing number for each engine event sent to the
        Pulumi Service. Since events may be sent concurrently, and/or delayed via network routing,
        the sequence number is to ensure events can be placed into a total ordering.
        - No two events can have the same sequence number.
        - Events with a lower sequence number must have been emitted before those with a higher
          sequence number.
    timestamp: int
        Timestamp is a Unix timestamp (seconds) of when the event was emitted.
    """
    def __init__(self,
                 sequence: int,
                 timestamp: int,
                 cancel_event: Optional[CancelEvent] = None,
                 stdout_event: Optional[StdoutEngineEvent] = None,
                 diagnostic_event: Optional[DiagnosticEvent] = None,
                 prelude_event: Optional[PreludeEvent] = None,
                 summary_event: Optional[SummaryEvent] = None,
                 resource_pre_event: Optional[ResourcePreEvent] = None,
                 res_outputs_event: Optional[ResOutputsEvent] = None,
                 res_op_failed_event: Optional[ResOpFailedEvent] = None,
                 policy_event: Optional[PolicyEvent] = None):
        self.sequence = sequence
        self.timestamp = timestamp
        self.cancel_event = cancel_event
        self.stdout_event = stdout_event
        self.diagnostic_event = diagnostic_event
        self.prelude_event = prelude_event
        self.summary_event = summary_event
        self.resource_pre_event = resource_pre_event
        self.res_outputs_event = res_outputs_event
        self.res_op_failed_event = res_op_failed_event
        self.policy_event = policy_event

    @classmethod
    def from_json(cls, data: dict) -> 'EngineEvent':
        stdout_event = data.get("stdoutEvent")
        diagnostic_event = data.get("diagnosticEvent")
        prelude_event = data.get("preludeEvent")
        summary_event = data.get("summaryEvent")
        resource_pre_event = data.get("resourcePreEvent")
        res_outputs_event = data.get("resOutputsEvent")
        res_op_failed_event = data.get("resOpFailedEvent")
        policy_event = data.get("policyEvent")

        return cls(sequence=data.get("sequence", 0),
                   timestamp=data.get("timestamp", 0),
                   cancel_event=CancelEvent() if "cancelEvent" in data else None,
                   stdout_event=StdoutEngineEvent.from_json(stdout_event) if stdout_event else None,
                   diagnostic_event=DiagnosticEvent.from_json(diagnostic_event) if diagnostic_event else None,
                   prelude_event=PreludeEvent.from_json(prelude_event) if prelude_event else None,
                   summary_event=SummaryEvent.from_json(summary_event) if summary_event else None,
                   resource_pre_event=ResourcePreEvent.from_json(resource_pre_event) if resource_pre_event else None,
                   res_outputs_event=ResOutputsEvent.from_json(res_outputs_event) if res_outputs_event else None,
                   res_op_failed_event=ResOpFailedEvent.from_json(res_op_failed_event) if res_op_failed_event else None,
                   policy_event=PolicyEvent.from_json(policy_event) if policy_event else None)
