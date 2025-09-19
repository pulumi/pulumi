import pulumi
import pulumi_provider as provider

class ParentComponent(pulumi.ComponentResource):
    def __init__(self, name, opts=None):
        super().__init__('ParentComponent', name, {}, opts)

parent = ParentComponent("parent")

comp = provider.MyComponent(
    "comp",
    str_input="hello",
    optional_int_input=42,
    dict_input={"a": 1, "b": 2, "c": 3},
    list_input=["a", "b", "c"],
    complex_input={
        "str_input": "complex_str_input_value",
        "nested_input": {
            "str_plain": "nested_str_plain_value",
        },
    },
    asset_input=pulumi.StringAsset("Hello, World!"),
    archive_input=pulumi.AssetArchive(
        {"asset1": pulumi.StringAsset("im inside an archive")}
    ),
    enum_input=provider.Emu.A,
    opts=pulumi.ResourceOptions(parent=parent)
)

pulumi.export("urn", comp.urn)
pulumi.export("strOutput", comp.str_output)
pulumi.export("optionalIntOutput", comp.optional_int_output)
pulumi.export("dictOutput", comp.dict_output)
pulumi.export("listOutput", comp.list_output)
pulumi.export("complexOutput", comp.complex_output)
pulumi.export("assetOutput", comp.asset_output)
pulumi.export("archiveOutput", comp.archive_output)
pulumi.export("enumOutput", comp.enum_output)
