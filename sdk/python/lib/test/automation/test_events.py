"""Tests for automation event deserialization."""

import unittest

from pulumi.automation.events import (
    DiffKind,
    DiagnosticEvent,
    PolicyEvent,
    PropertyDiff,
    ResOpFailedEvent,
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


class TestResOpFailedEventFromJson(unittest.TestCase):
    def test_awaited_deserialized(self):
        data = {"metadata": {"op": "create"}}
        assert ResOpFailedEvent.from_json({**data, "awaited": True}).awaited is True
        assert ResOpFailedEvent.from_json(data).awaited is False


class TestDiagnosticSourceContext(unittest.TestCase):
    def test_source_context_deserialized(self):
        event = DiagnosticEvent.from_json(
            {
                "message": "invalid port",
                "severity": "error",
                "color": "never",
                "sourceContext": {
                    "sourcePosition": "project:///main.py#2",
                    "stackTrace": [
                        {
                            "sourcePosition": "project:///main.py#2",
                            "module": "__main__",
                            "origin": "pulumi",
                            "boundary": {
                                "kind": "program",
                                "callSite": "project:///main.py#2",
                            },
                        },
                        {},
                    ],
                    "rendered": "Registered in your program\n  main.py:2",
                },
            }
        )
        assert event.message == "invalid port"
        context = event.source_context
        assert context is not None
        assert context.source_position == "project:///main.py#2"
        assert context.stack_trace is not None
        assert context.stack_trace[0].source_position == "project:///main.py#2"
        assert context.stack_trace[0].module == "__main__"
        assert context.stack_trace[0].origin == "pulumi"
        assert context.stack_trace[0].boundary == {
            "kind": "program",
            "callSite": "project:///main.py#2",
        }
        assert context.stack_trace[1].source_position is None
        assert context.rendered == "Registered in your program\n  main.py:2"

    def test_older_event_without_source_context(self):
        assert (
            DiagnosticEvent.from_json({"message": "invalid port"}).source_context
            is None
        )
