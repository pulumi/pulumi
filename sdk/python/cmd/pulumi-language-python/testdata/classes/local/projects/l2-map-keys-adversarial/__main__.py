import pulumi
import pulumi_primitive as primitive

res = primitive.Resource("res",
    boolean=False,
    float=2.17,
    integer=-12,
    string="adversarial",
    number_array=[
        float(0),
        float(1),
    ],
    boolean_map={
        "__type": True,
        "__internal": False,
        "__provider": True,
        "__version": False,
        "": True,
        "Some ${common} \"characters\" 'that' need escaping: \\ (backslash), \x09 (tab), \x1b (escape), \x07 (bell), \x00 (null), \U000e0021 (tag space)": False,
    })
invoke_result = primitive.invoke_output(boolean=False,
    float=2.17,
    integer=-12,
    string="adversarial",
    number_array=[
        float(0),
        float(1),
    ],
    boolean_map={
        "__type": True,
        "__internal": False,
        "__provider": True,
        "__version": False,
        "": True,
        "Some ${common} \"characters\" 'that' need escaping: \\ (backslash), \x09 (tab), \x1b (escape), \x07 (bell), \x00 (null), \U000e0021 (tag space)": False,
    })
pulumi.export("resourceBooleanMap", res.boolean_map)
pulumi.export("invokeBooleanMap", invoke_result.boolean_map)
