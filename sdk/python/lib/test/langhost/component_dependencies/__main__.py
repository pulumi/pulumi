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
from pulumi import ComponentResource, CustomResource, Output, ResourceOptions


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


class MyComponent(ComponentResource):
    def __init__(self, name, opts=None):
        ComponentResource.__init__(
            self, "test:index:MyComponent", name, props={}, opts=opts
        )


resA = MyResource("resA", {})
comp1 = MyComponent("comp1")
resB = MyResource("resB", {}, ResourceOptions(parent=comp1))
resC = MyResource("resC", {}, ResourceOptions(parent=resB))
comp2 = MyComponent("comp2", ResourceOptions(parent=comp1))

resD = MyResource("resD", {"propA": resA}, ResourceOptions(parent=comp2))
resE = MyResource("resE", {"propA": resD}, ResourceOptions(parent=comp2))

resF = MyResource("resF", {"propA": resA})
resG = MyResource("resG", {"propA": comp1})
resH = MyResource("resH", {"propA": comp2})
resI = MyResource("resI", {"propA": resG})
resJ = MyResource("resJ", {}, ResourceOptions(depends_on=[comp2]))

first = MyComponent("first")
firstChild = MyResource("firstChild", {}, ResourceOptions(parent=first))
second = MyComponent("second", ResourceOptions(parent=first, depends_on=[first]))
myresource = MyResource("myresource", {}, ResourceOptions(parent=second))
