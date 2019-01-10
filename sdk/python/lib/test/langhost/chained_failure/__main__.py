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
from pulumi import CustomResource, Output, Input


class ResourceA(CustomResource):
    inprop: Output[int]
    outprop: Output[int]

    def __init__(self, name: str) -> None:
        CustomResource.__init__(self, "test:index:ResourceA", name, {
            "inprop": 777,
            "outprop": None
        })

class ResourceB(CustomResource):
    other_in: Output[int]
    other_out: Output[str]

    def __init__(self, name: str, res: Input[int]) -> None:
        CustomResource.__init__(self, "test:index:ResourceB", name, {
            "other_in": res.inprop,
            "other_out": None
        })

a = ResourceA("resourceA")
# Dividing by zero always throws an exception. This will throw whenever ResourceB tries to prepare itself
# for the RegisterResource RPC.
b_value = a.outprop.apply(lambda number: number / 0)
b = ResourceB("resourceB", b_value)

# C depends on B, but B's outputs will never resolve since B fails to initialize.
# This should NOT deadlock. (pulumi/pulumi#2189)
c = ResourceB("resourceC", b.other_out)
