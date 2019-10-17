# Copyright 2016-2018, Pulumi Corporation.
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
from pulumi import CustomResource, Output, Input

async def read_a_file_or_something():
    await asyncio.sleep(0)
    return "here's a file"

def assert_eq(l, r):
    assert l == r

class FileResource(CustomResource):
    contents: Output[str]

    def __init__(self, name: str, file_contents: Input[str]) -> None:
        CustomResource.__init__(self, "test:index:FileResource", name, {
            "contents": file_contents
        })

# read_a_file_or_something returns a coroutine when called, which needs to be scheduled
# and awaited in order to yield a value.
file_res = FileResource("file", read_a_file_or_something())
file_res.contents.apply(lambda c: assert_eq(c, "here's a file"))
