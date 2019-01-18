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
import functools
from pulumi import CustomResource, Output, ResourceOptions

class MyResource(CustomResource):
    def __init__(self, name, args, opts=None):
        CustomResource.__init__(self, "test:index:MyResource", name, props={
            **args,
            "outprop": None,
        }, opts=opts)

resA = MyResource("resA", {})
resB = MyResource("resB", {}, ResourceOptions(depends_on=[ resA ]))
resC = MyResource("resC", {
    "propA": resA.outprop,
    "propB": resB.outprop,
    "propC": "foo",
});
resD = MyResource("resD", {
    "propA": Output.all([resA.outprop, resB.outprop]).apply(lambda l: f"{l}"),
    "propB": resC.outprop,
    "propC": "bar",
})
resE = MyResource("resE", {
    "propA": resC.outprop,
    "propB": Output.all([resA.outprop, resB.outprop]).apply(lambda l: f"{l}"),
    "propC": "baz",
}, ResourceOptions(depends_on=[ resD ]))
