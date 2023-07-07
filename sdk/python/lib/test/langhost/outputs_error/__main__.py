# Copyright 2016-2023, Pulumi Corporation.
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

# This is _convoluted_ but it seems the only way to repro https://github.com/pulumi/pulumi/issues/10637 is to
# have an invoke that is used as an input to a resource, which you then call apply on and have that raise an
# exception. Trying to write this test without the invoke or resource doesn't repro the issue that the
# exception gets silently swallowed. A lot of the code here is copied and simplified from the python aws and
# awsx SDKs.

@pulumi.output_type
class GetRegionResult:
    def __init__(__self__, name=None):
        if name and not isinstance(name, str):
            raise TypeError("Expected argument 'name' to be a str")
        pulumi.set(__self__, "name", name)

    @property
    @pulumi.getter
    def name(self) -> str:
        return pulumi.get(self, "name")


class AwaitableGetRegionResult(GetRegionResult):
    def __await__(self):
        if False:
            yield self
        return GetRegionResult(name=self.name)

def get_region(opts = None):
    __args__ = dict()
    __ret__ = pulumi.runtime.invoke('aws:index/getRegion:getRegion', __args__, opts=opts, typ=GetRegionResult).value

    return AwaitableGetRegionResult(name=__ret__.name)


@pulumi.input_type
class MyComponentArgs:
    pass

class MyComponent(pulumi.ComponentResource):
    number: pulumi.Output[float]

    def __init__(self, name, input, opts=None):
        __props__ = MyComponentArgs.__new__(MyComponentArgs)
        __props__.__dict__["input"] = input
        __props__.__dict__["number"] = None

        super(MyComponent, self).__init__(
            "test:index:MyComponent",
            name,
            __props__,
            opts=opts,
            remote=True,
        )

    @property
    @pulumi.getter(name="number")
    def number(self) -> pulumi.Output[float]:
        return pulumi.get(self, "number")


def raise_error(value):
    raise Exception("this is an error %s" % value)

region = get_region()
res1 = MyComponent("testResource1", input=region.name)
res1.number.apply(raise_error)
