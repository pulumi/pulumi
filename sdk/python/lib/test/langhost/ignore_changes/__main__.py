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
from pulumi import CustomResource, ResourceOptions, InvokeOptions
from pulumi.runtime import invoke


class MyResource(CustomResource):
    def __init__(self, name, opts=None):
        CustomResource.__init__(self, "test:index:MyResource", name, opts=opts)

    def translate_input_property(self, prop: str) -> str:
        if prop == "ignored_property":
            return "ignoredProperty"
        
        return prop


res = MyResource("testResource", opts=ResourceOptions(ignore_changes=["ignored_property", "ignored_property_other"]))
