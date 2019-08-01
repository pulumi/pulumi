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
import pulumi

class TestClass:
    def __init__(self):
        self.num = 1
        self._private = 2

recursive = {"a": 1}
recursive["b"] = 2
recursive["c"] = recursive

pulumi.export("string", "pulumi")
pulumi.export("number", 1)
pulumi.export("boolean", True)
pulumi.export("list", [])
pulumi.export("list_with_none", [None])
pulumi.export("list_of_lists", [[], []])
pulumi.export("set", set(["val"]))
pulumi.export("dict", {"a": 1})
pulumi.export("output", pulumi.Output.from_input(1))
pulumi.export("class", TestClass())
pulumi.export("recursive", recursive)
