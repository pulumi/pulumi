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
from pulumi import CustomResource, Output, log
from pulumi.runtime import invoke

def assert_eq(l, r):
    assert l == r


class MyResource(CustomResource):
    value: Output[int]

    def __init__(self, name, value):
        CustomResource.__init__(self, "test:index:MyResource", name, props={
            "value": value,
        })


value = invoke("test:index:MyFunction", props={
    "value": 41,
})

async def do_invoke():
    value = await invoke("test:index:MyFunction", props={"value": 41})
    return value["value"]

res = MyResource("resourceA", do_invoke())
res.value.apply(lambda v: assert_eq(v, 42))
