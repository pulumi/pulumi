import pulumi
import pulumi_constant as constant

first = constant.Resource("first",
    kind="Constant",
    flag=True,
    count=3,
    ratio=1.5)
pulumi.export("kind", first.kind)
pulumi.export("flag", first.flag)
pulumi.export("count", first.count)
pulumi.export("ratio", first.ratio)
