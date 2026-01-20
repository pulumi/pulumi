import pulumi
from myComponent import MyComponent

pulumi.check_pulumi_version(">=3.0.1")
some_component = MyComponent("someComponent", {
    'input': True})
pulumi.export("result", some_component.output)
