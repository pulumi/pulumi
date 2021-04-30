# Copyright 2016-2021, Pulumi Corporation.
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

from . import outputs


class MyResource(pulumi.CustomResource):
    def __init__(self, name):
        @pulumi.input_type
        class Args:
            pass
        props = Args()
        props.__dict__["foo"] = None
        super().__init__("test:index:MyResource", name, props)

    @property
    @pulumi.getter
    def foo(self) -> pulumi.Output['outputs.MyResourceFoo']:
        return pulumi.get(self, "foo")

