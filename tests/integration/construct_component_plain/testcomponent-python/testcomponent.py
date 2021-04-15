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

from typing import Any, Optional
import sys

from pulumi import Input, Inputs, ComponentResource, ResourceOptions
import pulumi
import pulumi.dynamic as dynamic
import pulumi.provider as provider


_ID = 0


class MyDynamicProvider(dynamic.ResourceProvider):

    def create(self, props: Any) -> dynamic.CreateResult:
        global _ID
        _ID = _ID + 1
        return dynamic.CreateResult(id_=str(_ID))


class Resource(dynamic.Resource):
    def __init__(self, name: str, options: Optional[ResourceOptions]=None):
        super().__init__(MyDynamicProvider(), name, {}, options)


class Component(ComponentResource):
    def __init__(self, name: str, children: int, options: Optional[ResourceOptions] = None):
        super().__init__('testcomponent:index:Component', name, {}, options)

        for i in range(0, children):
            Resource(f'child-{name}-{i+1}', options=ResourceOptions(parent=self))


class Provider(provider.Provider):
    VERSION = "0.0.1"

    def __init__(self):
        super().__init__(Provider.VERSION)

    def construct(self,
                  name: str,
                  resource_type: str,
                  inputs: Inputs,
                  options: Optional[ResourceOptions] = None) -> provider.ConstructResult:

        if resource_type != 'testcomponent:index:Component':
            raise Exception('unknown resource type {}'.format(resource_type))

        component = Component(name,
                              children=int(inputs.get('children', 0)),
                              options=options)

        return provider.ConstructResult(urn=component.urn, state={})


if __name__ == '__main__':
    provider.main(Provider(), sys.argv[1:])
