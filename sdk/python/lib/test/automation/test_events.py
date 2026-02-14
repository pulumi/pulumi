"""Tests for automation event deserialization."""

import unittest

from pulumi.automation.events import (
    DiffKind,
    PolicyEvent,
    PropertyDiff,
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


class TestPolicyEventFromJson(unittest.TestCase):
    """Test PolicyEvent.from_json reads camelCase keys matching Go JSON tags."""

    def test_resource_urn_deserialized(self):
        """resourceUrn (camelCase from Go engine) should be deserialized into resource_urn."""
        data = {
            "resourceUrn": "urn:pulumi:stack::project::type::name",
        }
        event = PolicyEvent.from_json(data)
        assert event.resource_urn == "urn:pulumi:stack::project::type::name"
