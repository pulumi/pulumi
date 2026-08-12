import pulumi

config = pulumi.Config()
a = config.require_float("a")
b = config.require_float("b")
c = config.require_int("c")
d = config.require_int("d")
pulumi.export("maxResult", max(a, b))
pulumi.export("minResult", min(a, b))
pulumi.export("intMaxResult", max(c, d))
pulumi.export("intMinResult", min(c, d))
