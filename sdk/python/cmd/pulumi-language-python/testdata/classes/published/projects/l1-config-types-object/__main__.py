import pulumi

config = pulumi.Config()
a_map = config.require_object("aMap")
pulumi.export("theMap", {
    "a": a_map["a"] + 1,
    "b": a_map["b"] + 1,
})
an_object = config.require_object("anObject")
pulumi.export("theObject", an_object["prop"][0])
any_object = config.require_object("anyObject")
pulumi.export("theThing", any_object["a"] + any_object["b"])
