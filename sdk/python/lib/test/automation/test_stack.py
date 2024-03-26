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

import tempfile

class TestStack(unittest.TestCase):
    def test_always_read_complete_lines(self):
        tempfile = tempfile.NamedTemporaryFile()
        async def write_lines():
            with tempfile as f:
                parts = ['{"stdoutEvent": ', '{"message": "hello", "color": "blue"}', '}\n', '{"stdoutEvent": ', '{"message": "world"', ', "color": "red"}}\n']
                for part in parts:
                    f.write(part)
                    await asyncio.sleep(0.2)
        write_task = asyncio.create_task(write_lines())
        event_num = 0
        def callback(event):
            if event_num == 0:
                self.assertEqual(event, {"message": "hello", "color": "blue"})
            elif event_num == 1:
                self.assertEqual(event, {"message": "world", "color": "red"})

        _watch_logs(tempfile.name, callback)
