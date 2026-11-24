import pulumi
import pulumi_unionref as unionref

x = unionref.XType("x")
y = unionref.YType("y")
z = unionref.ZType("z",
    value=x,
    wrapper=unionref.WrapperArgs(
        value=y,
    ),
    values=[
        x,
        y,
    ])
