"""Tests for automation event deserialization."""

import unittest

from pulumi.automation.events import (
    DiffKind,
    EngineEvent,
    PolicyEvent,
    PropertyDiff,
    StateMigrationEvent,
    StepEventMetadata,
)


class TestStepEventMetadataFromJson(unittest.TestCase):
    """Test StepEventMetadata.from_json reads camelCase keys matching Go JSON tags."""

    def test_detailed_diff_deserialized(self):
        """detailedDiff (camelCase from Go engine) should be deserialized into PropertyDiff objects."""
        data = {
            "op": "update",
            "detailedDiff": {
                "tags": {"diffKind": "update", "inputDiff": False},
                "name": {"diffKind": "update", "inputDiff": True},
            },
        }
        meta = StepEventMetadata.from_json(data)
        assert meta.detailed_diff is not None
        assert len(meta.detailed_diff) == 2
        assert isinstance(meta.detailed_diff["tags"], PropertyDiff)
        assert meta.detailed_diff["tags"].diff_kind == DiffKind.UPDATE
        assert meta.detailed_diff["tags"].input_diff is False
        assert isinstance(meta.detailed_diff["name"], PropertyDiff)
        assert meta.detailed_diff["name"].diff_kind == DiffKind.UPDATE
        assert meta.detailed_diff["name"].input_diff is True

    def test_detailed_diff_null_deserialized(self):
        """detailedDiff can be null in engine events and should deserialize as an empty map."""
        data = {
            "op": "create",
            "detailedDiff": None,
        }
        meta = StepEventMetadata.from_json(data)
        assert meta.detailed_diff == {}


class TestPolicyEventFromJson(unittest.TestCase):
    """Test PolicyEvent.from_json reads camelCase keys matching Go JSON tags."""

    def test_resource_urn_deserialized(self):
        """resourceUrn (camelCase from Go engine) should be deserialized into resource_urn."""
        data = {
            "resourceUrn": "urn:pulumi:stack::project::type::name",
        }
        event = PolicyEvent.from_json(data)
        assert event.resource_urn == "urn:pulumi:stack::project::type::name"


class TestStateMigrationEventFromJson(unittest.TestCase):
    """Test state migration payloads are selected and decoded by EngineEvent."""

    def test_state_migration_event_deserialized(self):
        old_urn = "urn:pulumi:stack::project::old:index:Resource::name"
        new_urn = "urn:pulumi:stack::project::new:index:Resource::name"
        event = EngineEvent.from_json(
            {
                "sequence": 7,
                "timestamp": 42,
                "stateMigrationEvent": {
                    "urn": "urn:pulumi:stack::project::component:index:Component::name",
                    "migrated": 2,
                    "added": [new_urn],
                    "removed": [old_urn],
                    "successors": {old_urn: new_urn},
                },
            }
        )

        assert isinstance(event.state_migration_event, StateMigrationEvent)
        assert event.state_migration_event.migrated == 2
        assert event.state_migration_event.added == [new_urn]
        assert event.state_migration_event.removed == [old_urn]
        assert event.state_migration_event.successors == {old_urn: new_urn}
