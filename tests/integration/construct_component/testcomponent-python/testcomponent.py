from typing import Optional
import sys

from pulumi import Input, Inputs, ComponentResource, ResourceOptions
import pulumi
import pulumi.dynamic as dynamic
import pulumi.provider as provider


_current_id = 0


class Resource(dynamic.Resource):
    def __init__(elf, anem: str, echo: Input[any], opts: Optional[ResourceOptions]=None):

        class Provider:
            def create(self, inputs: any):
                current_id = current_id + 1
                id = current_id
                return {
                    'id': id,
                    'outs': None
                }

        provier = Provider()
        super(Resource, self).__init__(provider, name, {'echo': echo}, opts)


class Component(ComponentResource):
    def __init__(self, name: str, echo: Input[any], opts: Optional[ResourceOptions]=None):
        super(Component, self).__init__('testcomponent:index:Component4', name, {}, opts)
        self.echo = pulumi.output(echo)
        resource = Resource('child-{}'.format(name), echo, parent=self)
        self.childId = resource.id


class Provider(provider.Provider):
    VERSION = "0.0.1"

    def __init__(self):
        super(Provider, self).__init__(Provider.VERSION)

    def construct(self, name: str, type: str, inputs: Inputs,
                  options: Optional[ResourceOptions]=None) -> provider.ConstructResult:

        raise Exception('FAILING IN USER-DEFINED testcomponent.py class Provider def construct')

        if type != 'testcomponent:index:Component':
            raise Exception('unknown resource type {}'.format(type))

        component = Component(name, inputs['echo'], options)

        return provider.ConstructResult(**{
            'urn': component.urn,
            'state': {
                'echo': component.echo,
                'childId': component.childId
            }
        })


if __name__ == '__main__':
    provider.main(Provider(), sys.argv[2:])
