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
from pulumi import Output, CustomResource

class MyResource(CustomResource):
    number: Output[str]

    def __init__(self, name):
        CustomResource.__init__(self, "test:index:MyResource", name, {
            "number": None,
        })

class FinalResource(CustomResource):
    number: Output[str]

    def __init__(self, name, number):
        CustomResource.__init__(self, "test:index:FinalResource", name, {
            "number": number,
        })


def assert_eq(lhs, rhs):
    assert lhs == rhs


res1 = MyResource("testResource1")
res2 = MyResource("testResource2")

res1.number.apply(lambda n: assert_eq(n, 2))
res2.number.apply(lambda n: assert_eq(n, 3))

# Output.all combines a list of outputs into an output of a list.
resSum = Output.all(res1.number, res2.number).apply(lambda l: l[0] + l[1])
FinalResource("testResource3", resSum)
