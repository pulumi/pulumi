# Copyright 2024-2024, Pulumi Corporation.
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

import asyncio
import builtins
import tempfile
import time
import unittest
from unittest.mock import MagicMock, patch

from pulumi.automation._stack import Stack, _watch_logs
from pulumi.automation import EngineEvent, create_stack
import pytest
from .test_utils import stack_namer


class TestStack(unittest.IsolatedAsyncioTestCase):
    async def test_always_read_complete_lines(self):
        tmp = tempfile.NamedTemporaryFile(delete=False)

        async def write_lines(tmp):
            with open(tmp, "w") as f:
                parts = [
                    '{"stdoutEvent": ',
                    '{"message": "hello", "color": "blue"}',
                    "}\n",
                    '{"stdoutEvent": ',
                    '{"message": "world"',
                    ', "color": "red"}}\n',
                    '{"cancelEvent": {}}\n',
                ]
                for part in parts:
                    f.write(part)
                    f.flush()
                    time.sleep(0.2)

        write_task = asyncio.create_task(write_lines(tmp.name))
        event_num = 0

        def callback(event):
            nonlocal event_num
            if event_num == 0:
                self.assertTrue(event.stdout_event)
                self.assertEqual(event.stdout_event.message, "hello")
                self.assertEqual(event.stdout_event.color, "blue")
            elif event_num == 1:
                self.assertTrue(event.stdout_event)
                self.assertEqual(event.stdout_event.message, "world")
                self.assertEqual(event.stdout_event.color, "red")
            event_num += 1

        async def watch_async():
            _watch_logs(tmp.name, callback)

        watch_task = asyncio.create_task(watch_async())
        await asyncio.gather(write_task, watch_task)

    # This test was hanging forever before fixing a threading issue
    @pytest.mark.timeout(300)
    def test_operation_exception(self):
        def pulumi_program():
            pass

        project_name = "test_preview_errror"
        stack_name = stack_namer(project_name)
        stack = create_stack(
            stack_name, program=pulumi_program, project_name=project_name
        )

        # Passing an invalid color option will throw after we've setup the
        # log watcher thread, but before the actual Pulumi operation starts.
        # This means that we never send a CancelEvent to the events log.

        try:
            # Preview
            try:
                stack.preview(color="invalid color name")
                self.assertFalse(True, "should have thrown")
            except Exception as e:
                self.assertIn("unsupported color option", str(e))

            # Preview always starts the log watcher thread (to gather events for the summary),
            # but the other operations only do so if the `on_event` callback is provided.
            def on_event(event: EngineEvent):
                pass

            # Up
            try:
                stack.up(color="invalid color name", on_event=on_event)
                self.assertFalse(True, "should have thrown")
            except Exception as e:
                self.assertIn("unsupported color option", str(e))

            # Refresh
            try:
                stack.refresh(color="invalid color name", on_event=on_event)
                self.assertFalse(True, "should have thrown")
            except Exception as e:
                self.assertIn("unsupported color option", str(e))

            # Destroy
            try:
                stack.destroy(color="invalid color name", on_event=on_event)
                self.assertFalse(True, "should have thrown")
            except Exception as e:
                self.assertIn("unsupported color option", str(e))

        finally:
            stack.workspace.remove_stack(stack_name)


class TestStackArgOrdering(unittest.TestCase):
    """Tests for _run_pulumi_cmd_sync handling of -- separator."""

    def _create_mock_stack(self, additional_args=None):
        """Helper to create a mock stack for testing."""
        from pulumi.automation._local_workspace import LocalWorkspace

        mock_workspace = MagicMock()
        mock_workspace.serialize_args_for_op.return_value = additional_args or []
        mock_workspace.pulumi_home = None
        mock_workspace.env_vars = {}
        mock_workspace.pulumi_command.run.return_value = MagicMock(
            stdout="", stderr="", code=0
        )
        # _remote property on Stack checks isinstance(workspace, LocalWorkspace)
        # and then reads workspace._remote. By setting this, the property returns False.
        mock_workspace._remote = False

        with patch.object(Stack, "__init__", lambda self, *args, **kwargs: None):
            stack = Stack.__new__(Stack)
            stack.name = "test-stack"
            stack.workspace = mock_workspace

        # Patch isinstance to make it think mock_workspace is a LocalWorkspace
        # This is needed because Stack._remote property checks isinstance
        original_isinstance = builtins.isinstance

        def patched_isinstance(obj, classinfo):
            if obj is mock_workspace and classinfo is LocalWorkspace:
                return True
            return original_isinstance(obj, classinfo)

        patcher = patch.object(builtins, "isinstance", patched_isinstance)
        patcher.start()
        self.addCleanup(patcher.stop)

        return stack, mock_workspace

    def test_stack_arg_inserted_before_separator(self):
        """
        Test that --stack is inserted before the -- separator when present.

        This is critical for commands like import_resources with converters,
        where arguments after -- are passed to the converter plugin.
        """
        stack, mock_workspace = self._create_mock_stack()

        # Simulate args from import_resources with converter
        args = [
            "import",
            "--yes",
            "--skip-preview",
            "--from",
            "terraform",
            "--",
            "/path/to/file.json",
        ]

        stack._run_pulumi_cmd_sync(args)

        # Verify the command was called with --stack before --
        called_args = mock_workspace.pulumi_command.run.call_args[0][0]

        separator_index = called_args.index("--")
        stack_index = called_args.index("--stack")

        self.assertLess(
            stack_index,
            separator_index,
            f"--stack should appear before -- separator. Got: {' '.join(called_args)}",
        )

    def test_stack_arg_appended_when_no_separator(self):
        """
        Test that --stack is appended normally when no -- separator is present.
        """
        stack, mock_workspace = self._create_mock_stack()

        # Simulate args without -- separator
        args = ["up", "--yes", "--skip-preview"]

        stack._run_pulumi_cmd_sync(args)

        called_args = mock_workspace.pulumi_command.run.call_args[0][0]

        # --stack should be at the end
        self.assertEqual(
            called_args[-2:],
            ["--stack", "test-stack"],
            f"--stack test-stack should be at end. Got: {' '.join(called_args)}",
        )

    def test_additional_args_also_inserted_before_separator(self):
        """
        Test that additional args from serialize_args_for_op are also
        inserted before the -- separator.
        """
        stack, mock_workspace = self._create_mock_stack(
            additional_args=["--config-file", "Pulumi.yaml"]
        )

        args = ["import", "--from", "terraform", "--", "/path/to/file.json"]

        stack._run_pulumi_cmd_sync(args)

        called_args = mock_workspace.pulumi_command.run.call_args[0][0]

        separator_index = called_args.index("--")
        config_file_index = called_args.index("--config-file")

        self.assertLess(
            config_file_index,
            separator_index,
            f"--config-file should appear before -- separator. Got: {' '.join(called_args)}",
        )

    def test_converter_args_remain_after_separator(self):
        """
        Test that converter args stay after the -- separator.
        """
        stack, mock_workspace = self._create_mock_stack()

        converter_file = "/path/to/terraform_statefile.json"
        args = ["import", "--from", "terraform", "--", converter_file]

        stack._run_pulumi_cmd_sync(args)

        called_args = mock_workspace.pulumi_command.run.call_args[0][0]

        separator_index = called_args.index("--")

        # The converter file should be the only thing after --
        args_after_separator = called_args[separator_index + 1 :]
        self.assertEqual(
            args_after_separator,
            [converter_file],
            f"Only converter args should be after --. Got: {' '.join(called_args)}",
        )
