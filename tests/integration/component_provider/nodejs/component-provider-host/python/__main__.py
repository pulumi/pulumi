import pulumi
import pulumi_nodejs_component_provider as provider

class ParentComponent(pulumi.ComponentResource):
    def __init__(self, name, opts=None):
        super().__init__('ParentComponent', name, {}, opts)

parent = ParentComponent("parent")

comp = provider.MyComponent(
    "comp",
    a_number=123,
    an_optional_string="Bonnie",
    a_boolean_input=pulumi.Output.from_input(True),
    a_complex_type_input={"aNumber": 7, "nestedComplexType": {"aNumber": 9}},
    enum_input=provider.MyEnum.B,
    opts=pulumi.ResourceOptions(parent=parent),
)

pulumi.export("urn", comp.urn)
pulumi.export("aNumberOutput", comp.a_number_output)
pulumi.export("anOptionalStringOutput", comp.an_optional_string_output)
pulumi.export("aBooleanOutput", comp.a_boolean_output)
pulumi.export("aComplexTypeOutput", comp.a_complex_type_output)
pulumi.export("aResourceOutputUrn", comp.a_resource_output.urn)
pulumi.export("aString", comp.a_string)
pulumi.export("enumOutput", comp.enum_output)
