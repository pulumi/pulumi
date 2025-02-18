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
from pulumi import ComponentResource, CustomResource


class MyResource(CustomResource):
    def __init__(self, name, opts=None):
        self.__internal_init__(name, opts)

    def __internal_init__(self, name, opts):
        CustomResource.__init__(
            self, "test:index:MyResource", name, props={}, opts=opts
        )


class MyComponent(ComponentResource):
    def __init__(self, name, opts=None):
        ComponentResource.__init__(
            self, "test:index:MyComponent", name, props={}, opts=opts
        )


custom = MyResource("custom")
component = MyComponent("component")
