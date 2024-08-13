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
from typing import Optional
import pulumi


@pulumi.input_type
class MyResourceArgs:
    def __init__(__self__, *, in_prop: Optional[pulumi.Input[str]] = None):
        if in_prop is not None:
            pulumi.set(__self__, "in_prop", in_prop)

    @property
    @pulumi.getter(name="inProp")
    def in_prop(self) -> Optional[pulumi.Input[str]]:
        return pulumi.get(self, "in_prop")

    @in_prop.setter
    def in_prop(self, value: pulumi.Input[str]):
        pulumi.set(self, "in_prop", value)


class MyResource(pulumi.ComponentResource):
    @property
    @pulumi.getter(name="outProp")
    def out_prop(self) -> pulumi.Output[str]:
        return pulumi.get(self, "out_prop")

    def __init__(self, name, args, opts=None):
        args.__dict__["out_prop"] = None
        pulumi.ComponentResource.__init__(
            self, "test:index:MyResource", name, props=args, opts=opts, remote=True
        )


resA = MyResource("resA", MyResourceArgs())
# resB is not registered, but is used as a dependency of A's output property
resC = MyResource("resC", MyResourceArgs(in_prop=resA.out_prop))
