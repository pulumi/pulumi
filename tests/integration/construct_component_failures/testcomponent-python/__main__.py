# Copyright 2016-2024, Pulumi Corporation.
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

from pulumi import Input, Inputs, ComponentResource, ResourceOptions, InputPropertiesError
import pulumi
import pulumi.provider as provider
import grpc

class Component(pulumi.ComponentResource):
    def __init__(self,
                 resource_name: str,
                 foo: Input[str],
                 opts: Optional[pulumi.ResourceOptions] = None) -> None:
        super().__init__("testcomponent:index:Component", resource_name, {}, opts)
        self.foo = pulumi.Output.from_input(foo)
        self.register_outputs({
            'foo': self.foo
        })

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

        component = Component(name, inputs['foo'], options)

        raise InputPropertiesError("failing for a reason", [{"property_path": "foo", "reason": "the failure reason"}])

if __name__ == "__main__":
    provider.main(Provider(), sys.argv[1:])
