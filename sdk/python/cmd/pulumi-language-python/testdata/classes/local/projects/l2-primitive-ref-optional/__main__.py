import pulumi
import pulumi_optional_primitive_ref as optional_primitive_ref

set_res = optional_primitive_ref.Resource("setRes", data=optional_primitive_ref.DataArgs(
    boolean=True,
    float=3.14,
    integer=42,
    string="hello",
    number_array=[
        float(-1),
        float(0),
        float(1),
    ],
    boolean_map={
        "t": True,
        "f": False,
    },
))
unset_res = optional_primitive_ref.Resource("unsetRes", data=optional_primitive_ref.DataArgs())
pulumi.export("setBoolean", set_res.data.boolean)
pulumi.export("setFloat", set_res.data.float)
pulumi.export("setInteger", set_res.data.integer)
pulumi.export("setString", set_res.data.string)
pulumi.export("setNumberArray", set_res.data.number_array)
pulumi.export("setBooleanMap", set_res.data.boolean_map)
pulumi.export("unsetBoolean", unset_res.data.apply(lambda data: "null" if data.boolean is None else "not null"))
pulumi.export("unsetFloat", unset_res.data.apply(lambda data: "null" if data.float is None else "not null"))
pulumi.export("unsetInteger", unset_res.data.apply(lambda data: "null" if data.integer is None else "not null"))
pulumi.export("unsetString", unset_res.data.apply(lambda data: "null" if data.string is None else "not null"))
pulumi.export("unsetNumberArray", unset_res.data.apply(lambda data: "null" if data.number_array is None else "not null"))
pulumi.export("unsetBooleanMap", unset_res.data.apply(lambda data: "null" if data.boolean_map is None else "not null"))
