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
import os
import tempfile
import time
import unittest

from pulumi.automation._stack import _watch_logs
from pulumi.automation import EngineEvent, StdoutEngineEvent, create_stack
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

            # Refres
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
