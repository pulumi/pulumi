import pulumi
import pulumi_nodejs_component_provider as provider

comp = provider.MyComponent(
    "comp",
    a_number=123,
    an_optional_string="Bonnie",
    a_boolean_input=pulumi.Output.from_input(True),
    a_complex_type_input={"aNumber": 7, "nestedComplexType": {"aNumber": 9}},
)

pulumi.export("urn", comp.urn)
pulumi.export("aNumberOutput", comp.a_number_output)
pulumi.export("anOptionalStringOutput", comp.an_optional_string_output)
pulumi.export("aBooleanOutput", comp.a_boolean_output)
pulumi.export("aComplexTypeOutput", comp.a_complex_type_output)
