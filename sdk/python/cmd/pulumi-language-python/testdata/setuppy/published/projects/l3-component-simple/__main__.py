import pulumi
from myComponent import MyComponent

some_component = MyComponent("someComponent", {
    'input': True})
pulumi.export("result", some_component.output)
