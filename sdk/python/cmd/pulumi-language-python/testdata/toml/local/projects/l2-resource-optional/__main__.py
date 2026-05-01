import pulumi
import pulumi_optionalprimitive as optionalprimitive
import pulumi_primitive as primitive

unset_a = optionalprimitive.Resource("unsetA")
unset_b = optionalprimitive.Resource("unsetB",
    boolean=unset_a.boolean,
    float=unset_a.float,
    integer=unset_a.integer,
    string=unset_a.string,
    number_array=unset_a.number_array,
    boolean_map=unset_a.boolean_map)
pulumi.export("unsetBoolean", unset_b.boolean.apply(lambda boolean: "null" if boolean == None else "not null"))
pulumi.export("unsetFloat", unset_b.float.apply(lambda float: "null" if float == None else "not null"))
pulumi.export("unsetInteger", unset_b.integer.apply(lambda integer: "null" if integer == None else "not null"))
pulumi.export("unsetString", unset_b.string.apply(lambda string: "null" if string == None else "not null"))
pulumi.export("unsetNumberArray", unset_b.number_array.apply(lambda number_array: "null" if number_array == None else "not null"))
pulumi.export("unsetBooleanMap", unset_b.boolean_map.apply(lambda boolean_map: "null" if boolean_map == None else "not null"))
set_a = optionalprimitive.Resource("setA",
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
    })
set_b = optionalprimitive.Resource("setB",
    boolean=set_a.boolean,
    float=set_a.float,
    integer=set_a.integer,
    string=set_a.string,
    number_array=set_a.number_array,
    boolean_map=set_a.boolean_map)
source_primitive = primitive.Resource("sourcePrimitive",
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
    })
from_primitive = optionalprimitive.Resource("fromPrimitive",
    boolean=source_primitive.boolean,
    float=source_primitive.float,
    integer=source_primitive.integer,
    string=source_primitive.string,
    number_array=source_primitive.number_array,
    boolean_map=source_primitive.boolean_map)
pulumi.export("setBoolean", set_b.boolean)
pulumi.export("setFloat", set_b.float)
pulumi.export("setInteger", set_b.integer)
pulumi.export("setString", set_b.string)
pulumi.export("setNumberArray", set_b.number_array)
pulumi.export("setBooleanMap", set_b.boolean_map)
