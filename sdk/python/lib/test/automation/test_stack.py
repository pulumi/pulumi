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
from pulumi.automation import EngineEvent, StdoutEngineEvent


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
