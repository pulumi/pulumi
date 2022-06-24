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

from typing import Optional
import sys

import pulumi
import pulumi.provider as provider

from random_ import Random


class Component(pulumi.ComponentResource):
    def __init__(self,
                 resource_name: str,
                 opts: Optional[pulumi.ResourceOptions] = None) -> None:
        super().__init__("testcomponent:index:Component", resource_name, {}, opts)

    def create_random(self, length: pulumi.Input[int]) -> pulumi.Output[str]:
        r = Random("myrandom", length=length, opts=pulumi.ResourceOptions(parent=self))
        return r.result

class Provider(provider.Provider):
    VERSION = "0.0.1"

    class Module(pulumi.runtime.ResourceModule):
        def version(self):
            return Provider.VERSION

        def construct(self, name: str, typ: str, urn: str) -> pulumi.Resource:
            if typ == "testcomponent:index:Component":
                return Component(name, pulumi.ResourceOptions(urn=urn))
            else:
                raise Exception(f"unknown resource type {typ}")

    def __init__(self):
        super().__init__(Provider.VERSION)
        pulumi.runtime.register_resource_module("testcomponent", "index", Provider.Module())

    def construct(self, name: str, resource_type: str, inputs: pulumi.Inputs,
                  options: Optional[pulumi.ResourceOptions] = None) -> provider.ConstructResult:

        if resource_type != "testcomponent:index:Component":
            raise Exception(f"unknown resource type {resource_type}")

        component = Component(name, options)

        return provider.ConstructResult(
            urn=component.urn,
            state=inputs)

    def call(self, token: str, args: pulumi.Inputs) -> provider.CallResult:
        if token != "testcomponent:index:Component/createRandom":
            raise Exception(f'unknown method {token}')

        comp: Component = args["__self__"]
        outputs = {
            "result": comp.create_random(args["length"])
        }
        return provider.CallResult(outputs=outputs)


if __name__ == "__main__":
    provider.main(Provider(), sys.argv[1:])
