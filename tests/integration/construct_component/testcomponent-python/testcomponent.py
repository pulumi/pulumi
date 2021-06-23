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
import pulumi.runtime.config as config
import pulumi.provider as provider


_ID = 0


class MyDynamicProvider(dynamic.ResourceProvider):

    def create(self, props: Any) -> dynamic.CreateResult:
        global _ID
        _ID = _ID + 1
        return dynamic.CreateResult(id_=str(_ID))


class Resource(dynamic.Resource):
    def __init__(self, name: str, echo: Input[any], opts: Optional[ResourceOptions]=None):
        super().__init__(MyDynamicProvider(), name, {'echo': echo}, opts)


class Component(ComponentResource):
    def __init__(self, name: str, echo: Input[any], secret: Input[str], opts: Optional[ResourceOptions]=None):
        super().__init__('testcomponent:index:Component', name, {}, opts)
        self.echo = pulumi.Output.from_input(echo)
        resource = Resource('child-{}'.format(name), echo, ResourceOptions(parent=self))
        self.child_id = resource.id
        self.secret = secret
        self.register_outputs({
            'childId': self.child_id,
            'echo': self.echo,
            'secret': self.secret
        })


class Provider(provider.Provider):
    VERSION = "0.0.1"

    def __init__(self):
        super().__init__(Provider.VERSION)

    def construct(self, name: str, resource_type: str, inputs: Inputs,
                  options: Optional[ResourceOptions]=None) -> provider.ConstructResult:

        if resource_type != 'testcomponent:index:Component':
            raise Exception('unknown resource type {}'.format(resource_type))
        

        secret_key = "secret"
        cfg = pulumi.Config()
        full_secret_key = cfg.full_key(secret_key)
        if not config.is_config_secret(full_secret_key):
            raise Exception('expected config for key to be secret: {}'.format(full_secret_key))
        secret = cfg.require_secret('secret')
        component = Component(name, inputs['echo'], secret, options)

        return provider.ConstructResult(
            urn=component.urn,
            state={
                'echo': component.echo,
                'childId': component.child_id,
                'secret': component.secret
            })


if __name__ == '__main__':
    provider.main(Provider(), sys.argv[1:])
