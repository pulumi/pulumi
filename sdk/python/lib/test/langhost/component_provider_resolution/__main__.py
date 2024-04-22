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
import functools
from pulumi import (
    ComponentResource,
    CustomResource,
    Output,
    ResourceOptions,
    ProviderResource,
)


class MyResource(CustomResource):
    def __init__(self, name, args, opts=None):
        CustomResource.__init__(
            self,
            "test:index:MyResource",
            name,
            props={
                **args,
                "outprop": None,
            },
            opts=opts,
        )


class OtherResource(CustomResource):
    def __init__(self, name, args, opts=None):
        super().__init__(
            "other:index:OtherResource",
            name,
            props={
                **args,
                "outprop": None,
            },
            opts=opts,
        )


class OtherProvider(ProviderResource):
    def __init__(self, name):
        super().__init__("other", name)


class CombinedComponent(ComponentResource):
    def __init__(self, name, opts=None):
        super().__init__("combined:index:CombinedResource", name, opts=opts)
        parent = ResourceOptions(parent=self).merge(opts)
        MyResource("combined-mine", {}, opts=parent)
        OtherResource("combined-other", {}, opts=parent)


class MyComponent(ComponentResource):
    def __init__(self, name, opts=None):
        ComponentResource.__init__(self, "test:index:MyComponent", name, opts=opts)


prov1 = OtherProvider("prov1")
comp3 = CombinedComponent(
    "comp3",
    ResourceOptions(
        providers={"other": prov1},
        protect=True,
    ),
)
