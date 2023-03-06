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

from pulumi import ComponentResource, Inputs, Output, ResourceOptions
import pulumi.provider as provider


def panic(text: str):
    print(text)
    sys.exit(1)

class Component(ComponentResource):
    def __init__(self, name: str, inputs: Inputs, opts: Optional[ResourceOptions]=None):
        super().__init__('testcomponent:index:Component', name, {}, opts)
        Output.from_input(inputs['message']).apply(lambda v: panic("should not run (message)"))
        Output.from_input(inputs['nested']).apply(lambda v: panic("should not run (nested)"))


class Provider(provider.Provider):
    VERSION = "0.0.1"

    def __init__(self):
        super().__init__(Provider.VERSION)

    def construct(self, name: str, resource_type: str, inputs: Inputs,
                  options: Optional[ResourceOptions]=None) -> provider.ConstructResult:

        if resource_type != 'testcomponent:index:Component':
            raise Exception('unknown resource type {}'.format(resource_type))

        component = Component(name, inputs, options)

        return provider.ConstructResult(
            urn=component.urn,
            state={})


if __name__ == '__main__':
    provider.main(Provider(), sys.argv[1:])
