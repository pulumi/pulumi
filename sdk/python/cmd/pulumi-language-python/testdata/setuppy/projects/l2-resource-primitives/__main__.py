import pulumi
import pulumi_primitive as primitive

res = primitive.Resource("res",
    b=True,
    f=3.14,
    i=42,
    s="hello",
    a=[
        -1,
        0,
        1,
    ],
    m={
        "t": True,
        "f": False,
    })
