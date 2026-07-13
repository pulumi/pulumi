import pulumi
from providerComponent import ProviderComponent

my_component = ProviderComponent("myComponent", {
    'text': "hello"})
pulumi.export("result", my_component.result)
