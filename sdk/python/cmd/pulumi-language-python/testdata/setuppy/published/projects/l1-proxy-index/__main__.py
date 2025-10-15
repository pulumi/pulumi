import pulumi

config = pulumi.Config()
an_object = config.require_object("anObject")
any_object = config.require_object("anyObject")
l = pulumi.Output.secret([1])
m = pulumi.Output.secret({
    "key": True,
})
c = pulumi.Output.secret(an_object)
o = pulumi.Output.secret({
    "property": "value",
})
a = pulumi.Output.secret(any_object)
pulumi.export("l", l[0])
pulumi.export("m", m["key"])
pulumi.export("c", c["property"])
pulumi.export("o", o["property"])
pulumi.export("a", a["property"])
