import pulumi
import pulumi_primitive as primitive

res = primitive.Resource("res",
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
