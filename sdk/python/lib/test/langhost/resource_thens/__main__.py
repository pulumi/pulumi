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
from pulumi import CustomResource
from pulumi.runtime import Unknown


class MyResource(CustomResource):
    def __init__(self, name):
        CustomResource.__init__(self, "test:index:MyResource", name, props={
            "foo": "bar"
        })

    def set_outputs(self, outputs):
        self.outprop = Unknown()
        self.stable = Unknown()
        if "outprop" in outputs:
            self.outprop = outputs["outprop"]

        if "stable" in outputs:
            self.stable = outputs["stable"]


   
class OtherResource(CustomResource):
    def __init__(self, name, props):
        CustomResource.__init__(self, "test:index:OtherResource", name, props=props)

    def set_outputs(self, outputs):
        self.inprop = Unknown()
        self.stable = Unknown()
        if "inprop" in outputs:
            self.inprop = outputs["inprop"]

        if "stable" in outputs:
            self.stable = outputs["stable"]

a = MyResource("first")
assert a.stable == "yeah"
b = OtherResource("second", {
    "inprop": a.outprop
})
assert b.inprop == a.outprop
assert b.stable == "yeah"
