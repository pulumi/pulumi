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
    def __init__(self, name: str, echo: Input[any], opts: Optional[ResourceOptions]=None):
        super().__init__(MyDynamicProvider(), name, {'echo': echo}, opts)


class Component(ComponentResource):
    def __init__(self, name: str, echo: Input[any], opts: Optional[ResourceOptions]=None):
        super().__init__('testcomponent:index:Component', name, {}, opts)
        self.echo = pulumi.Output.from_input(echo)
        resource = Resource('child-{}'.format(name), echo, ResourceOptions(parent=self))
        self.child_id = resource.id


class Provider(provider.Provider):
    VERSION = "0.0.1"

    def __init__(self):
        super().__init__(Provider.VERSION)

    def construct(self, name: str, resource_type: str, inputs: Inputs,
                  options: Optional[ResourceOptions]=None) -> provider.ConstructResult:

        if resource_type != 'testcomponent:index:Component':
            raise Exception('unknown resource type {}'.format(resource_type))

        component = Component(name, inputs['echo'], options)

        return provider.ConstructResult(
            urn=component.urn,
            state={
                'echo': component.echo,
                'childId': component.child_id
            })


if __name__ == '__main__':
    provider.main(Provider(), sys.argv[1:])
