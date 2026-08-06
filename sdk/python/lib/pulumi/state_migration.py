# Copyright 2026, Pulumi Corporation.
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

"""Helpers for constructing component state migration results."""

import copy
from collections.abc import Mapping
from typing import Any, Optional, Union

from . import urn as urn_util
from .resource import StateMigrationArgs, StateMigrationResult


class StateMigrationContext:
    """
    StateMigrationContext provides helpers for building a state migration result. Construct it
    from the `StateMigrationArgs` passed to a callback to obtain a mutable working copy of the
    prior checkpoint state while accounting for the predecessor of every moved or merged resource.

    The context does not change deployment state. Calling `result` constructs the
    `StateMigrationResult` returned to the engine, which performs authoritative validation before
    accepting it.
    """

    urn: str
    """
    The current-program URN of the resource registration that triggered the migration.
    """

    old_state: list[dict[str, Any]]
    """
    The original prior state of the resource and its descendants in checkpoint format, with the
    prior root first. Treat this value as read-only. Access `state` to obtain a mutable working
    copy.
    """

    def __init__(self, args: StateMigrationArgs) -> None:
        self.urn = args.urn
        self.old_state = args.old_state
        self._state: Optional[list[dict[str, Any]]] = None
        self._successors: dict[str, str] = {}
        self._old_urns = {self._resource_urn(resource) for resource in self.old_state}

    @property
    def state(self) -> list[dict[str, Any]]:
        """
        A mutable, context-owned copy of `old_state`. The copy is created on first access, so a
        callback that returns `None` without accessing this property does not copy the subtree.

        The resources remain in raw checkpoint format so migrations can inspect and modify fields
        not covered by the helpers.
        """
        if self._state is None:
            self._state = copy.deepcopy(self.old_state)
        return self._state

    @property
    def successors(self) -> dict[str, str]:
        """
        The mutable successor map that will be returned by `result`. The helper methods populate
        this map automatically. Advanced migrations may edit it directly or call `successor`.
        """
        return self._successors

    def replace(
        self,
        old: Union[str, Mapping[str, Any]],
        replacement: Mapping[str, Any],
    ) -> dict[str, Any]:
        """
        Replace one resource's checkpoint entry with `replacement`, preserving its position in the
        working state. If its URN changes, record the replacement as the old resource's successor.

        This only replaces checkpoint state; it does not request a provider replacement.
        """
        index, current = self._find_resource(old)
        prepared = copy.deepcopy(dict(replacement))
        old_urn = self._resource_urn(current)
        new_urn = self._resource_urn(prepared)
        self.state[index] = prepared
        if old_urn != new_urn:
            self._record_successor(old_urn, new_urn)
        return prepared

    def rename(
        self,
        resource: Union[str, Mapping[str, Any]],
        new_urn: str,
        *,
        parent: Optional[Union[str, Mapping[str, Any]]] = ...,  # type: ignore
    ) -> dict[str, Any]:
        """
        Move a resource to `new_urn`, update its checkpoint type from that URN, and record its old
        URN as a predecessor. If `parent` is supplied, also move it below that resource; `None`
        removes its parent.
        """
        _, current = self._find_resource(resource)
        renamed = copy.deepcopy(current)
        renamed["urn"] = new_urn
        renamed["type"] = urn_util._parse_urn(new_urn).typ
        if parent is not ...:
            if parent is None:
                renamed.pop("parent", None)
            else:
                renamed["parent"] = self._resource_urn(parent)
        return self.replace(current, renamed)

    def merge(
        self,
        resource: Union[str, Mapping[str, Any]],
        *,
        into: Union[str, Mapping[str, Any]],
    ) -> dict[str, Any]:
        """
        Remove `resource` from the working state and record `into` as its successor. This expresses
        that the removed resource's identity or responsibility has been folded into a retained
        resource.
        """
        source_index, source = self._find_resource(resource)
        _, target = self._find_resource(into)
        source_urn = self._resource_urn(source)
        target_urn = self._resource_urn(target)
        if source_urn == target_urn:
            raise ValueError(f"cannot merge resource {source_urn} into itself")
        del self.state[source_index]
        self._record_successor(source_urn, target_urn)
        return target

    def successor(
        self,
        resource: Union[str, Mapping[str, Any]],
        successor: Union[str, Mapping[str, Any]],
    ) -> None:
        """
        Record a successor mapping directly. This is the raw escape hatch for state manipulations
        not expressed through `replace`, `rename`, or `merge`.
        """
        self._record_successor(
            self._resource_urn(resource), self._resource_urn(successor)
        )

    def result(self) -> StateMigrationResult:
        """
        Construct the callback result from the working state and successor mappings.

        Returning this value explicitly distinguishes a state-changing migration from the fast
        `None` no-op path.
        """
        new_urns: set[str] = set()
        for resource in self.state:
            urn = self._resource_urn(resource)
            if urn in new_urns:
                raise ValueError(f"state migration contains duplicate resource {urn}")
            new_urns.add(urn)

        for source, target in self.successors.items():
            if source not in self._old_urns:
                raise ValueError(
                    f"successor source {source} is not present in the prior state"
                )
            if source in new_urns:
                raise ValueError(
                    f"resource {source} is both returned and assigned successor {target}"
                )
            if target not in new_urns:
                raise ValueError(
                    f"successor {target} for resource {source} is not present in the new state"
                )

        for old_urn in self._old_urns:
            if old_urn not in new_urns and old_urn not in self.successors:
                raise ValueError(
                    f"resource {old_urn} is missing from the new state and has no successor"
                )

        return StateMigrationResult(
            new_state=self.state,
            successors=dict(self.successors) or None,
        )

    @staticmethod
    def _resource_urn(resource: Union[str, Mapping[str, Any]]) -> str:
        urn: Any
        if isinstance(resource, str):
            urn = resource
        else:
            urn = resource.get("urn")
        if not isinstance(urn, str) or not urn:
            raise ValueError(f"state migration resource has invalid URN {urn!r}")
        return urn

    def _find_resource(
        self, resource: Union[str, Mapping[str, Any]]
    ) -> tuple[int, dict[str, Any]]:
        urn = self._resource_urn(resource)
        matches = [
            (index, candidate)
            for index, candidate in enumerate(self.state)
            if candidate.get("urn") == urn
        ]
        if not matches:
            raise ValueError(f"resource {urn} is not present in the working state")
        if len(matches) > 1:
            raise ValueError(f"working state contains duplicate resource {urn}")
        return matches[0]

    def _record_successor(self, source: str, target: str) -> None:
        if source == target:
            raise ValueError(f"resource {source} cannot be its own successor")

        # Compose repeated helper operations so the result only maps resources from old_state
        # directly to resources in the final working state.
        predecessors = [
            predecessor
            for predecessor, current_target in self.successors.items()
            if current_target == source
        ]
        for predecessor in predecessors:
            self.successors[predecessor] = target

        if source in self._old_urns:
            existing = self.successors.get(source)
            if existing is not None and existing != target:
                raise ValueError(
                    f"resource {source} has conflicting successors {existing} and {target}"
                )
            self.successors[source] = target
        elif not predecessors:
            raise ValueError(
                f"successor source {source} is not present in the prior state"
            )
