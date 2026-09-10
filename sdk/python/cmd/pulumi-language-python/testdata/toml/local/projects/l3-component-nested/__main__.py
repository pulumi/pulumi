import pulumi
from outerComponent import OuterComponent

outer_component = OuterComponent("outerComponent", {
    'input': True})
pulumi.export("result", outer_component.output)
