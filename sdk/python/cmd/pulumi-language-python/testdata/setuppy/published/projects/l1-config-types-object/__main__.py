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
pulumi.export("theThing", float(any_object["a"]) + float(any_object["b"]))
optional_untyped_object = config.get_object("optionalUntypedObject")
if optional_untyped_object is None:
    optional_untyped_object = {
        "key": "value",
    }
pulumi.export("defaultUntypedObject", optional_untyped_object)
