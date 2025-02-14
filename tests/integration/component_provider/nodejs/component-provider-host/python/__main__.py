import pulumi
import pulumi_nodejs_component_provider as provider

comp = provider.MyComponent(
    "comp",
    a_number=123,
    an_optional_string="hello",
    a_boolean_input=pulumi.Output.from_input(True),
)

pulumi.export("urn", comp.urn)
