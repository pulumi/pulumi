import pulumi
from myComponent import MyComponent
import pulumi_simple as simple

input = simple.Resource("input", value=True)
some_component = MyComponent("someComponent", {
    'input': input.value})
pulumi.export("result", some_component.output)
