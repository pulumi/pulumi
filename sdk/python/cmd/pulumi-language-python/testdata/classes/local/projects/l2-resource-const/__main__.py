import pulumi
import pulumi_constant as constant

first = constant.Resource("first", kind="Constant")
pulumi.export("kind", first.kind)
