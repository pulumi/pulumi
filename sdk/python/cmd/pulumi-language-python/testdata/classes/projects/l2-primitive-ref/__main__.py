import pulumi
import pulumi_primitive_ref as primitive_ref

res = primitive_ref.Resource("res", data=primitive_ref.DataArgs(
    boolean=False,
    float=2.17,
    integer=-12,
    string="Goodbye",
    bool_array=[
        False,
        True,
    ],
    string_map={
        "two": "turtle doves",
        "three": "french hens",
    },
))
