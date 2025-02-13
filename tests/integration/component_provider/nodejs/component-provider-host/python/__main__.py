import pulumi
import pulumi_nodejs_component_provider as provider

comp = provider.MyComponent(
    "comp",
    a_number=123,
    an_optional_string="hello",
)

pulumi.export("urn", comp.urn)
