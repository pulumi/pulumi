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
from pulumi import ProviderResource, ComponentResource, CustomResource, Output, InvokeOptions, ResourceOptions, log
from pulumi.runtime import invoke

def assert_eq(l, r):
    assert l == r


class MyResource(CustomResource):
    value: Output[int]

    def __init__(self, name, value, opts=None):
        CustomResource.__init__(self, "test:index:MyResource", name, props={
            "value": value,
        }, opts=opts)

class MyProvider(ProviderResource):
    def __init__(self, name, opts=None):
        ProviderResource.__init__(self, "test", name, {}, opts)


class MyComponent(ComponentResource):
    def __init__(self, name, opts=None):
        ComponentResource.__init__(self, "test:index:MyComponent", name, {}, opts)


# Explicitly use a provider for an Invoke.
prov = MyProvider("testprov")
async def do_provider_invoke():
    value = await invoke("test:index:MyFunction", props={"value": 9000}, opts=InvokeOptions(provider=prov))
    return value["value"]

res = MyResource("resourceA", do_provider_invoke())
res.value.apply(lambda v: assert_eq(v, 9001))
# The Invoke RPC call should contain a reference to prov.


# Implicitly use a provider for an Invoke by passing a parent to InvokeOptions. The parent's provider is used when
# performing the invoke.
componentRes = MyComponent("resourceB", opts=ResourceOptions(providers={"test": prov}))

async def do_provider_invoke_with_parent(parent):
    value = await invoke("test:index:MyFunctionWithParent", props={"value": 41}, opts=InvokeOptions(parent=parent))
    return value["value"]

res2 = MyResource("resourceC", do_provider_invoke_with_parent(componentRes))
res2.value.apply(lambda v: assert_eq(v, 42))
# The Invoke RPC call should again contain a reference to prov.
