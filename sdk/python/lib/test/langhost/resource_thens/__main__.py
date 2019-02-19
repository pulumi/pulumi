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
from functools import partial
from pulumi import CustomResource, Output

def assert_eq(l, r):
    assert l == r

class ResourceA(CustomResource):
    inprop: Output[int]
    outprop: Output[str]

    def __init__(self, name: str) -> None:
        CustomResource.__init__(self, "test:index:ResourceA", name, {
            "inprop": 777,
            "outprop": None
        })

class ResourceB(CustomResource):
    other_in: Output[int]
    other_out: Output[str]

    def __init__(self, name: str, res: ResourceA) -> None:
        CustomResource.__init__(self, "test:index:ResourceB", name, {
            "other_in": res.inprop,
            "other_out": res.outprop,
            "other_id": res.id,
        })

a = ResourceA("resourceA")
a.urn.apply(lambda urn: assert_eq(urn, "test:index:ResourceA::resourceA"))
a.inprop.apply(lambda v: assert_eq(v, 777))
a.outprop.apply(lambda v: assert_eq(v, "output yeah"))

b = ResourceB("resourceB", a)
b.urn.apply(lambda urn: assert_eq(urn, "test:index:ResourceB::resourceB"))
b.other_in.apply(lambda v: assert_eq(v, 777))
b.other_out.apply(lambda v: assert_eq(v, "output yeah"))
