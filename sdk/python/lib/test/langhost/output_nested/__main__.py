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
    nested_numbers: Output[dict]

    def __init__(self, name):
        CustomResource.__init__(self, "test:index:MyResource", name, {
            "nested_numbers": None,
        })


class SumResource(CustomResource):
    sum: Output[int]

    def __init__(self, name, sum):
        CustomResource.__init__(self, "test:index:SumResource", name, {
            "sum": sum,
        })


res1 = MyResource("testResource1")
res2 = MyResource("testResource2")

sum = Output.from_input(res1.nested_numbers).apply(lambda d: d["foo"]["bar"] + d["baz"])
sumRes = SumResource("sumResource", sum)
