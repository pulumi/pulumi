import pulumi

def can_(fn):
    try:
        _result = fn()
        return True
    except:
        return False


str = "str"
a_list = [
    "a",
    "b",
    "c",
]
pulumi.export("nonOutputCan", can_(lambda: a_list[0]))
config = pulumi.Config()
object = config.require_object("object")
another_object = {
    "nested": "nestedValue",
}
pulumi.export("canFalse", canOutput_(lambda: object["a"]))
pulumi.export("canFalseDoubleNested", canOutput_(lambda: object["a"]["b"]))
pulumi.export("canTrue", can_(lambda: another_object["nested"]))
