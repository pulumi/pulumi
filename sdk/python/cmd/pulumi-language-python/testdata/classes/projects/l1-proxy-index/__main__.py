import pulumi

config = pulumi.Config()
object = config.require_object("object")
l = pulumi.Output.secret([1])
m = pulumi.Output.secret({
    "key": True,
})
c = pulumi.Output.secret(object)
o = pulumi.Output.secret({
    "property": "value",
})
pulumi.export("l", l[0])
pulumi.export("m", m["key"])
pulumi.export("c", c["property"])
pulumi.export("o", o["property"])
