"""Tests for automation event deserialization."""

import unittest

from pulumi.automation.events import (
    PolicyEvent,
    PropertyDiff,
    StepEventMetadata,
)


class TestStepEventMetadataFromJson(unittest.TestCase):
    """Test StepEventMetadata.from_json reads camelCase keys matching Go JSON tags."""

    def test_detailed_diff_camel_case_key(self):
        """detailedDiff (camelCase from Go engine) should be deserialized."""
        data = {
            "op": "update",
            "urn": "urn:pulumi:stack::project::type::name",
            "type": "type",
            "provider": "provider",
            "detailedDiff": {
                "tags": {"diffKind": "update", "inputDiff": False},
                "name": {"diffKind": "update", "inputDiff": True},
            },
        }
        meta = StepEventMetadata.from_json(data)
        self.assertIsNotNone(meta.detailed_diff)
        self.assertIn("tags", meta.detailed_diff)
        self.assertIn("name", meta.detailed_diff)

    def test_detailed_diff_values_are_property_diff(self):
        """detailedDiff values should be deserialized into PropertyDiff objects."""
        data = {
            "op": "update",
            "urn": "urn:pulumi:stack::project::type::name",
            "type": "type",
            "provider": "provider",
            "detailedDiff": {
                "tags": {"diffKind": "update", "inputDiff": False},
            },
        }
        meta = StepEventMetadata.from_json(data)
        self.assertIsInstance(meta.detailed_diff["tags"], PropertyDiff)

    def test_detailed_diff_none_when_absent(self):
        """detailed_diff should be None when detailedDiff is not in the data."""
        data = {
            "op": "create",
            "urn": "urn:pulumi:stack::project::type::name",
            "type": "type",
            "provider": "provider",
        }
        meta = StepEventMetadata.from_json(data)
        self.assertIsNone(meta.detailed_diff)


class TestPolicyEventFromJson(unittest.TestCase):
    """Test PolicyEvent.from_json reads camelCase keys matching Go JSON tags."""

    def test_resource_urn_camel_case_key(self):
        """resourceUrn (camelCase from Go engine) should be deserialized."""
        data = {
            "message": "test",
            "color": "",
            "policyName": "policy",
            "policyPackName": "pack",
            "policyPackVersion": "1",
            "policyPackVersionTag": "v1",
            "enforcementLevel": "mandatory",
            "resourceUrn": "urn:pulumi:stack::project::type::name",
        }
        event = PolicyEvent.from_json(data)
        self.assertEqual(event.resource_urn, "urn:pulumi:stack::project::type::name")
