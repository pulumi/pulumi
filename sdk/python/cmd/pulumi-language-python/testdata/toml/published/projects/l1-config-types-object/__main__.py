import pulumi
import json

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
optional_list = config.get_object("optionalList")
if optional_list is None:
    optional_list = None
optional_map = config.get_object("optionalMap")
if optional_map is None:
    optional_map = None
optional_object = config.get_object("optionalObject")
if optional_object is None:
    optional_object = None
pulumi.export("optionalList", "null" if optional_list is None else json.dumps(optional_list))
pulumi.export("optionalMap", "null" if optional_map is None else json.dumps(optional_map))
pulumi.export("optionalObject", "null" if optional_object is None else json.dumps(optional_object))
