# Copyright 2016-2019, Pulumi Corporation.
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
from pulumi import CustomResource, ResourceOptions

CustomResource("test:read:resource", "foo", {
    "a": "bar",
    "b": ["c", 4, "d"],
    "c": {
        "nest": "baz"
    }
}, opts=ResourceOptions(id="myresourceid", version="0.17.9"))

parent = CustomResource("test:index:MyResource", "foo2")
CustomResource("test:read:resource", "foo-with-parent", {
    "state": "foo",
}, opts=ResourceOptions(id="myresourceid2", version="0.17.9", parent=parent))
